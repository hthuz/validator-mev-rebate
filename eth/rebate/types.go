package main

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// ============== Bundle 核心结构 ==============

// SendMevBundleArgs MEV-Share Bundle 的核心结构
type SendMevBundleArgs struct {
	Version         string              `json:"version"`
	ReplacementUUID string              `json:"replacementUuid,omitempty"`
	Inclusion       MevBundleInclusion  `json:"inclusion"`
	Body            []MevBundleBody     `json:"body"`
	Validity        MevBundleValidity   `json:"validity,omitempty"`
	Privacy         *MevBundlePrivacy   `json:"privacy,omitempty"`
	Metadata        *MevBundleMetadata  `json:"metadata,omitempty"`
}

// MevBundleInclusion 定义 Bundle 目标区块范围
type MevBundleInclusion struct {
	BlockNumber hexutil.Uint64 `json:"block"`
	MaxBlock    hexutil.Uint64 `json:"maxBlock"`
}

// MevBundleBody Bundle 主体元素
type MevBundleBody struct {
	Hash      *common.Hash       `json:"hash,omitempty"`      // 引用另一个 Bundle 的哈希 (用于 backrunning)
	Tx        *hexutil.Bytes     `json:"tx,omitempty"`        // 原始交易数据
	Bundle    *SendMevBundleArgs `json:"bundle,omitempty"`    // 嵌套 Bundle
	CanRevert bool               `json:"canRevert,omitempty"` // 是否允许交易回滚
}

// MevBundleValidity Bundle 有效性约束
type MevBundleValidity struct {
	Refund       []RefundConfig    `json:"refund,omitempty"`
	RefundConfig []RefundRecipient `json:"refundConfig,omitempty"`
}

// RefundConfig 退款配置
type RefundConfig struct {
	BodyIdx int `json:"bodyIdx"`
	Percent int `json:"percent"`
}

// RefundRecipient 退款接收者
type RefundRecipient struct {
	Address common.Address `json:"address"`
	Percent int            `json:"percent"`
}

// MevBundlePrivacy 隐私和分发配置
type MevBundlePrivacy struct {
	Hints      HintIntent `json:"hints,omitempty"`      // 要提取的 Hint 类型
	Builders   []string   `json:"builders,omitempty"`   // 目标 Builder 列表
	WantRefund *int       `json:"wantRefund,omitempty"` // 期望的退款百分比
}

// MevBundleMetadata 节点生成的元数据
type MevBundleMetadata struct {
	BundleHash       common.Hash    `json:"bundleHash"`
	BodyHashes       []common.Hash  `json:"bodyHashes,omitempty"`
	Signer           common.Address `json:"signer,omitempty"`
	OriginID         string         `json:"originId,omitempty"`
	ReceivedAt       hexutil.Uint64 `json:"receivedAt,omitempty"`
	MatchingHash     common.Hash    `json:"matchingHash,omitempty"`
	Prematched       bool           `json:"prematched,omitempty"`
	ReplacementNonce uint64         `json:"replacementNonce,omitempty"`
	Cancelled        bool           `json:"cancelled,omitempty"`
}

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
	Hash        common.Hash       `json:"hash"`                  // 匹配哈希
	Logs        []CleanLog        `json:"logs,omitempty"`        // 日志
	Txs         []TxHint          `json:"txs,omitempty"`         // 交易提示
	MevGasPrice *hexutil.Big      `json:"mevGasPrice,omitempty"` // MEV Gas Price
	GasUsed     *hexutil.Uint64   `json:"gasUsed,omitempty"`     // Gas 使用量
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

// ============== 模拟结果结构 ==============

// SimMevBundleResponse 模拟结果
type SimMevBundleResponse struct {
	Success         bool              `json:"success"`
	Error           string            `json:"error,omitempty"`
	StateBlock      hexutil.Uint64    `json:"stateBlock"`
	MevGasPrice     hexutil.Big       `json:"mevGasPrice"`
	Profit          hexutil.Big       `json:"profit"`
	RefundableValue hexutil.Big       `json:"refundableValue"`
	GasUsed         hexutil.Uint64    `json:"gasUsed"`
	BodyLogs        []SimMevBodyLogs  `json:"logs,omitempty"`
	ExecError       string            `json:"execError,omitempty"`
	Revert          hexutil.Bytes     `json:"revert,omitempty"`
}

// SimMevBodyLogs 模拟执行的日志
type SimMevBodyLogs struct {
	TxLogs  []SimLog `json:"txLogs,omitempty"`
	BundleLogs []SimMevBodyLogs `json:"bundleLogs,omitempty"`
}

// SimLog 单条日志
type SimLog struct {
	Address common.Address `json:"address"`
	Topics  []common.Hash  `json:"topics"`
	Data    hexutil.Bytes  `json:"data"`
}

// ============== API 响应结构 ==============

// SendMevBundleResponse SendBundle 的响应
type SendMevBundleResponse struct {
	BundleHash common.Hash `json:"bundleHash"`
}

// CancelBundleResponse 取消 Bundle 的响应
type CancelBundleResponse struct {
	Cancelled []common.Hash `json:"cancelled"`
}
