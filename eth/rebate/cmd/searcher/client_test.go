package main

import (
	"context"
	"fmt"
	"math/big"
	"rebate/api"
	"rebate/internal/queue"
	"rebate/internal/sim"
	"rebate/pkg/types"
	"rebate/pkg/utils"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// // TestClient 测试 MEV-Share Demo
// func TestClient(t *testing.T) {
// 	// 启动服务器
// 	ctx, cancel := context.WithCancel(context.Background())

// 	signer, _ := sim.GenerateSigner()
// 	store := sim.NewBundleStore()
// 	queue := queue.NewSimulationQueue()
// 	simulator := sim.NewMockSimulator()
// 	api := api.NewMevShareAPI(signer, queue, store, simulator)
// 	hintBroadcaster := &internal.LogHintBroadcaster{}
// 	metricsStore := metrics.NewMetricsStore()
// 	worker := sim.NewSimulationWorker(simulator, queue, store, hintBroadcaster, signer, metricsStore)

// 	// 启动 worker
// 	worker.Start(ctx)
// 	// 正确的关闭顺序: 先 cancel context, 再关闭 queue, 最后 stop worker
// 	defer func() {
// 		cancel()
// 		queue.Close()
// 		worker.Stop()
// 	}()

// 	// 创建测试交易
// 	tx := createTestTx(t)

// 	// 测试 SendBundle
// 	t.Run("SendBundle", func(t *testing.T) {
// 		bundle := types.SendMevBundleArgs{
// 			Version: "v0.1",
// 			Inclusion: MevBundleInclusion{
// 				BlockNumber: hexutil.Uint64(1000000),
// 				MaxBlock:    hexutil.Uint64(1000010),
// 			},
// 			Body: []MevBundleBody{
// 				{Tx: &tx},
// 			},
// 			Privacy: &MevBundlePrivacy{
// 				Hints: HintHash | HintLogs | HintContractAddress,
// 			},
// 		}

// 		result, err := api.SendBundle(ctx, bundle)
// 		if err != nil {
// 			t.Fatalf("SendBundle failed: %v", err)
// 		}

// 		if result.BundleHash == (common.Hash{}) {
// 			t.Error("Expected non-zero bundle hash")
// 		}

// 		t.Logf("Bundle hash: %s", result.BundleHash.Hex())
// 	})

// 	// 等待处理
// 	time.Sleep(200 * time.Millisecond)

// 	// 测试 SimBundle
// 	t.Run("SimBundle", func(t *testing.T) {
// 		bundle := SendMevBundleArgs{
// 			Version: "v0.1",
// 			Inclusion: MevBundleInclusion{
// 				BlockNumber: hexutil.Uint64(1000000),
// 				MaxBlock:    hexutil.Uint64(1000010),
// 			},
// 			Body: []MevBundleBody{
// 				{Tx: &tx},
// 			},
// 		}

// 		result, err := api.SimBundle(ctx, bundle)
// 		if err != nil {
// 			t.Fatalf("SimBundle failed: %v", err)
// 		}

// 		if !result.Success {
// 			t.Errorf("Expected success, got error: %s", result.Error)
// 		}

// 		t.Logf("Simulation result: gasUsed=%d, profit=%s", result.GasUsed, result.Profit.ToInt().String())
// 	})
// }

// // TestValidation 测试 Bundle 验证
// func TestValidation(t *testing.T) {
// 	signer, _ := GenerateSigner()
// 	tx := createTestTx(t)

// 	tests := []struct {
// 		name    string
// 		bundle  SendMevBundleArgs
// 		wantErr bool
// 	}{
// 		{
// 			name: "valid bundle",
// 			bundle: SendMevBundleArgs{
// 				Version: "v0.1",
// 				Inclusion: MevBundleInclusion{
// 					BlockNumber: hexutil.Uint64(1000000),
// 					MaxBlock:    hexutil.Uint64(1000010),
// 				},
// 				Body: []MevBundleBody{{Tx: &tx}},
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "invalid version",
// 			bundle: SendMevBundleArgs{
// 				Version: "invalid",
// 				Inclusion: MevBundleInclusion{
// 					BlockNumber: hexutil.Uint64(1000000),
// 					MaxBlock:    hexutil.Uint64(1000010),
// 				},
// 				Body: []MevBundleBody{{Tx: &tx}},
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "invalid inclusion (maxBlock < block)",
// 			bundle: SendMevBundleArgs{
// 				Version: "v0.1",
// 				Inclusion: MevBundleInclusion{
// 					BlockNumber: hexutil.Uint64(1000010),
// 					MaxBlock:    hexutil.Uint64(1000000),
// 				},
// 				Body: []MevBundleBody{{Tx: &tx}},
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "empty body",
// 			bundle: SendMevBundleArgs{
// 				Version: "v0.1",
// 				Inclusion: MevBundleInclusion{
// 					BlockNumber: hexutil.Uint64(1000000),
// 					MaxBlock:    hexutil.Uint64(1000010),
// 				},
// 				Body: []MevBundleBody{},
// 			},
// 			wantErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			_, _, err := ValidateBundle(&tt.bundle, 1000000, signer)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("ValidateBundle() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// // TestHintExtraction 测试 Hint 提取
// func TestHintExtraction(t *testing.T) {
// 	tx := createTestTx(t)

// 	bundle := &SendMevBundleArgs{
// 		Version: "v0.1",
// 		Inclusion: MevBundleInclusion{
// 			BlockNumber: hexutil.Uint64(1000000),
// 			MaxBlock:    hexutil.Uint64(1000010),
// 		},
// 		Body: []MevBundleBody{{Tx: &tx}},
// 		Privacy: &MevBundlePrivacy{
// 			Hints: HintHash | HintContractAddress | HintFunctionSelector | HintLogs,
// 		},
// 		Metadata: &MevBundleMetadata{
// 			MatchingHash: common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
// 		},
// 	}

// 	simResult := &SimMevBundleResponse{
// 		Success:     true,
// 		GasUsed:     21000,
// 		MevGasPrice: hexutil.Big(*big.NewInt(1000000000)),
// 		BodyLogs: []SimMevBodyLogs{
// 			{
// 				TxLogs: []SimLog{
// 					{
// 						Address: common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D"),
// 						Topics: []common.Hash{
// 							common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"),
// 						},
// 						Data: []byte{0x01, 0x02, 0x03},
// 					},
// 				},
// 			},
// 		},
// 	}

// 	hint := ExtractHints(bundle, simResult)
// 	if hint == nil {
// 		t.Fatal("Expected non-nil hint")
// 	}

// 	if hint.Hash != bundle.Metadata.MatchingHash {
// 		t.Error("Hint hash mismatch")
// 	}

// 	if len(hint.Txs) != 1 {
// 		t.Errorf("Expected 1 tx hint, got %d", len(hint.Txs))
// 	}

// 	if len(hint.Logs) != 1 {
// 		t.Errorf("Expected 1 log, got %d", len(hint.Logs))
// 	}

// 	t.Logf("Hint extracted: hash=%s, txs=%d, logs=%d", hint.Hash.Hex(), len(hint.Txs), len(hint.Logs))
// }

// // TestJSONRPCEndpoint 测试 JSON-RPC 端点
// func TestJSONRPCEndpoint(t *testing.T) {
// 	// 创建服务器
// 	signer, _ := GenerateSigner()
// 	store := NewBundleStore()
// 	queue := NewSimulationQueue()
// 	simulator := NewMockSimulator()
// 	api := NewMevShareAPI(signer, queue, store, simulator)
// 	handler := NewJSONRPCHandler(api)

// 	// 创建测试服务器
// 	server := &http.Server{Addr: ":18080", Handler: handler}
// 	go server.ListenAndServe()
// 	defer server.Close()
// 	time.Sleep(100 * time.Millisecond)

// 	// 构造请求
// 	tx := createTestTx(t)
// 	reqBody := map[string]interface{}{
// 		"jsonrpc": "2.0",
// 		"method":  "mev_sendBundle",
// 		"params": []map[string]interface{}{
// 			{
// 				"version": "v0.1",
// 				"inclusion": map[string]string{
// 					"block":    "0xF4240",
// 					"maxBlock": "0xF424A",
// 				},
// 				"body": []map[string]interface{}{
// 					{"tx": hexutil.Encode(tx)},
// 				},
// 			},
// 		},
// 		"id": 1,
// 	}

// 	body, _ := json.Marshal(reqBody)
// 	resp, err := http.Post("http://localhost:18080", "application/json", bytes.NewReader(body))
// 	if err != nil {
// 		t.Fatalf("HTTP request failed: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	var result api.JSONRPCResponse
// 	json.NewDecoder(resp.Body).Decode(&result)

// 	if result.Error != nil {
// 		t.Errorf("RPC error: %s", result.Error.Message)
// 	}

// 	t.Logf("RPC response: %+v", result.Result)
// }

// createTestTx 创建测试交易
func createTestTx(t *testing.T) hexutil.Bytes {
	// 生成测试私钥
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// 创建交易
	tx := etypes.NewTx(&etypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1000000000), // 1 Gwei
		Gas:      21000,
		To:       ptrAddress(common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D")),
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		Data:     []byte{0xa9, 0x05, 0x9c, 0xbb},  // transfer 函数选择器
	})

	// 签名
	signer := etypes.NewEIP155Signer(big.NewInt(1))
	signedTx, err := etypes.SignTx(tx, signer, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign tx: %v", err)
	}

	// RLP 编码
	data, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		t.Fatalf("Failed to encode tx: %v", err)
	}

	return data
}

func ptrAddress(addr common.Address) *common.Address {
	return &addr
}

// ExampleUsage 示例用法
func ExampleUsage() {
	// 1. 启动服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signer, _ := utils.GenerateSigner()
	store := sim.NewBundleStore()
	queue := queue.NewSimulationQueue()
	simulator := sim.NewMockSimulator()
	api := api.NewMevShareAPI(signer, queue, store, simulator)

	// 2. 创建 Bundle
	tx := hexutil.MustDecode("0xf86c808504a817c80082520894...")
	bundle := types.SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: types.MevBundleInclusion{
			BlockNumber: hexutil.Uint64(1000000),
			MaxBlock:    hexutil.Uint64(1000010),
		},
		Body: []types.MevBundleBody{
			{Tx: (*hexutil.Bytes)(&tx)},
		},
		Privacy: &types.MevBundlePrivacy{
			Hints:    types.HintHash | types.HintLogs,
			Builders: []string{"flashbots"},
		},
	}

	// 3. 提交 Bundle
	result, err := api.SendBundle(ctx, bundle)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Bundle submitted: %s\n", result.BundleHash.Hex())
}
