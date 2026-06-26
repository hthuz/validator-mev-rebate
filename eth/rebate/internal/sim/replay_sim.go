package sim

import (
	"context"
	"fmt"
	"math/big"
	"rebate/internal/dataset"
	"rebate/mylog"
	"rebate/pkg/types"
	"rebate/pkg/utils"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
)

const defaultReplayRefundPercent = int64(10)

type ReplaySimulator struct {
	dataset       *dataset.Dataset
	blockGasLimit uint64

	mu           sync.RWMutex
	currentBlock uint64
	currentIndex int
}

type candidateTx struct {
	hash                common.Hash
	from                common.Address
	to                  *common.Address
	nonce               uint64
	gasUsed             uint64
	effectiveGasPrice   *big.Int
	priorityFeePerGas   *big.Int
	rawTx               []byte
	source              string
	logsCount           int
	matchedHistoricalTx bool
}

func NewReplaySimulator(data *dataset.Dataset, blockGasLimit uint64) (*ReplaySimulator, error) {
	firstBlock, ok := data.FirstBlock()
	if !ok {
		return nil, fmt.Errorf("replay dataset is empty")
	}
	if blockGasLimit == 0 {
		blockGasLimit = 30000000
	}

	return &ReplaySimulator{
		dataset:       data,
		blockGasLimit: blockGasLimit,
		currentBlock:  firstBlock.Number - 1,
		currentIndex:  -1,
	}, nil
}

func (r *ReplaySimulator) SimulateBundle(ctx context.Context, bundle *types.SendMevBundleArgs, overrides map[string]interface{}) (*types.SimMevBundleResponse, error) {
	_ = ctx

	targetBlock, err := r.resolveTargetBlock(uint64(bundle.Inclusion.BlockNumber))
	if err != nil {
		return nil, err
	}

	bundleTxs, err := flattenBundleTransactions(bundle.Body, targetBlock.BaseFee)
	if err != nil {
		return nil, err
	}
	if len(bundleTxs) == 0 {
		return nil, fmt.Errorf("bundle has no executable transactions")
	}

	historical := make([]*candidateTx, 0, len(targetBlock.Transactions))
	conflictedHistorical := make([]common.Hash, 0)
	for _, tx := range targetBlock.Transactions {
		candidate := candidateFromHistorical(tx)
		if conflictsWithBundleNonce(candidate, bundleTxs) {
			conflictedHistorical = append(conflictedHistorical, candidate.hash)
			continue
		}
		historical = append(historical, candidate)
	}

	totalGasUsed := sumGasUsed(bundleTxs)
	totalPriorityFee := sumBundlePriorityFee(bundleTxs)
	mevGasPrice := big.NewInt(0)
	if totalGasUsed > 0 {
		mevGasPrice = new(big.Int).Div(new(big.Int).Set(totalPriorityFee), big.NewInt(int64(totalGasUsed)))
	}

	insertionIndex := resolveInsertionIndex(historical, mevGasPrice)
	combined := make([]*candidateTx, 0, len(historical)+len(bundleTxs))
	combined = append(combined, historical[:insertionIndex]...)
	combined = append(combined, bundleTxs...)
	combined = append(combined, historical[insertionIndex:]...)

	displaced := append([]common.Hash{}, conflictedHistorical...)
	totalCombinedGas := sumGasUsed(combined)
	for totalCombinedGas > r.blockGasLimit && len(combined) > 0 {
		last := combined[len(combined)-1]
		totalCombinedGas -= last.gasUsed
		displaced = append(displaced, last.hash)
		combined = combined[:len(combined)-1]
	}

	positionByHash := make(map[common.Hash]int, len(combined))
	for idx, tx := range combined {
		positionByHash[tx.hash] = idx
	}

	success := true
	execError := ""
	bodyLogs := buildReplayBodyLogs(bundleTxs)
	resultTxs := make([]types.SimulatedTransaction, 0, len(bundleTxs))
	for _, tx := range bundleTxs {
		position, ok := positionByHash[tx.hash]
		if !ok {
			success = false
			execError = "bundle evicted during block reordering"
			break
		}
		resultTxs = append(resultTxs, types.SimulatedTransaction{
			Hash:              tx.hash,
			From:              tx.from,
			To:                tx.to,
			Nonce:             hexutil.Uint64(tx.nonce),
			GasUsed:           hexutil.Uint64(tx.gasUsed),
			EffectiveGasPrice: hexutil.Big(*tx.effectiveGasPrice),
			Position:          hexutil.Uint64(position),
			Source:            tx.source,
			MatchedHistorical: tx.matchedHistoricalTx,
		})
	}

	sort.Slice(resultTxs, func(i, j int) bool {
		return resultTxs[i].Position < resultTxs[j].Position
	})

	blockInfo := &types.SimulatedBlockContext{
		BlockNumber:          hexutil.Uint64(targetBlock.Number),
		BlockHash:            targetBlock.Hash,
		BlockTimestamp:       hexutil.Uint64(targetBlock.Timestamp),
		BaseFee:              hexutil.Big(*targetBlock.BaseFee),
		HistoricalTxCount:    hexutil.Uint64(len(targetBlock.Transactions)),
		BundleInsertionIndex: hexutil.Uint64(insertionIndex),
		DisplacedTxs:         displaced,
		BundleTxs:            resultTxs,
	}

	refundable := new(big.Int).Div(new(big.Int).Mul(totalPriorityFee, big.NewInt(defaultReplayRefundPercent)), big.NewInt(100))
	response := &types.SimMevBundleResponse{
		Success:         success,
		StateBlock:      hexutil.Uint64(r.CurrentBlock()),
		MevGasPrice:     hexutil.Big(*mevGasPrice),
		Profit:          hexutil.Big(*totalPriorityFee),
		RefundableValue: hexutil.Big(*refundable),
		GasUsed:         hexutil.Uint64(totalGasUsed),
		BodyLogs:        bodyLogs,
		ExecError:       execError,
		Block:           blockInfo,
	}

	mylog.Logger.Info().
		Uint64("stateBlock", r.CurrentBlock()).
		Uint64("targetBlock", targetBlock.Number).
		Int("bundleTxs", len(bundleTxs)).
		Int("displacedTxs", len(displaced)).
		Str("mevGasPrice", mevGasPrice.String()).
		Msg("Replay bundle simulated")

	return response, nil
}

