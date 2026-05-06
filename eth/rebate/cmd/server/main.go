package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"rebate/api"
	"rebate/internal/hints"
	"rebate/internal/metrics"
	"rebate/internal/queue"
	"rebate/internal/sim"
	"rebate/log"
	"rebate/pkg/utils"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var logger = log.Logger

func main() {
	// 解析命令行参数
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()

	log.Logger.Info().Msg("Starting Validator MEV Rebate Node...")

	// 1. 生成签名密钥 (用于 MatchingHash)
	signer, err := utils.GenerateSigner()
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to generate signer")
	}
	log.Logger.Info().Msg("Signer key generated")

	// 2. 创建组件
	store := sim.NewBundleStore()
	queue := queue.NewSimulationQueue()
	simulator := sim.NewMockSimulator()
	hintBroadcaster := &hints.LogHintBroadcaster{}
	metricsStore := metrics.NewMetricsStore()

	// 3. 创建 API
	share_api := api.NewMevShareAPI(signer, queue, store, simulator)

	// 4. 创建模拟工作器
	worker := sim.NewSimulationWorker(simulator, queue, store, hintBroadcaster, signer, metricsStore)

	// 5. 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle("/", api.NewRootHandler(share_api))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 添加指标查询端点
	metricsHandler := metrics.NewMetricsHandler(metricsStore)
	mux.HandleFunc("/metrics/block/", metricsHandler.GetBlockMetrics)
	mux.HandleFunc("/metrics/validator/", metricsHandler.GetValidatorMetrics)
	mux.HandleFunc("/metrics/validators", metricsHandler.GetAllValidators)
	mux.HandleFunc("/metrics/searcher/", metricsHandler.GetSearcherMetrics)
	mux.HandleFunc("/metrics/searchers", metricsHandler.GetAllSearchers)
	mux.HandleFunc("/metrics/global", metricsHandler.GetGlobalMetrics)
	mux.HandleFunc("/metrics/recent", metricsHandler.GetRecentBlocks)

	server := &http.Server{
		Addr:    ":" + *port,
		Handler: mux,
	}

	// 6. 启动后台服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动模拟工作器
	worker.Start(ctx)

	// 启动块号更新 (模拟区块增长)
	go blockUpdater(ctx, simulator, queue, metricsStore)

	// 7. 启动 HTTP 服务器
	go func() {
		logger.Info().Str("addr", server.Addr).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// 8. 打印使用说明
	printUsage(*port)

	// 9. 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down...")

	// 10. 优雅关闭
	cancel()
	worker.Stop()
	queue.Close()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	logger.Info().Msg("Validator MEV Rebate Node stopped")
}

// blockUpdater 模拟区块增长
func blockUpdater(ctx context.Context, sim *sim.MockSimulator, queue *queue.SimulationQueue, metrics *metrics.MetricsStore) {
	ticker := time.NewTicker(12 * time.Second) // 12秒一个块 (以太坊主网)
	defer ticker.Stop()

	// 模拟验证者地址列表
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
			// 结束上一个区块的指标收集
			prevBlock := sim.GetBlock()
			if metrics != nil && prevBlock > 1000000 {
				metrics.FinalizeBlock(prevBlock, 30000000) // 30M gas limit
			}

			// 切换到新区块
			newBlock := prevBlock + 1
			sim.SetBlock(newBlock)
			queue.UpdateBlock(newBlock)

			// 开始新区块的指标收集
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
	logger.Info().Msg("")
	logger.Info().Msgf("Web Interface: http://localhost:%s/", port)
	logger.Info().Msg("")
	logger.Info().Msg("Supported JSON-RPC methods:")
	logger.Info().Msg("  - mev_sendBundle    : Submit a bundle")
	logger.Info().Msg("  - mev_simBundle     : Simulate a bundle")
	logger.Info().Msg("  - eth_cancelBundleByHash : Cancel a bundle")
	logger.Info().Msg("")
	logger.Info().Msg("Metrics Endpoints:")
	logger.Info().Msg("  GET /metrics/block/{blockNumber}    : 获取指定区块的 MEV 指标")
	logger.Info().Msg("  GET /metrics/validator/{address}    : 获取指定 Validator 的历史表现")
	logger.Info().Msg("  GET /metrics/validators             : 获取所有 Validators 的列表")
	logger.Info().Msg("  GET /metrics/searcher/{address}     : 获取指定 Searcher 的指标")
	logger.Info().Msg("  GET /metrics/searchers              : 获取所有 Searchers 的列表")
	logger.Info().Msg("  GET /metrics/global                 : 获取全局 MEV 统计")
	logger.Info().Msg("  GET /metrics/recent                 : 获取最近区块的指标")
	logger.Info().Msg("")
	logger.Info().Msgf("Example: curl -X POST http://localhost:%s \\", port)
	logger.Info().Msg("  -H 'Content-Type: application/json' \\")
	logger.Info().Msg("  -d '{")
	logger.Info().Msg("    \"jsonrpc\": \"2.0\",")
	logger.Info().Msg("    \"method\": \"mev_sendBundle\",")
	logger.Info().Msg("    \"params\": [{")
	logger.Info().Msg("      \"version\": \"v0.1\",")
	logger.Info().Msg("      \"inclusion\": {\"block\": \"0xF4240\", \"maxBlock\": \"0xF4258\"},")
	logger.Info().Msg("      \"body\": [{\"tx\": \"0x...\"}],")
	logger.Info().Msg("      \"privacy\": {\"hints\": [\"hash\", \"logs\"]}")
	logger.Info().Msg("    }],")
	logger.Info().Msg("    \"id\": 1")
	logger.Info().Msg("  }'")
	logger.Info().Msg("")
	logger.Info().Msg("Press Ctrl+C to stop")
	logger.Info().Msg("=================================================")
}
