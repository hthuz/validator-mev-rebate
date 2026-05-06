package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"rebate/pkg/types"
	"rebate/pkg/utils"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func main() {

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
			},
		},
		"id": 1,
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result types.JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Error != nil {
		log.Println(result.Error)
	}

	log.Println("RPC response: %+v", result.Result)
}
