package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// ============== Bundle 核心结构 ==============

// SendMevBundleArgs MEV-Share Bundle 的核心结构
type SendMevBundleArgs struct {
	Version         string             `json:"version"`
	ReplacementUUID string             `json:"replacementUuid,omitempty"`
	Inclusion       MevBundleInclusion `json:"inclusion"`
	Body            []MevBundleBody    `json:"body"`
	Validity        MevBundleValidity  `json:"validity,omitempty"`
	Privacy         *MevBundlePrivacy  `json:"privacy,omitempty"`
	Metadata        *MevBundleMetadata `json:"metadata,omitempty"`
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

// ============== API 响应结构 ==============

// SendMevBundleResponse SendBundle 的响应
type SendMevBundleResponse struct {
	BundleHash common.Hash `json:"bundleHash"`
}

// CancelBundleResponse 取消 Bundle 的响应
type CancelBundleResponse struct {
	Cancelled []common.Hash `json:"cancelled"`
}
