package main

import (
	"context"
	"fmt"
	"rebate/api"
	"rebate/internal/queue"
	"rebate/internal/sim"
	"rebate/pkg/types"
	"rebate/pkg/utils"

	"github.com/ethereum/go-ethereum/common/hexutil"
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
