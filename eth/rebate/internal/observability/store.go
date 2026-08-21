package observability

import (
	"math/big"
	"rebate/pkg/types"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// MetricsStore stores in-memory aggregate metrics for HTTP queries.
type MetricsStore struct {
	mu sync.RWMutex

	blockMetrics     map[uint64]*BlockMevMetrics
	validatorMetrics map[common.Address]*ValidatorMetrics
	searcherMetrics  map[common.Address]*SearcherMetrics
	globalStats      *GlobalMetrics
	currentBlock     uint64
}

func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		blockMetrics:     make(map[uint64]*BlockMevMetrics),
		validatorMetrics: make(map[common.Address]*ValidatorMetrics),
		searcherMetrics:  make(map[common.Address]*SearcherMetrics),
		globalStats: &GlobalMetrics{
			TotalMevProfit: big.NewInt(0),
			TotalRefunded:  big.NewInt(0),
			StartTime:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
}

func (m *MetricsStore) StartNewBlock(blockNumber uint64, validator common.Address) *BlockMevMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := newBlockMevMetrics(blockNumber, validator)
	m.blockMetrics[blockNumber] = metrics
	m.currentBlock = blockNumber

	if _, exists := m.validatorMetrics[validator]; !exists {
		m.validatorMetrics[validator] = newValidatorMetrics(validator, blockNumber)
		m.globalStats.UniqueValidators++
	}

	return metrics
}

func (m *MetricsStore) FinalizeBlock(blockNumber uint64, blockGasLimit uint64) (*BlockSummaryEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.blockMetrics[blockNumber]
	if !exists {
		return nil, false
	}

	metrics.finalize(blockGasLimit)

	if validator, ok := m.validatorMetrics[metrics.ValidatorAddress]; ok {
		validator.updateWithBlock(metrics)
	}

	m.globalStats.TotalBlocks++
	m.globalStats.TotalBundles += uint64(metrics.BundleCount)
	m.globalStats.TotalMevProfit.Add(m.globalStats.TotalMevProfit, metrics.TotalMevProfit)
	m.globalStats.TotalRefunded.Add(m.globalStats.TotalRefunded, metrics.TotalRefundable)
	m.globalStats.UpdatedAt = time.Now()

	successRate := 0.0
	if metrics.BundleCount > 0 {
		successRate = float64(metrics.SuccessCount) / float64(metrics.BundleCount)
	}

	builderDistribution := make(map[string]int, len(metrics.BuilderDistribution))
	for builder, count := range metrics.BuilderDistribution {
		builderDistribution[builder] = count
	}

	return &BlockSummaryEvent{
		RecordedAt:          time.Now(),
		BlockNumber:         metrics.BlockNumber,
		Validator:           metrics.ValidatorAddress.Hex(),
		BlockTimestamp:      metrics.Timestamp,
		BundleCount:         metrics.BundleCount,
		SuccessCount:        metrics.SuccessCount,
		FailedCount:         metrics.FailedCount,
		SuccessRate:         successRate,
		TotalMevProfitWei:   metrics.TotalMevProfit.String(),
		TotalRefundableWei:  metrics.TotalRefundable.String(),
		TotalGasUsed:        metrics.TotalGasUsed,
		MevGasPriceWei:      metrics.MevGasPrice.String(),
		BlockSpaceUsed:      metrics.BlockSpaceUsed,
		UniqueBuilders:      len(metrics.BuilderDistribution),
		BuilderDistribution: builderDistribution,
	}, true
}

func (m *MetricsStore) RecordBundleResult(blockNumber uint64, result *types.SimMevBundleResponse, builder string, searcher common.Address) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if block, exists := m.blockMetrics[blockNumber]; exists {
		block.addBundleResult(result, builder)
	}

	m.updateSearcherMetrics(searcher, result)
}

