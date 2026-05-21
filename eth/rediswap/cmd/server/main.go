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

	printUsage(*port)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info().Msg("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	log.Info().Msg("RediSwap Server stopped")
}

func printUsage(port int) {
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
	log.Info().Msgf("Example: curl -X POST http://localhost:%d -H 'Content-Type: application/json' -d '{...}'", port)
	log.Info().Msg("Press Ctrl+C to stop")
	log.Info().Msg("=================================================")
}
