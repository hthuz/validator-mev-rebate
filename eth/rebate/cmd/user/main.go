package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"rebate/api"
	"rebate/internal/client"
	"rebate/mylog"
	"rebate/pkg/types"
	"time"
)

var logger = mylog.Logger

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "rebate server url")
	datasetPath := flag.String("dataset", "data/ethereum_transactions.csv", "path to collected Ethereum transaction dataset")
	interval := flag.Duration("interval", 2*time.Second, "bundle send interval")
	flag.Parse()

	SendMultipleTx(*serverURL, *datasetPath, *interval)
}

func SendMultipleTx(serverURL, datasetPath string, interval time.Duration) {
	builder, err := client.NewReplayBundleBuilder(datasetPath)
	if err != nil {
		logger.Fatal().Err(err).Str("dataset", datasetPath).Msg("failed to load replay dataset")
	}

	for {
		SendSingleTx(serverURL, builder)
		time.Sleep(interval)
	}
}

func SendSingleTx(serverURL string, builder *client.ReplayBundleBuilder) {
	currentBlock, err := client.GetCurrentBlock(serverURL)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch current block")
		return
	}

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
