package metrics

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ============== Validator 历史表现 ==============

// ValidatorMetrics Validator 历史表现指标
type ValidatorMetrics struct {
	Address        common.Address `json:"address"`
	FirstSeenBlock uint64         `json:"firstSeenBlock"`
	LastSeenBlock  uint64         `json:"lastSeenBlock"`
	UpdatedAt      time.Time      `json:"updatedAt"`

	// 出块统计
	TotalBlocks uint64 `json:"totalBlocks"` // 出块总数
	MevBlocks   uint64 `json:"mevBlocks"`   // 包含 MEV 的区块数

	// 收益统计
	TotalMevRevenue   *big.Int `json:"totalMevRevenue"`   // 累计 MEV 收益
	TotalRefunded     *big.Int `json:"totalRefunded"`     // 累计退款
	AvgMevPerBlock    *big.Int `json:"avgMevPerBlock"`    // 平均每块 MEV
	AvgMevPerMevBlock *big.Int `json:"avgMevPerMevBlock"` // 平均有 MEV 的区块收益

	// 效率指标
	MevCaptureRate    float64 `json:"mevCaptureRate"`    // 实际捕获 MEV / 理论最大 MEV (需要外部数据)
	ParticipationRate float64 `json:"participationRate"` // 参与 MEV 区块的比例

	// Bundle 统计
	TotalBundles   uint64  `json:"totalBundles"`   // 总 bundle 数
	SuccessBundles uint64  `json:"successBundles"` // 成功 bundle 数
	SuccessRate    float64 `json:"successRate"`    // 成功率

	// 历史趋势 (保留最近 100 个区块)
	RecentBlocks []BlockMevSummary `json:"recentBlocks,omitempty"`
}

// BlockMevSummary 区块 MEV 摘要
type BlockMevSummary struct {
	BlockNumber uint64   `json:"blockNumber"`
	MevProfit   *big.Int `json:"mevProfit"`
	BundleCount int      `json:"bundleCount"`
	Timestamp   int64    `json:"timestamp"`
}

// NewValidatorMetrics 创建新的 Validator 指标
func NewValidatorMetrics(address common.Address, firstBlock uint64) *ValidatorMetrics {
	return &ValidatorMetrics{
		Address:         address,
		FirstSeenBlock:  firstBlock,
		LastSeenBlock:   firstBlock,
		UpdatedAt:       time.Now(),
		TotalMevRevenue: big.NewInt(0),
		TotalRefunded:   big.NewInt(0),
		AvgMevPerBlock:  big.NewInt(0),
		RecentBlocks:    make([]BlockMevSummary, 0, 100),
	}
}

// UpdateWithBlock 用区块指标更新 Validator 统计
func (v *ValidatorMetrics) UpdateWithBlock(block *BlockMevMetrics) {
	v.LastSeenBlock = block.BlockNumber
	v.UpdatedAt = time.Now()
	v.TotalBlocks++

	if block.BundleCount > 0 {
		v.MevBlocks++
		v.TotalMevRevenue.Add(v.TotalMevRevenue, block.TotalMevProfit)
		v.TotalRefunded.Add(v.TotalRefunded, block.TotalRefundable)
		v.TotalBundles += uint64(block.BundleCount)
		v.SuccessBundles += uint64(block.SuccessCount)
	}

	// 计算平均收益
	if v.TotalBlocks > 0 {
		v.AvgMevPerBlock = new(big.Int).Div(v.TotalMevRevenue, big.NewInt(int64(v.TotalBlocks)))
	}
	if v.MevBlocks > 0 {
		v.AvgMevPerMevBlock = new(big.Int).Div(v.TotalMevRevenue, big.NewInt(int64(v.MevBlocks)))
	}

	// 计算成功率
	if v.TotalBundles > 0 {
		v.SuccessRate = float64(v.SuccessBundles) / float64(v.TotalBundles)
	}

	// 参与率
	v.ParticipationRate = float64(v.MevBlocks) / float64(v.TotalBlocks)

	// 添加近期区块记录
	summary := BlockMevSummary{
		BlockNumber: block.BlockNumber,
		MevProfit:   new(big.Int).Set(block.TotalMevProfit),
		BundleCount: block.BundleCount,
		Timestamp:   block.Timestamp.Unix(),
	}
	v.RecentBlocks = append(v.RecentBlocks, summary)
	if len(v.RecentBlocks) > 100 {
		v.RecentBlocks = v.RecentBlocks[1:]
	}
}
