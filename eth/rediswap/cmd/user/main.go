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
	flag.Parse()

	// Setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Starting RediSwap User Client...")

	if *continuous {
		sendContinuous(*serverURL, *direction, *input, *output)
	} else {
		sendSingle(*serverURL, *direction, *input, *output)
	}
}

func sendSingle(serverURL, direction string, input, output float64) {
	if err := sendSwap(serverURL, direction, input, output); err != nil {
		log.Fatal().Err(err).Msg("Failed to send swap")
	}
}

func sendContinuous(serverURL, direction string, input, output float64) {
	for {
		if err := sendSwap(serverURL, direction, input, output); err != nil {
			log.Error().Err(err).Msg("Failed to send swap")
		}
		time.Sleep(3 * time.Second)
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

	log.Info().
		Str("direction", direction).
		Float64("input", input).
		Float64("output", output).
		Msg("Sending swap transaction")

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

	log.Info().Interface("result", result["result"]).Msg("Swap submitted")
	return nil
}
