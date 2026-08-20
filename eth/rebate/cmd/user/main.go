package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"rebate/api"
	"rebate/internal/client"
	"rebate/internal/logging"
	"rebate/pkg/types"
	"strconv"
	"strings"
	"time"
)

var logger = logging.Logger

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "rebate server url")
	datasetPath := flag.String("dataset", "data/ethereum_transactions.csv", "path to collected Ethereum transaction dataset")
	interval := flag.Duration("interval", 2*time.Second, "bundle send interval")
	mock := flag.Bool("mock", false, "generate random mock transactions instead of loading a dataset")
	mockBundlesPerBlock := flag.Int("mock-bundles-per-block", 5, "maximum random mock bundles sent per new block")
	mockWorkers := flag.Int("mock-workers", 64, "maximum concurrent mock bundle submissions")
	flag.Parse()

	if *mock {
		SendMockBundlesPerBlock(*serverURL, *mockBundlesPerBlock, *mockWorkers)
		return
	}
	SendMultipleTx(*serverURL, *datasetPath, *interval)
}

func SendMultipleTx(serverURL, datasetPath string, interval time.Duration) {
	var builder *client.ReplayBundleBuilder
	var err error
	builder, err = client.NewReplayBundleBuilder(datasetPath)
	if err != nil {
		logger.Fatal().Err(err).Str("dataset", datasetPath).Msg("failed to load replay dataset")
	}

	for {
		currentBlock, err := client.GetCurrentBlock(serverURL)
		if err != nil {
			logger.Error().Err(err).Msg("failed to fetch current block")
			time.Sleep(interval)
			continue
		}
		SendSingleTx(serverURL, builder, currentBlock)
		time.Sleep(interval)
	}
}

func SendMockBundlesPerBlock(serverURL string, maxBundlesPerBlock, maxWorkers int) {
	if maxBundlesPerBlock < 0 || maxWorkers <= 0 {
		logger.Fatal().Int("maxBundlesPerBlock", maxBundlesPerBlock).Msg("max mock bundles per block must be non-negative")
	}
	builder := client.NewMockBundleBuilder()
	jobs := make(chan uint64, 100000)
	for i := 0; i < maxWorkers; i++ {
		go func() {
			for block := range jobs {
				SendSingleTx(serverURL, builder, block)
			}
		}()
	}
	for {
		resp, err := http.Get(strings.TrimRight(serverURL, "/") + "/blocks")
		if err != nil {
			logger.Error().Err(err).Msg("failed to subscribe to block stream")
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Error().Int("status", resp.StatusCode).Msg("block stream returned non-200 status")
			time.Sleep(10 * time.Millisecond)
			continue
		}
		err = consumeBlockStream(resp.Body, func(currentBlock uint64) {
			count := builder.RandomIntn(maxBundlesPerBlock + 1)
			for i := 0; i < count; i++ {
				jobs <- currentBlock
			}
		})
		resp.Body.Close()
		if err != nil && err != io.EOF {
			logger.Error().Err(err).Msg("block stream disconnected")
		}
	}
}

func consumeBlockStream(body io.Reader, onBlock func(uint64)) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if value == "{}" || value == "" {
			continue
		}
		block, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("parse block event %q: %w", value, err)
		}
		onBlock(block)
	}
	return scanner.Err()
}

func SendSingleTx(serverURL string, builder *client.ReplayBundleBuilder, currentBlock uint64) {
	bundle, err := builder.BuildBundle(currentBlock)
	if err != nil {
		logger.Error().Err(err).Msg("failed to build replay bundle")
		return
	}

	body, err := client.BuildHTTPBody(api.SendBundleMethod, bundle, 1)
	if err != nil {
		logger.Error().Err(err).Msg("failed to build json-rpc request")
		return
	}

	logger.Info().
		Uint64("currentBlock", currentBlock).
		Uint64("targetBlock", uint64(bundle.Inclusion.BlockNumber)).
		Int("bodyLen", len(bundle.Body)).
		Msg("sending replay bundle")

	resp, err := http.Post(serverURL, "application/json", body)
	if err != nil {
		logger.Err(err).Msg("request failed")
		return
	}
	defer resp.Body.Close()

	var result types.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Error().Err(err).Msg("failed to decode response")
		return
	}

	if result.Error != nil {
		logger.Error().Any("err", result.Error).Msg("rpc error")
		return
	}

	logger.Info().Any("RPC response", result.Result).Msg("received resp")
}
