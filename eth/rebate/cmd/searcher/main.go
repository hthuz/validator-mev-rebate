package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"

	"rebate/mylog"
	"rebate/pkg/types"
)

var logger = mylog.Logger

func main() {
	subscribeHints("http://localhost:8080/events")
}

func subscribeHints(url string) {
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

		for i, tx := range hint.Txs {
			if tx.Hash != nil {
				logger.Info().Str("txHash", tx.Hash.Hex()).Int("index", i).Msg("tx hint")
			}
		}

		for i, log := range hint.Logs {
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
