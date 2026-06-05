package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rediswap/api"
	"rediswap/internal/pool"
	"rediswap/internal/store"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

func main() {
	port := flag.Int("port", 8080, "Server port")
	poolX := flag.Float64("pool-x", 4, "Initial pool reserve X")
	poolY := flag.Float64("pool-y", 100, "Initial pool reserve Y")
	autoProcess := flag.Bool("auto-process", false, "Automatically process blocks periodically")
	processInterval := flag.Int("process-interval", 10, "Block processing interval in seconds (if auto-process enabled)")
	flag.Parse()

	// Setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Starting RediSwap Server...")

	// Create pool
	p := pool.NewPool(
		decimal.NewFromFloat(*poolX),
		decimal.NewFromFloat(*poolY),
	)

	log.Info().
		Float64("x", *poolX).
		Float64("y", *poolY).
		Str("k", p.K().String()).
		Msg("Pool initialized")

	// Create stores
	txStore := store.NewTransactionStore()
	beliefStore := store.NewBeliefStore()

	// Create API
	rediswapAPI := api.NewRediSwapAPI(txStore, beliefStore, p)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", rediswapAPI.HandleRPC)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	// Start server
	go func() {
		log.Info().Int("port", *port).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// Start auto-processing worker if enabled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *autoProcess {
		log.Info().
			Int("interval_seconds", *processInterval).
			Msg("Auto-processing enabled")
		go startAutoProcessor(ctx, rediswapAPI, time.Duration(*processInterval)*time.Second)
	}

	printUsage(*port, *autoProcess, *processInterval)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info().Msg("Shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	log.Info().Msg("RediSwap Server stopped")
}

func startAutoProcessor(ctx context.Context, api *api.RediSwapAPI, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	blockNum := uint64(1)
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Auto-processor stopped")
			return
		case <-ticker.C:
			log.Info().
				Uint64("block", blockNum).
				Msg("Auto-processing block")

			result, err := api.ProcessBlock([]interface{}{})
			if err != nil {
				log.Error().
					Err(err).
					Uint64("block", blockNum).
					Msg("Failed to process block")
			} else {
				// Log summary
				if blockResult, ok := result.(map[string]interface{}); ok {
					if msg, exists := blockResult["message"]; exists {
						log.Info().
							Uint64("block", blockNum).
							Str("status", msg.(string)).
							Msg("Block processed")
					} else {
						log.Info().
							Uint64("block", blockNum).
							Interface("result", result).
							Msg("Block processed")
					}
				}
			}
			blockNum++
		}
	}
}

func printUsage(port int, autoProcess bool, processInterval int) {
	log.Info().Msg("=================================================")
	log.Info().Msg("RediSwap Server is running!")
	log.Info().Msg("=================================================")
	log.Info().Msgf("Endpoint: http://localhost:%d/", port)
	log.Info().Msg("")
	log.Info().Msg("Supported JSON-RPC methods:")
	log.Info().Msg("  - rediswap_sendSwap      : Submit a swap transaction")
	log.Info().Msg("  - rediswap_sendBelief    : Submit arbitrager belief")
	log.Info().Msg("  - rediswap_processBlock  : Process pending txs and run auctions")
	log.Info().Msg("")
	if autoProcess {
		log.Info().Msgf("Auto-processing: ENABLED (every %d seconds)", processInterval)
	} else {
		log.Info().Msg("Auto-processing: DISABLED (call rediswap_processBlock manually)")
	}
	log.Info().Msg("")
	log.Info().Msgf("Example: curl -X POST http://localhost:%d -H 'Content-Type: application/json' -d '{...}'", port)
	log.Info().Msg("Press Ctrl+C to stop")
	log.Info().Msg("=================================================")
}
