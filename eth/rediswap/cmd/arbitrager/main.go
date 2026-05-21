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
	flag.Parse()

	// Setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Starting RediSwap Arbitrager Client...")

	if *continuous {
		sendContinuous(*serverURL, *arbID, *belief)
	} else {
		sendSingle(*serverURL, *arbID, *belief)
	}
}

func sendSingle(serverURL, arbID string, belief float64) {
	if err := sendBelief(serverURL, arbID, belief); err != nil {
		log.Fatal().Err(err).Msg("Failed to send belief")
	}
}

func sendContinuous(serverURL, arbID string, belief float64) {
	for {
		if err := sendBelief(serverURL, arbID, belief); err != nil {
			log.Error().Err(err).Msg("Failed to send belief")
		}
		time.Sleep(5 * time.Second)
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

	body, _ := json.Marshal(reqBody)

	log.Info().
		Str("arb_id", arbID).
		Float64("belief", belief).
		Msg("Sending belief")

	resp, err := http.Post(serverURL, "application/json", bytes.NewReader(body))
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

	log.Info().Interface("result", result["result"]).Msg("Belief registered")
	return nil
}