func (m *MetricsStore) updateSearcherMetrics(searcher common.Address, result *types.SimMevBundleResponse) {
	sm, exists := m.searcherMetrics[searcher]
	if !exists {
		sm = &SearcherMetrics{
			Address:       searcher,
			FirstSeen:     time.Now(),
			TotalProfit:   big.NewInt(0),
			AvgProfit:     big.NewInt(0),
			BackrunProfit: big.NewInt(0),
		}
		m.searcherMetrics[searcher] = sm
		m.globalStats.UniqueSearchers++
	}

	sm.LastSeen = time.Now()
	sm.TotalBundles++

	if result.Success {
		sm.SuccessBundles++
		profit := result.Profit.ToInt()
		sm.TotalProfit.Add(sm.TotalProfit, profit)

		if sm.TotalBundles > 0 {
			sm.AvgProfit = new(big.Int).Div(sm.TotalProfit, big.NewInt(int64(sm.TotalBundles)))
		}
	} else {
		sm.FailedBundles++
	}

	if sm.TotalBundles > 0 {
		sm.SuccessRate = float64(sm.SuccessBundles) / float64(sm.TotalBundles)
	}
}

func (m *MetricsStore) GetBlockMetrics(blockNumber uint64) (*BlockMevMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.blockMetrics[blockNumber]
	return metrics, exists
}

func (m *MetricsStore) GetValidatorMetrics(address common.Address) (*ValidatorMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.validatorMetrics[address]
	return metrics, exists
}

func (m *MetricsStore) GetAllValidatorMetrics() map[common.Address]*ValidatorMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[common.Address]*ValidatorMetrics, len(m.validatorMetrics))
	for k, v := range m.validatorMetrics {
		result[k] = v
	}
	return result
}

func (m *MetricsStore) GetSearcherMetrics(address common.Address) (*SearcherMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.searcherMetrics[address]
	return metrics, exists
}

func (m *MetricsStore) GetAllSearcherMetrics() map[common.Address]*SearcherMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[common.Address]*SearcherMetrics, len(m.searcherMetrics))
	for k, v := range m.searcherMetrics {
		result[k] = v
	}
	return result
}

func (m *MetricsStore) GetGlobalMetrics() *GlobalMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &GlobalMetrics{
		TotalBlocks:      m.globalStats.TotalBlocks,
		TotalBundles:     m.globalStats.TotalBundles,
		TotalMevProfit:   new(big.Int).Set(m.globalStats.TotalMevProfit),
		TotalRefunded:    new(big.Int).Set(m.globalStats.TotalRefunded),
		UniqueValidators: m.globalStats.UniqueValidators,
		UniqueSearchers:  m.globalStats.UniqueSearchers,
		StartTime:        m.globalStats.StartTime,
		UpdatedAt:        m.globalStats.UpdatedAt,
	}
}

func (m *MetricsStore) GetRecentBlocks(n int) []*BlockMevMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n <= 0 || len(m.blockMetrics) == 0 {
		return nil
	}

	if len(m.blockMetrics) < n {
		n = len(m.blockMetrics)
	}

	result := make([]*BlockMevMetrics, 0, n)
	for i := 0; i < n; i++ {
		if block, exists := m.blockMetrics[m.currentBlock-uint64(i)]; exists {
			result = append(result, block)
		}
	}

	return result
}

func (m *MetricsStore) CleanupOldBlocks(keepBlocks uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentBlock <= keepBlocks {
		return
	}

	cutoff := m.currentBlock - keepBlocks
	for blockNum := range m.blockMetrics {
		if blockNum < cutoff {
			delete(m.blockMetrics, blockNum)
		}
	}
}

func weightedAverage(current *big.Int, newVal *big.Int, currentWeight, newWeight int64) *big.Int {
	if currentWeight == 0 {
		return new(big.Int).Set(newVal)
	}

	totalWeight := currentWeight + newWeight
	if totalWeight == 0 {
		return big.NewInt(0)
	}

	currentPart := new(big.Int).Mul(current, big.NewInt(currentWeight))
	newPart := new(big.Int).Mul(newVal, big.NewInt(newWeight))
	sum := new(big.Int).Add(currentPart, newPart)

	return new(big.Int).Div(sum, big.NewInt(totalWeight))
}
