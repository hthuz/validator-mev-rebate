package metrics

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// SearcherMetrics 搜索者指标
type SearcherMetrics struct {
	Address        common.Address `json:"address"`
	TotalBundles   uint64         `json:"totalBundles"`
	SuccessBundles uint64         `json:"successBundles"`
	FailedBundles  uint64         `json:"failedBundles"`
	SuccessRate    float64        `json:"successRate"`

	TotalProfit    *big.Int `json:"totalProfit"`
	AvgProfit      *big.Int `json:"avgProfit"`
	BackrunCount   uint64   `json:"backrunCount"`
	BackrunSuccess uint64   `json:"backrunSuccess"`
	BackrunProfit  *big.Int `json:"backrunProfit"`

	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}
