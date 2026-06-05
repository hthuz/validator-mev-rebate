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
	arbID := flag.String("id", "arb1", "Arbitrager ID")
	belief := flag.Float64("belief", 4.0, "Price belief (1X = belief*Y)")
	continuous := flag.Bool("continuous", false, "Send beliefs continuously")
	interval := flag.Int("interval", 5, "Interval between beliefs in seconds (continuous mode)")
	flag.Parse()

	// Setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Starting RediSwap Arbitrager Client...")
	log.Info().
		Str("server", *serverURL).
		Str("arb_id", *arbID).
		Float64("belief", *belief).
		Bool("continuous", *continuous).
		Msg("Configuration")

	if *continuous {
		log.Info().Int("interval_seconds", *interval).Msg("Running in continuous mode")
		sendContinuous(*serverURL, *arbID, *belief, time.Duration(*interval)*time.Second)
	} else {
		log.Info().Msg("Running in single-shot mode")
		sendSingle(*serverURL, *arbID, *belief)
	}
}

func sendSingle(serverURL, arbID string, belief float64) {
	if err := sendBelief(serverURL, arbID, belief); err != nil {
		log.Fatal().Err(err).Msg("Failed to send belief")
	}
}

func sendContinuous(serverURL, arbID string, belief float64, interval time.Duration) {
	updateCount := 0
	for {
		updateCount++
		log.Info().Int("update_count", updateCount).Msg("Sending belief update")
		if err := sendBelief(serverURL, arbID, belief); err != nil {
			log.Error().Err(err).Int("update_count", updateCount).Msg("Failed to send belief")
		} else {
			log.Info().Int("update_count", updateCount).Msg("Belief registered successfully")
		}
		time.Sleep(interval)
	}
}

func sendBelief(serverURL, arbID string, belief float64) error {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "rediswap_sendBelief",
		"params": []map[string]interface{}{
			{
				"arb_id": arbID,
				"belief": belief,
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
			Str("arb_id", resultData["arb_id"].(string)).
			Str("status", resultData["status"].(string)).
			Msg("Belief accepted by server")
	}
	return nil
}
