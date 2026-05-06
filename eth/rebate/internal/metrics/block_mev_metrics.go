package metrics

import (
	"math/big"
	"rebate/pkg/types"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ============== 区块级 MEV 指标 ==============

// BlockMevMetrics 单个区块的 MEV 指标
type BlockMevMetrics struct {
	BlockNumber      uint64         `json:"blockNumber"`
	ValidatorAddress common.Address `json:"validatorAddress"`
	Timestamp        time.Time      `json:"timestamp"`

	// 基础收益
	TotalMevProfit  *big.Int `json:"totalMevProfit"`  // 该区块所有 bundles 的总利润
	TotalRefundable *big.Int `json:"totalRefundable"` // 应退还给 validator 的金额
	TotalGasUsed    uint64   `json:"totalGasUsed"`    // MEV 消耗的总 gas

	// 效率指标
	MevGasPrice    *big.Int `json:"mevGasPrice"`    // 平均 MEV gas price (wei)
	BlockSpaceUsed float64  `json:"blockSpaceUsed"` // MEV 占区块 gas limit 的比例

	// Bundle 统计
	BundleCount  int `json:"bundleCount"`  // 该区块包含的 bundle 数量
	SuccessCount int `json:"successCount"` // 成功执行的 bundle
	FailedCount  int `json:"failedCount"`  // 失败的 bundle

	// Builder 分布
	BuilderDistribution map[string]int `json:"builderDistribution"` // 各 builder 的 bundle 数量
}

// NewBlockMevMetrics 创建新的区块指标
func NewBlockMevMetrics(blockNumber uint64, validator common.Address) *BlockMevMetrics {
	return &BlockMevMetrics{
		BlockNumber:         blockNumber,
		ValidatorAddress:    validator,
		Timestamp:           time.Now(),
		TotalMevProfit:      big.NewInt(0),
		TotalRefundable:     big.NewInt(0),
		MevGasPrice:         big.NewInt(0),
		BuilderDistribution: make(map[string]int),
	}
}

// AddBundleResult 添加 bundle 结果到区块指标
func (b *BlockMevMetrics) AddBundleResult(result *types.SimMevBundleResponse, builder string) {
	b.BundleCount++

	if result.Success {
		b.SuccessCount++
		profit := result.Profit.ToInt()
		refundable := result.RefundableValue.ToInt()
		gasUsed := uint64(result.GasUsed)

		b.TotalMevProfit.Add(b.TotalMevProfit, profit)
		b.TotalRefundable.Add(b.TotalRefundable, refundable)
		b.TotalGasUsed += gasUsed

		// 更新平均 MEV gas price (加权平均)
		if b.TotalGasUsed > 0 {
			mevGasPrice := new(big.Int).Div(profit, big.NewInt(int64(gasUsed)))
			b.MevGasPrice = weightedAverage(b.MevGasPrice, mevGasPrice, int64(b.TotalGasUsed-gasUsed), int64(gasUsed))
		}
	} else {
		b.FailedCount++
	}

	// 记录 builder 分布
	if builder != "" {
		b.BuilderDistribution[builder]++
	}
}

// Finalize 计算最终指标
func (b *BlockMevMetrics) Finalize(blockGasLimit uint64) {
	if blockGasLimit > 0 {
		b.BlockSpaceUsed = float64(b.TotalGasUsed) / float64(blockGasLimit)
	}
}
