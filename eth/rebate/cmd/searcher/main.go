package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"net/http"
	"rebate/api"
	"rebate/internal/client"
	"strings"
	"sync"

	"rebate/mylog"
	"rebate/pkg/types"
)

var logger = mylog.Logger
var seenMatchingHashes sync.Map

func main() {
	streamURL := flag.String("events", "http://localhost:8080/events", "rebate SSE stream url")
	serverURL := flag.String("server", "http://localhost:8080", "rebate server url")
	datasetPath := flag.String("dataset", "data/ethereum_transactions.csv", "path to collected Ethereum transaction dataset")
	maxChainDepth := flag.Int("max-chain-depth", 2, "maximum bundle depth to backrun")
	flag.Parse()

	builder, err := client.NewReplayBundleBuilder(*datasetPath)
	if err != nil {
		logger.Fatal().Err(err).Str("dataset", *datasetPath).Msg("failed to load replay dataset")
	}

	subscribeHints(*streamURL, *serverURL, builder, *maxChainDepth)
}

func subscribeHints(url, serverURL string, builder *client.ReplayBundleBuilder, maxChainDepth int) {
	resp, err := http.Get(url)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to SSE stream")
	}
	defer resp.Body.Close()

	logger.Info().Str("url", url).Msg("connected to SSE stream")

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE 格式：跳过空行和 event: 行，只处理 data: 行
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		raw := strings.TrimPrefix(line, "data:")
		raw = strings.TrimSpace(raw)

		// 跳过连接确认事件
		if raw == "{}" {
			logger.Info().Msg("SSE connected confirmed")
			continue
		}

		var hint types.Hint
		if err := json.Unmarshal([]byte(raw), &hint); err != nil {
			logger.Error().Err(err).Str("raw", raw).Msg("failed to parse hint")
			continue
		}

		logger.Info().
			Str("hash", hint.Hash.Hex()).
			Int("txs", len(hint.Txs)).
			Int("logs", len(hint.Logs)).
			Msg("received hint")

		submitBackrun(serverURL, builder, &hint, maxChainDepth)

		for i, tx := range hint.Txs {
			if tx.Hash != nil {
				logger.Info().Str("txHash", tx.Hash.Hex()).Int("index", i).Msg("tx hint")
			}
		}

		maxLogsToPrint := 5
		for i, log := range hint.Logs {
			if i >= maxLogsToPrint {
				logger.Info().Int("remaining", len(hint.Logs)-maxLogsToPrint).Msg("additional logs omitted")
				break
			}
			logger.Info().
				Str("address", log.Address.Hex()).
				Int("topics", len(log.Topics)).
				Int("index", i).
				Msg("log hint")
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error().Err(err).Msg("SSE stream error")
	}
}

func submitBackrun(serverURL string, builder *client.ReplayBundleBuilder, hint *types.Hint, maxChainDepth int) {
	if len(hint.Txs) >= maxChainDepth {
		logger.Info().
			Str("matchingHash", hint.Hash.Hex()).
			Int("txDepth", len(hint.Txs)).
			Int("maxChainDepth", maxChainDepth).
			Msg("skipping backrun because chain depth limit reached")
		return
	}

	if _, exists := seenMatchingHashes.LoadOrStore(hint.Hash, struct{}{}); exists {
		return
	}

	currentBlock, err := client.GetCurrentBlock(serverURL)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch current block for backrun")
		return
	}

	bundle, err := builder.BuildBackrunBundle(currentBlock, hint)
	if err != nil {
		logger.Error().Err(err).Msg("failed to build backrun bundle")
		return
	}

	body, err := client.BuildHTTPBody(api.SendBundleMethod, bundle, 1)
	if err != nil {
		logger.Error().Err(err).Msg("failed to build backrun request")
		return
	}

	resp, err := http.Post(serverURL, "application/json", body)
	if err != nil {
		logger.Error().Err(err).Msg("failed to submit backrun bundle")
		return
	}
	defer resp.Body.Close()

	var result types.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Error().Err(err).Msg("failed to decode backrun response")
		return
	}
	if result.Error != nil {
		logger.Error().Any("err", result.Error).Msg("backrun rpc error")
		return
	}

	logger.Info().
		Str("matchingHash", hint.Hash.Hex()).
		Uint64("targetBlock", uint64(bundle.Inclusion.BlockNumber)).
		Msg("submitted backrun bundle")
}
