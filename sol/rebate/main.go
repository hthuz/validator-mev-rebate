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
	port := flag.String("port", "8081", "HTTP server port")
	flag.Parse()

	logger.Info().Msg("Starting Solana MEV Rebate Node...")

	// 1. 创建组件
	store := NewBundleStore()
	queue := NewSimulationQueue()
	simulator := NewMockSimulator()
	hintBroadcaster := &LogHintBroadcaster{}

	// 2. 创建 API
	api := NewMevShareAPI(queue, store, simulator)

	// 3. 创建模拟工作器
	worker := NewSimulationWorker(simulator, queue, store, hintBroadcaster)

	// 4. 创建 HTTP 服务器
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

	// 5. 启动后台服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动模拟工作器
	worker.Start(ctx)

	// 启动 slot 更新 (模拟 Solana slot 增长，约 400ms 一个 slot)
	go slotUpdater(ctx, simulator, queue)

	// 6. 启动 HTTP 服务器
	go func() {
		logger.Info().Str("addr", server.Addr).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// 7. 打印使用说明
	printUsage(*port)

	// 8. 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down...")

	// 9. 优雅关闭
	cancel()
	worker.Stop()
	queue.Close()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	logger.Info().Msg("Solana MEV Rebate Node stopped")
}

// slotUpdater 模拟 Solana slot 增长
func slotUpdater(ctx context.Context, sim *MockSimulator, queue *SimulationQueue) {
	ticker := time.NewTicker(400 * time.Millisecond) // Solana 约 400ms 一个 slot
	defer ticker.Stop()

	// 模拟验证者地址列表 (Solana base58 格式)
	validators := []string{
		"7a1z7dP7m3WJGkQxjoJ2X2Qb7Q3dFhD9kKJqmKxV6qQz",
		"GNvXKKR6mgcM7kvhvKPob8kBpRLkFFbEE5Qr9h3pSSxy",
		"9QU2QSxhb24FUX3Ti2S2kD9yXs8YqK3Y3qoQW8Q8JQjd",
	}
	validatorIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 切换到新 slot
			prevSlot := sim.GetSlot()
			newSlot := prevSlot + 1
			sim.SetSlot(newSlot)
			queue.UpdateSlot(newSlot)

			validator := validators[validatorIndex%len(validators)]
			validatorIndex++

			logger.Info().
				Uint64("slot", newSlot).
				Str("validator", validator).
				Msg("New slot")
		}
	}
}

// printUsage 打印使用说明
func printUsage(port string) {
	logger.Info().Msg("=================================================")
	logger.Info().Msg("Solana MEV Rebate Node is running!")
	logger.Info().Msg("=================================================")
	logger.Info().Msg("")
	logger.Info().Msg("Supported JSON-RPC methods:")
	logger.Info().Msg("  - mev_sendBundle    : Submit a bundle")
	logger.Info().Msg("  - mev_simBundle     : Simulate a bundle")
	logger.Info().Msg("  - mev_cancelBundle  : Cancel a bundle")
	logger.Info().Msg("")
	logger.Info().Msg("Bundle format (Solana):")
	logger.Info().Msg("  - version   : 'solana-v0.1'")
	logger.Info().Msg("  - inclusion : slot range (Solana uses slot instead of block)")
	logger.Info().Msg("  - body      : transactions (base64 encoded)")
	logger.Info().Msg("  - privacy   : hints config (program, logs, etc.)")
	logger.Info().Msg("")
	logger.Info().Msgf("Example: curl -X POST http://localhost:%s \\", port)
	logger.Info().Msg("  -H 'Content-Type: application/json' \\")
	logger.Info().Msg("  -d '")
	logger.Info().Msg("  {")
	logger.Info().Msg("    \"jsonrpc\": \"2.0\",")
	logger.Info().Msg("    \"method\": \"mev_sendBundle\",")
	logger.Info().Msg("    \"params\": [{")
	logger.Info().Msg("      \"version\": \"solana-v0.1\",")
	logger.Info().Msg("      \"inclusion\": {\"slot\": 1000100, \"maxSlot\": 1000120},")
	logger.Info().Msg("      \"body\": [{\"tx\": \"base64_encoded_solana_transaction\"}],")
	logger.Info().Msg("      \"privacy\": {\"hints\": [\"program\", \"logs\"], \"builders\": [\"jito\"]}")
	logger.Info().Msg("    }],")
	logger.Info().Msg("    \"id\": 1")
	logger.Info().Msg("  }'")
	logger.Info().Msg("")
	logger.Info().Msg("Hint types for Solana:")
	logger.Info().Msg("  - program      : Program ID (like contract address)")
	logger.Info().Msg("  - instruction  : Instruction type")
	logger.Info().Msg("  - logs         : All logs")
	logger.Info().Msg("  - specialLogs  : DEX-related logs only")
	logger.Info().Msg("  - hash         : Matching hash (for backrunning)")
	logger.Info().Msg("")
	logger.Info().Msg("Backrun example:")
	logger.Info().Msg("  1. Submit bundle A with hint 'hash' enabled")
	logger.Info().Msg("  2. Get matchingHash from response")
	logger.Info().Msg("  3. Submit bundle B with body[0].hash = matchingHash")
	logger.Info().Msg("  4. Bundle B will backrun bundle A")
	logger.Info().Msg("")
	logger.Info().Msg("Press Ctrl+C to stop")
	logger.Info().Msg("=================================================")
}
