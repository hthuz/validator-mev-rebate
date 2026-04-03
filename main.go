package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

// 全局 logger
var logger zerolog.Logger

func init() {
	// 配置 zerolog
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	logger = zerolog.New(output).With().Timestamp().Logger()
}

func main() {
	// 解析命令行参数
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()

	logger.Info().Msg("Starting Validator MEV Rebate Node...")

	// 1. 生成签名密钥 (用于 MatchingHash)
	signer, err := GenerateSigner()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate signer")
	}
	logger.Info().Msg("Signer key generated")

	// 2. 创建组件
	store := NewBundleStore()
	queue := NewSimulationQueue()
	simulator := NewMockSimulator()
	hintBroadcaster := &LogHintBroadcaster{}

	// 3. 创建 API
	api := NewMevShareAPI(signer, queue, store, simulator)

	// 4. 创建模拟工作器
	worker := NewSimulationWorker(simulator, queue, store, hintBroadcaster, signer)

	// 5. 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle("/", NewJSONRPCHandler(api))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

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
	go blockUpdater(ctx, simulator, queue)

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
func blockUpdater(ctx context.Context, sim *MockSimulator, queue *SimulationQueue) {
	ticker := time.NewTicker(12 * time.Second) // 12秒一个块 (以太坊主网)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newBlock := sim.GetBlock() + 1
			sim.SetBlock(newBlock)
			queue.UpdateBlock(newBlock)
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
	logger.Info().Msg("Supported JSON-RPC methods:")
	logger.Info().Msg("  - mev_sendBundle    : Submit a bundle")
	logger.Info().Msg("  - mev_simBundle     : Simulate a bundle")
	logger.Info().Msg("  - eth_cancelBundleByHash : Cancel a bundle")
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
