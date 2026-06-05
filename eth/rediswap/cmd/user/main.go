package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "RediSwap server URL")
	direction := flag.String("direction", "X->Y", "Swap direction (X->Y or Y->X)")
	input := flag.Float64("input", 8, "Input amount")
	output := flag.Float64("output", 25, "Minimum output amount")
	continuous := flag.Bool("continuous", false, "Send transactions continuously")
	interval := flag.Int("interval", 3, "Interval between transactions in seconds (continuous mode)")
	flag.Parse()

	// Setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Starting RediSwap User Client...")
	log.Info().
		Str("server", *serverURL).
		Str("direction", *direction).
		Float64("input", *input).
		Float64("output", *output).
		Bool("continuous", *continuous).
		Msg("Configuration")

	if *continuous {
		log.Info().Int("interval_seconds", *interval).Msg("Running in continuous mode")
		sendContinuous(*serverURL, *direction, *input, *output, time.Duration(*interval)*time.Second)
	} else {
		log.Info().Msg("Running in single-shot mode")
		sendSingle(*serverURL, *direction, *input, *output)
	}
}

func sendSingle(serverURL, direction string, input, output float64) {
	if err := sendSwap(serverURL, direction, input, output); err != nil {
		log.Fatal().Err(err).Msg("Failed to send swap")
	}
}

func sendContinuous(serverURL, direction string, input, output float64, interval time.Duration) {
	txCount := 0
	for {
		txCount++
		log.Info().Int("tx_count", txCount).Msg("Sending swap transaction")
		if err := sendSwap(serverURL, direction, input, output); err != nil {
			log.Error().Err(err).Int("tx_count", txCount).Msg("Failed to send swap")
		} else {
			log.Info().Int("tx_count", txCount).Msg("Swap transaction sent successfully")
		}
		time.Sleep(interval)
	}
}

func sendSwap(serverURL, direction string, input, output float64) error {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "rediswap_sendSwap",
		"params": []map[string]interface{}{
			{
				"direction": direction,
				"input":     input,
				"output":    output,
			},
		},
		"id": 1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post(serverURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if errObj, ok := result["error"]; ok {
		return fmt.Errorf("RPC error: %v", errObj)
	}

	if resultData, ok := result["result"].(map[string]interface{}); ok {
		log.Info().
			Str("tx_id", resultData["tx_id"].(string)).
			Str("status", resultData["status"].(string)).
			Msg("Swap accepted by server")
	}
	return nil
}
