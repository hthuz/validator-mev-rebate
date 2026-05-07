package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"rebate/mylog"
	"rebate/pkg/types"
	"rebate/pkg/utils"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

var logger = mylog.Logger

func main() {
	SendMultipleTx()

}
func SendMultipleTx() {
	for {
		SendSingleTx()
		time.Sleep(2 * time.Second)
	}

}

func SendSingleTx() {

	// 构造请求
	tx, err := utils.CreateTestTx()
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "mev_sendBundle",
		"params": []map[string]interface{}{
			{
				"version": "v0.1",
				"inclusion": map[string]string{
					"block":    "0xF4240",
					"maxBlock": "0xF424A",
				},
				"body": []map[string]interface{}{
					{"tx": hexutil.Encode(tx)},
				},
				"privacy": map[string]interface{}{
					"hints": 84, // HintHash(16) | HintTxHash(64) | HintLogs(4)
				},
			},
		},
		"id": 1,
	}

	body, _ := json.Marshal(reqBody)
	logger.Info().Msg("sending req")
	resp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Err(err).Msg("err")
	}
	defer resp.Body.Close()

	var result types.JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Error != nil {
		logger.Fatal().Any("err", result.Error).Msg("rpc error")
	}

	logger.Info().Any("RPC response", result.Result).Msg("received resp")
}
