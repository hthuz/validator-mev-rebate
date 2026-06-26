package sim

import (
	"context"
	"os"
	"path/filepath"
	"rebate/internal/dataset"
	"rebate/pkg/types"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestReplaySimulatorSimulateBundle(t *testing.T) {
	dataPath := filepath.Join("..", "..", "data", "ethereum_transactions.csv")
	if _, err := os.Stat(dataPath); err != nil {
		t.Fatalf("stat dataset: %v", err)
	}

	ds, err := dataset.LoadCSV(dataPath)
	if err != nil {
		t.Fatalf("LoadCSV: %v", err)
	}

	block, ok := ds.FirstBlock()
	if !ok || len(block.Transactions) == 0 {
		t.Fatal("expected dataset to contain at least one block with transactions")
	}

	sim, err := NewReplaySimulator(ds, 30000000)
	if err != nil {
		t.Fatalf("NewReplaySimulator: %v", err)
	}

	stateBlock, ok := sim.AdvanceBlock()
	if !ok {
		t.Fatal("expected first replay block")
	}

	tx := block.Transactions[0]
	rawTx := hexutil.Bytes(tx.RawTx)
	bundle := &types.SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: types.MevBundleInclusion{
			BlockNumber: hexutil.Uint64(block.Number),
			MaxBlock:    hexutil.Uint64(block.Number),
		},
		Body: []types.MevBundleBody{
			{Tx: &rawTx},
		},
		Metadata: &types.MevBundleMetadata{},
	}

	result, err := sim.SimulateBundle(context.Background(), bundle, nil)
	if err != nil {
		t.Fatalf("SimulateBundle: %v", err)
	}

	if uint64(result.StateBlock) != stateBlock {
		t.Fatalf("unexpected state block: got %d want %d", uint64(result.StateBlock), stateBlock)
	}
	if result.Block == nil {
		t.Fatal("expected replay block context")
	}
	if uint64(result.Block.BlockNumber) != block.Number {
		t.Fatalf("unexpected target block: got %d want %d", uint64(result.Block.BlockNumber), block.Number)
	}
	if len(result.Block.BundleTxs) != 1 {
		t.Fatalf("expected 1 simulated tx, got %d", len(result.Block.BundleTxs))
	}
	if result.Block.BundleTxs[0].Hash != tx.Hash {
		t.Fatalf("unexpected simulated tx hash: got %s want %s", result.Block.BundleTxs[0].Hash.Hex(), tx.Hash.Hex())
	}
}
