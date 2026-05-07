package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"rebate/api"
	"rebate/config"
	"rebate/internal/builder"
	"rebate/internal/metrics"
	"rebate/internal/queue"
	"rebate/internal/sim"
	"rebate/internal/sse"
	"rebate/mylog"
	"rebate/pkg/utils"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var logger = mylog.Logger

func main() {
	configPath := flag.String("config", "", "path to config file (default: config/config.yaml)")
	flag.Parse()

	logger.Info().Msg("Starting Validator MEV Rebate Node...")

	// 1. 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}
	logger.Info().Str("file", *configPath).Msg("Config loaded")

	// 2. 生成签名密钥 (用于 MatchingHash)
	signer, err := utils.GenerateSigner()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate signer")
	}
	logger.Info().Msg("Signer key generated")

	// 3. 创建组件
	store := sim.NewBundleStore()
	simQueue := queue.NewSimulationQueue()
	simulator := sim.NewMockSimulator()
	hintBroadcaster := sse.NewHub()
	metricsStore := metrics.NewMetricsStore()

	// 4. 从配置创建 Builder Registry
	registry := builder.NewRegistry()
	for _, b := range cfg.Dispatcher.Builders {
		if err := registry.Register(b.Name, b.URL, b.Score); err != nil {
			logger.Fatal().Err(err).Str("builder", b.Name).Msg("Failed to register builder")
		}
	}
	dispatcher := builder.NewDispatcher(registry)

	// 打印已注册的 builder 列表
	logger.Info().Msg("=== Registered Builders ===")
	totalScore := registry.TotalScore()
	for _, b := range registry.All() {
		logger.Info().
			Str("name", b.Name).
			Str("url", b.URL).
			Float64("score", b.Score).
			Str("weight", fmt.Sprintf("%.1f%%", b.Score/totalScore*100)).
			Msg("Builder registered")
	}
	logger.Info().Msg("===========================")

	// 5. 启动 mock builder 节点
	for _, m := range cfg.MockBuilders {
		m := m // capture
		go func() {
			b := builder.NewMockBuilder(m.Addr)
			logger.Info().Str("name", m.Name).Str("addr", m.Addr).Msg("Starting mock builder")
			if err := b.Start(); err != nil {
				logger.Error().Err(err).Str("name", m.Name).Msg("Mock builder stopped")
			}
		}()
	}

	// 6. 创建 API
	shareAPI := api.NewMevShareAPI(signer, simQueue, store, simulator)

	// 7. 创建模拟工作器
	worker := sim.NewSimulationWorker(simulator, simQueue, store, hintBroadcaster, signer, metricsStore, dispatcher)

	// 8. 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle("/", api.NewRootHandler(shareAPI))
	mux.HandleFunc("/events", api.NewSSEHandler(hintBroadcaster))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	metricsHandler := metrics.NewMetricsHandler(metricsStore)
	mux.HandleFunc("/metrics/block/", metricsHandler.GetBlockMetrics)
	mux.HandleFunc("/metrics/validator/", metricsHandler.GetValidatorMetrics)
	mux.HandleFunc("/metrics/validators", metricsHandler.GetAllValidators)
	mux.HandleFunc("/metrics/searcher/", metricsHandler.GetSearcherMetrics)
	mux.HandleFunc("/metrics/searchers", metricsHandler.GetAllSearchers)
	mux.HandleFunc("/metrics/global", metricsHandler.GetGlobalMetrics)
	mux.HandleFunc("/metrics/recent", metricsHandler.GetRecentBlocks)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: mux,
	}

	// 9. 启动后台服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	go blockUpdater(ctx, simulator, simQueue, metricsStore)

	go func() {
		logger.Info().Str("addr", server.Addr).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	printUsage(fmt.Sprintf("%d", cfg.Server.Port))

	// 10. 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down...")

	// 打印分发汇总
	logger.Info().Msg("=== Builder Dispatch Summary ===")
	dispatchLog := dispatcher.Log()
	for _, b := range registry.All() {
		records := dispatchLog.ByBuilder(b.Name)
		success, failed := 0, 0
		for _, r := range records {
			if r.Success {
				success++
			} else {
				failed++
			}
		}
		logger.Info().
			Str("builder", b.Name).
			Int("total", len(records)).
			Int("success", success).
			Int("failed", failed).
			Msg("Builder dispatch stats")
	}
	logger.Info().Msg("================================")

	// 11. 优雅关闭
	cancel()
	worker.Stop()
	simQueue.Close()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	logger.Info().Msg("Validator MEV Rebate Node stopped")
}

// blockUpdater 模拟区块增长
func blockUpdater(ctx context.Context, sim *sim.MockSimulator, queue *queue.SimulationQueue, metrics *metrics.MetricsStore) {
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	validators := []string{
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0x9876543210987654321098765432109876543210",
	}
	validatorIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prevBlock := sim.GetBlock()
			if metrics != nil && prevBlock > 1000000 {
				metrics.FinalizeBlock(prevBlock, 30000000)
			}

			newBlock := prevBlock + 1
			sim.SetBlock(newBlock)
			queue.UpdateBlock(newBlock)

			if metrics != nil {
				validator := common.HexToAddress(validators[validatorIndex%len(validators)])
				metrics.StartNewBlock(newBlock, validator)
				validatorIndex++
			}

			logger.Info().Uint64("block", newBlock).Msg("New block")
		}
	}
}

// printUsage 打印使用说明
func printUsage(port string) {
	logger.Info().Msg("=================================================")
	logger.Info().Msg("Validator MEV Rebate Node is running!")
	logger.Info().Msg("=================================================")
	logger.Info().Msgf("Web Interface: http://localhost:%s/", port)
	logger.Info().Msg("")
	logger.Info().Msg("Supported JSON-RPC methods:")
	logger.Info().Msg("  - mev_sendBundle         : Submit a bundle")
	logger.Info().Msg("  - mev_simBundle          : Simulate a bundle")
	logger.Info().Msg("  - eth_cancelBundleByHash : Cancel a bundle")
	logger.Info().Msg("")
	logger.Info().Msg("Metrics Endpoints:")
	logger.Info().Msg("  GET /events                         : SSE hint stream")
	logger.Info().Msg("  GET /metrics/block/{blockNumber}    : Block MEV metrics")
	logger.Info().Msg("  GET /metrics/validator/{address}    : Validator metrics")
	logger.Info().Msg("  GET /metrics/validators             : All validators")
	logger.Info().Msg("  GET /metrics/searcher/{address}     : Searcher metrics")
	logger.Info().Msg("  GET /metrics/searchers              : All searchers")
	logger.Info().Msg("  GET /metrics/global                 : Global MEV stats")
	logger.Info().Msg("  GET /metrics/recent                 : Recent blocks")
	logger.Info().Msg("")
	logger.Info().Msgf("Example: curl -X POST http://localhost:%s -H 'Content-Type: application/json' -d '{...}'", port)
	logger.Info().Msg("Press Ctrl+C to stop")
	logger.Info().Msg("=================================================")
}
