package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"rebate/internal/dataset"
	"rebate/pkg/types"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type ReplayBundleBuilder struct {
	dataset *dataset.Dataset
	random  *rand.Rand
}

func NewReplayBundleBuilder(datasetPath string) (*ReplayBundleBuilder, error) {
	ds, err := dataset.LoadCSV(datasetPath)
	if err != nil {
		return nil, err
	}

	return &ReplayBundleBuilder{
		dataset: ds,
		random:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (b *ReplayBundleBuilder) BuildBundle(currentBlock uint64) (*types.SendMevBundleArgs, error) {
	block, ok := b.dataset.NextBlockAtOrAfter(currentBlock)
	if !ok {
		block, ok = b.dataset.FirstBlock()
	}
	if !ok || len(block.Transactions) == 0 {
		return nil, fmt.Errorf("dataset has no transactions")
	}

	tx := block.Transactions[b.random.Intn(len(block.Transactions))]
	rawTx := hexutil.Bytes(tx.RawTx)
	maxBlock := block.Number
	if nextBlock, ok := b.dataset.NextBlockAtOrAfter(block.Number + 1); ok {
		maxBlock = nextBlock.Number
	}

	return &types.SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: types.MevBundleInclusion{
			BlockNumber: hexutil.Uint64(block.Number),
			MaxBlock:    hexutil.Uint64(maxBlock),
		},
		Body: []types.MevBundleBody{
			{Tx: &rawTx},
		},
		Privacy: &types.MevBundlePrivacy{
			Hints: types.HintHash | types.HintTxHash | types.HintLogs | types.HintContractAddress | types.HintFunctionSelector,
		},
	}, nil
}

func (b *ReplayBundleBuilder) BuildBackrunBundle(currentBlock uint64, hint *types.Hint) (*types.SendMevBundleArgs, error) {
	block, ok := b.dataset.NextBlockAtOrAfter(currentBlock)
	if !ok {
		block, ok = b.dataset.FirstBlock()
	}
	if !ok || len(block.Transactions) == 0 {
		return nil, fmt.Errorf("dataset has no transactions")
	}

	excluded := make(map[string]struct{})
	for _, hintedTx := range hint.Txs {
		if hintedTx.Hash == nil {
			continue
		}
		if tx, ok := b.dataset.TransactionByHash(*hintedTx.Hash); ok {
			excluded[nonceKey(tx.From, tx.Nonce)] = struct{}{}
		}
	}

	tx := b.pickNonConflictingTx(block.Transactions, excluded)
	if tx == nil {
		tx = b.pickNonConflictingTx(b.dataset.Transactions(), excluded)
	}
	if tx == nil {
		return nil, fmt.Errorf("no non-conflicting transaction found for backrun")
	}

	rawTx := hexutil.Bytes(tx.RawTx)
	maxBlock := block.Number
	if nextBlock, ok := b.dataset.NextBlockAtOrAfter(block.Number + 1); ok {
		maxBlock = nextBlock.Number
	}

	return &types.SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: types.MevBundleInclusion{
			BlockNumber: hexutil.Uint64(block.Number),
			MaxBlock:    hexutil.Uint64(maxBlock),
		},
		Body: []types.MevBundleBody{
			{Hash: &hint.Hash},
			{Tx: &rawTx},
		},
		Privacy: &types.MevBundlePrivacy{
			Hints: types.HintHash | types.HintTxHash | types.HintLogs,
		},
	}, nil
}

func BuildJSONRPCRequest(method string, params interface{}, id int) ([]byte, error) {
	paramsValue := []interface{}{}
	if params != nil {
		paramsValue = []interface{}{params}
	}

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  paramsValue,
		"id":      id,
	}
	return json.Marshal(request)
}

func BuildHTTPBody(method string, params interface{}, id int) (*bytes.Reader, error) {
	payload, err := BuildJSONRPCRequest(method, params, id)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(payload), nil
}

func (b *ReplayBundleBuilder) pickNonConflictingTx(candidates []*dataset.Transaction, excluded map[string]struct{}) *dataset.Transaction {
	if len(candidates) == 0 {
		return nil
	}

	start := b.random.Intn(len(candidates))
	for offset := 0; offset < len(candidates); offset++ {
		tx := candidates[(start+offset)%len(candidates)]
		if _, blocked := excluded[nonceKey(tx.From, tx.Nonce)]; blocked {
			continue
		}
		return tx
	}
	return nil
}

func nonceKey(from common.Address, nonce uint64) string {
	return fmt.Sprintf("%s:%d", from.Hex(), nonce)
}

func GetCurrentBlock(serverURL string) (uint64, error) {
	body, err := BuildHTTPBody("eth_blockNumber", nil, 1)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(serverURL, "application/json", body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result types.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if result.Error != nil {
		return 0, fmt.Errorf("%s", result.Error.Message)
	}

	blockHex, ok := result.Result.(string)
	if !ok {
		bytes, err := json.Marshal(result.Result)
		if err != nil {
			return 0, err
		}
		if err := json.Unmarshal(bytes, &blockHex); err != nil {
			return 0, err
		}
	}

	return hexutil.DecodeUint64(blockHex)
}