func (r *ReplaySimulator) CurrentBlock() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentBlock
}

func (r *ReplaySimulator) AdvanceBlock() (uint64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	nextIndex := r.currentIndex + 1
	if nextIndex >= len(r.dataset.Blocks()) {
		return r.currentBlock, false
	}

	r.currentIndex = nextIndex
	r.currentBlock = r.dataset.Blocks()[nextIndex].Number
	return r.currentBlock, true
}

func (r *ReplaySimulator) BlockGasLimit() uint64 {
	return r.blockGasLimit
}

func (r *ReplaySimulator) resolveTargetBlock(requested uint64) (*dataset.Block, error) {
	if block, ok := r.dataset.BlockByNumber(requested); ok {
		return block, nil
	}
	if block, ok := r.dataset.NextBlockAtOrAfter(requested); ok {
		return block, nil
	}
	return nil, fmt.Errorf("target block %d not found in replay dataset", requested)
}

func flattenBundleTransactions(body []types.MevBundleBody, baseFee *big.Int) ([]*candidateTx, error) {
	var out []*candidateTx
	seenSenderNonce := make(map[string]struct{})

	var walk func([]types.MevBundleBody) error
	walk = func(items []types.MevBundleBody) error {
		for _, item := range items {
			switch {
			case item.Tx != nil:
				tx, err := utils.DecodeTransaction(*item.Tx)
				if err != nil {
					return fmt.Errorf("decode bundle tx: %w", err)
				}
				from, err := etypes.Sender(etypes.LatestSignerForChainID(tx.ChainId()), tx)
				if err != nil {
					return fmt.Errorf("recover sender for tx %s: %w", tx.Hash().Hex(), err)
				}
				key := fmt.Sprintf("%s:%d", from.Hex(), tx.Nonce())
				if _, exists := seenSenderNonce[key]; exists {
					return fmt.Errorf("duplicate sender nonce in bundle: %s", key)
				}
				seenSenderNonce[key] = struct{}{}

				rawTx, err := tx.MarshalBinary()
				if err != nil {
					return fmt.Errorf("marshal bundle tx %s: %w", tx.Hash().Hex(), err)
				}
				out = append(out, &candidateTx{
					hash:              tx.Hash(),
					from:              from,
					to:                tx.To(),
					nonce:             tx.Nonce(),
					gasUsed:           estimateBundleGasUsed(tx),
					effectiveGasPrice: effectiveGasPrice(tx, baseFee),
					priorityFeePerGas: priorityFeePerGas(tx, baseFee),
					rawTx:             rawTx,
					source:            "bundle",
				})
			case item.Bundle != nil:
				if err := walk(item.Bundle.Body); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(body); err != nil {
		return nil, err
	}
	return out, nil
}

func candidateFromHistorical(tx *dataset.Transaction) *candidateTx {
	return &candidateTx{
		hash:                tx.Hash,
		from:                tx.From,
		to:                  tx.To,
		nonce:               tx.Nonce,
		gasUsed:             tx.ReceiptGasUsed,
		effectiveGasPrice:   cloneBigInt(tx.EffectiveGasPrice),
		priorityFeePerGas:   historicalPriorityFee(tx),
		rawTx:               tx.RawTx,
		source:              "historical",
		logsCount:           tx.LogsCount,
		matchedHistoricalTx: true,
	}
}

func historicalPriorityFee(tx *dataset.Transaction) *big.Int {
	priority := new(big.Int).Sub(tx.EffectiveGasPrice, tx.BlockBaseFee)
	if priority.Sign() < 0 {
		return big.NewInt(0)
	}
	return priority
}

func conflictsWithBundleNonce(historical *candidateTx, bundleTxs []*candidateTx) bool {
	for _, tx := range bundleTxs {
		if tx.from == historical.from && tx.nonce == historical.nonce {
			return true
		}
	}
	return false
}

func resolveInsertionIndex(historical []*candidateTx, mevGasPrice *big.Int) int {
	for idx, tx := range historical {
		if mevGasPrice.Cmp(tx.priorityFeePerGas) > 0 {
			return idx
		}
	}
	return len(historical)
}

func sumGasUsed(txs []*candidateTx) uint64 {
	var total uint64
	for _, tx := range txs {
		total += tx.gasUsed
	}
	return total
}

func sumBundlePriorityFee(txs []*candidateTx) *big.Int {
	total := big.NewInt(0)
	for _, tx := range txs {
		fee := new(big.Int).Mul(tx.priorityFeePerGas, big.NewInt(int64(tx.gasUsed)))
		total.Add(total, fee)
	}
	return total
}

func buildReplayBodyLogs(bundleTxs []*candidateTx) []types.SimMevBodyLogs {
	result := make([]types.SimMevBodyLogs, 0, len(bundleTxs))
	for _, tx := range bundleTxs {
		logs := types.SimMevBodyLogs{}
		if tx.to != nil {
			logs.TxLogs = append(logs.TxLogs, types.SimLog{
				Address: *tx.to,
			})
		}
		result = append(result, logs)
	}
	return result
}

func estimateBundleGasUsed(tx *etypes.Transaction) uint64 {
	intrinsic := uint64(21000)
	for _, b := range tx.Data() {
		if b == 0 {
			intrinsic += 4
		} else {
			intrinsic += 16
		}
	}
	if tx.Gas() < intrinsic {
		return tx.Gas()
	}
	return intrinsic
}

func effectiveGasPrice(tx *etypes.Transaction, baseFee *big.Int) *big.Int {
	switch tx.Type() {
	case etypes.LegacyTxType, etypes.AccessListTxType:
		return cloneBigInt(tx.GasPrice())
	default:
		candidate := new(big.Int).Add(baseFee, tx.GasTipCap())
		if candidate.Cmp(tx.GasFeeCap()) > 0 {
			return cloneBigInt(tx.GasFeeCap())
		}
		return candidate
	}
}

func priorityFeePerGas(tx *etypes.Transaction, baseFee *big.Int) *big.Int {
	price := effectiveGasPrice(tx, baseFee)
	priority := new(big.Int).Sub(price, baseFee)
	if priority.Sign() < 0 {
		return big.NewInt(0)
	}
	return priority
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(value)
}
