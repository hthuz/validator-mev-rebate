package metrics

import (
	"math/big"
	"time"
)

// GlobalMetrics 全局统计
type GlobalMetrics struct {
	TotalBlocks      uint64   `json:"totalBlocks"`
	TotalBundles     uint64   `json:"totalBundles"`
	TotalMevProfit   *big.Int `json:"totalMevProfit"`
	TotalRefunded    *big.Int `json:"totalRefunded"`
	UniqueValidators uint64   `json:"uniqueValidators"`
	UniqueSearchers  uint64   `json:"uniqueSearchers"`

	// 时间范围
	StartTime time.Time `json:"startTime"`
	UpdatedAt time.Time `json:"updatedAt"`
}
