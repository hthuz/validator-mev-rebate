package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// ============== 模拟结果结构 ==============

// SimMevBundleResponse 模拟结果
type SimMevBundleResponse struct {
	Success         bool                   `json:"success"`
	Error           string                 `json:"error,omitempty"`
	StateBlock      hexutil.Uint64         `json:"stateBlock"`
	MevGasPrice     hexutil.Big            `json:"mevGasPrice"`
	Profit          hexutil.Big            `json:"profit"`
	RefundableValue hexutil.Big            `json:"refundableValue"`
	GasUsed         hexutil.Uint64         `json:"gasUsed"`
	BodyLogs        []SimMevBodyLogs       `json:"logs,omitempty"`
	ExecError       string                 `json:"execError,omitempty"`
	Revert          hexutil.Bytes          `json:"revert,omitempty"`
	Block           *SimulatedBlockContext `json:"block,omitempty"`
}

// SimMevBodyLogs 模拟执行的日志
type SimMevBodyLogs struct {
	TxLogs     []SimLog         `json:"txLogs,omitempty"`
	BundleLogs []SimMevBodyLogs `json:"bundleLogs,omitempty"`
}

// SimLog 单条日志
type SimLog struct {
	Address common.Address `json:"address"`
	Topics  []common.Hash  `json:"topics"`
	Data    hexutil.Bytes  `json:"data"`
}

type SimulatedBlockContext struct {
	BlockNumber          hexutil.Uint64         `json:"blockNumber"`
	BlockHash            common.Hash            `json:"blockHash"`
	BlockTimestamp       hexutil.Uint64         `json:"blockTimestamp"`
	BaseFee              hexutil.Big            `json:"baseFee"`
	HistoricalTxCount    hexutil.Uint64         `json:"historicalTxCount"`
	BundleInsertionIndex hexutil.Uint64         `json:"bundleInsertionIndex"`
	DisplacedTxs         []common.Hash          `json:"displacedTxs,omitempty"`
	BundleTxs            []SimulatedTransaction `json:"bundleTxs,omitempty"`
}

type SimulatedTransaction struct {
	Hash              common.Hash     `json:"hash"`
	From              common.Address  `json:"from"`
	To                *common.Address `json:"to,omitempty"`
	Nonce             hexutil.Uint64  `json:"nonce"`
	GasUsed           hexutil.Uint64  `json:"gasUsed"`
	EffectiveGasPrice hexutil.Big     `json:"effectiveGasPrice"`
	Position          hexutil.Uint64  `json:"position"`
	Source            string          `json:"source"`
	MatchedHistorical bool            `json:"matchedHistorical,omitempty"`
}
