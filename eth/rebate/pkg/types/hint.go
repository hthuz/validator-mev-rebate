package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// ============== Hint 相关结构 ==============

// HintIntent 定义要提取的 Hint 类型 (位掩码)
type HintIntent uint8

const (
	HintNone             HintIntent = 0
	HintContractAddress  HintIntent = 1 << 0 // 合约地址
	HintFunctionSelector HintIntent = 1 << 1 // 函数选择器 (4字节)
	HintLogs             HintIntent = 1 << 2 // 所有日志
	HintCallData         HintIntent = 1 << 3 // 调用数据
	HintHash             HintIntent = 1 << 4 // 哈希 (必需)
	HintSpecialLogs      HintIntent = 1 << 5 // 特殊日志 (DEX 相关)
	HintTxHash           HintIntent = 1 << 6 // 交易哈希
)

// Has 检查是否包含某个 Hint 类型
func (h HintIntent) Has(flag HintIntent) bool {
	return h&flag != 0
}

// Hint 提取的 Hint 信息
type Hint struct {
	Hash        common.Hash     `json:"hash"`                  // 匹配哈希
	Logs        []CleanLog      `json:"logs,omitempty"`        // 日志
	Txs         []TxHint        `json:"txs,omitempty"`         // 交易提示
	MevGasPrice *hexutil.Big    `json:"mevGasPrice,omitempty"` // MEV Gas Price
	GasUsed     *hexutil.Uint64 `json:"gasUsed,omitempty"`     // Gas 使用量
}

// TxHint 交易的 Hint 信息
type TxHint struct {
	Hash             *common.Hash    `json:"hash,omitempty"`
	To               *common.Address `json:"to,omitempty"`
	FunctionSelector *hexutil.Bytes  `json:"functionSelector,omitempty"`
	CallData         *hexutil.Bytes  `json:"callData,omitempty"`
}

// CleanLog 清理后的日志 (移除敏感信息)
type CleanLog struct {
	Address common.Address `json:"address"`
	Topics  []common.Hash  `json:"topics,omitempty"`
	Data    hexutil.Bytes  `json:"data,omitempty"`
}
