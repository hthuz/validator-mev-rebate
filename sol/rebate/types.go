package main

import (
	"encoding/base64"
	"encoding/json"
)

// ============== Bundle 核心结构 ==============

// SendMevBundleArgs MEV-Share Bundle 的核心结构 (Solana 版本)
type SendMevBundleArgs struct {
	Version         string             `json:"version"`
	ReplacementUUID string             `json:"replacementUuid,omitempty"`
	Inclusion       MevBundleInclusion `json:"inclusion"`
	Body            []MevBundleBody    `json:"body"`
	Validity        MevBundleValidity  `json:"validity,omitempty"`
	Privacy         *MevBundlePrivacy  `json:"privacy,omitempty"`
	Metadata        *MevBundleMetadata `json:"metadata,omitempty"`
}

// MevBundleInclusion 定义 Bundle 目标 slot 范围 (Solana 使用 slot 而不是 block)
type MevBundleInclusion struct {
	Slot     uint64 `json:"slot"`     // 目标 slot
	MaxSlot  uint64 `json:"maxSlot"`  // 最大 slot
}

// MevBundleBody Bundle 主体元素
type MevBundleBody struct {
	Hash      string             `json:"hash,omitempty"`      // 引用另一个 Bundle 的哈希 (用于 backrunning)
	Tx        string             `json:"tx,omitempty"`        // base64 编码的交易数据
	Bundle    *SendMevBundleArgs `json:"bundle,omitempty"`    // 嵌套 Bundle
	CanRevert bool               `json:"canRevert,omitempty"` // 是否允许交易失败
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

// RefundRecipient 退款接收者 (Solana 使用 base58 地址)
type RefundRecipient struct {
	Address string `json:"address"` // base58 编码的 Solana 地址
	Percent int    `json:"percent"`
}

// MevBundlePrivacy 隐私和分发配置
type MevBundlePrivacy struct {
	Hints      HintIntent `json:"hints,omitempty"`      // 要提取的 Hint 类型
	Builders   []string   `json:"builders,omitempty"`   // 目标 Builder 列表
	WantRefund *int       `json:"wantRefund,omitempty"` // 期望的退款百分比
}

// MevBundleMetadata 节点生成的元数据
type MevBundleMetadata struct {
	BundleHash       string `json:"bundleHash"`                 // Bundle 哈希 (base58)
	BodyHashes       []string `json:"bodyHashes,omitempty"`     // body 元素哈希列表
	Signer           string `json:"signer,omitempty"`           // 签名者地址
	OriginID         string `json:"originId,omitempty"`         // 来源标识
	ReceivedAt       int64  `json:"receivedAt,omitempty"`       // 接收时间 (微秒)
	MatchingHash     string `json:"matchingHash,omitempty"`     // 用于匹配的哈希
	Prematched       bool   `json:"prematched,omitempty"`       // 是否预匹配
	ReplacementNonce uint64 `json:"replacementNonce,omitempty"`
	Cancelled        bool   `json:"cancelled,omitempty"`
}

// ============== Hint 相关结构 ==============

// HintIntent 定义要提取的 Hint 类型 (位掩码)
type HintIntent uint8

const (
	HintNone             HintIntent = 0
	HintProgramID        HintIntent = 1 << 0 // 程序 ID (类似以太坊的合约地址)
	HintInstructionType  HintIntent = 1 << 1 // 指令类型
	HintLogs             HintIntent = 1 << 2 // 所有日志
	HintTransactionData  HintIntent = 1 << 3 // 交易数据
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
	Hash           string       `json:"hash"`                     // 匹配哈希
	Logs           []CleanLog   `json:"logs,omitempty"`           // 日志
	Txs            []TxHint     `json:"txs,omitempty"`            // 交易提示
	PriorityFee    uint64       `json:"priorityFee,omitempty"`    // Solana priority fee (替代 mevGasPrice)
	ComputeUnits   uint64       `json:"computeUnits,omitempty"`   // 计算单元使用量
}

// TxHint 交易的 Hint 信息
type TxHint struct {
	Hash             string   `json:"hash,omitempty"`             // 交易签名 (base58)
	ProgramID        string   `json:"programId,omitempty"`        // 主要程序 ID
	InstructionType  string   `json:"instructionType,omitempty"`  // 指令类型
	Data             string   `json:"data,omitempty"`             // 数据 (base64)
}

// CleanLog 清理后的日志 (移除敏感信息)
type CleanLog struct {
	Program string   `json:"program"`           // 程序地址
	Events  []string `json:"events,omitempty"`  // 事件签名
	Data    string   `json:"data,omitempty"`    // 数据 (可能已清理)
}

// ============== 模拟结果结构 ==============

// SimMevBundleResponse 模拟结果
type SimMevBundleResponse struct {
	Success          bool             `json:"success"`
	Error            string           `json:"error,omitempty"`
	StateSlot        uint64           `json:"stateSlot"`           // 模拟时的 slot
	PriorityFee      uint64           `json:"priorityFee"`         // 支付的 priority fee
	Profit           int64            `json:"profit"`              // 模拟利润 (lamports)
	RefundableValue  int64            `json:"refundableValue"`     // 可退款金额
	ComputeUnitsUsed uint64           `json:"computeUnitsUsed"`    // 计算单元使用量
	BodyLogs         []SimMevBodyLogs `json:"logs,omitempty"`
	ExecError        string           `json:"execError,omitempty"`
	Revert           string           `json:"revert,omitempty"`
}

// SimMevBodyLogs 模拟执行的日志
type SimMevBodyLogs struct {
	TxLogs     []SimLog         `json:"txLogs,omitempty"`
	BundleLogs []SimMevBodyLogs `json:"bundleLogs,omitempty"`
}

// SimLog 单条日志
type SimLog struct {
	Program string   `json:"program"`
	Events  []string `json:"events"`
	Data    string   `json:"data"`
}

// ============== API 响应结构 ==============

// SendMevBundleResponse SendBundle 的响应
type SendMevBundleResponse struct {
	BundleHash string `json:"bundleHash"`
}

// CancelBundleResponse 取消 Bundle 的响应
type CancelBundleResponse struct {
	Cancelled []string `json:"cancelled"`
}

// ============== 辅助函数 ==============

// DecodeTransaction 解码 base64 交易数据
func DecodeTransaction(txBase64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(txBase64)
}

// EncodeTransaction 编码交易数据为 base64
func EncodeTransaction(tx []byte) string {
	return base64.StdEncoding.EncodeToString(tx)
}

// SerializeBundle 序列化 Bundle
func SerializeBundle(bundle *SendMevBundleArgs) ([]byte, error) {
	return json.Marshal(bundle)
}
