package metrics

import (
	"math/big"
	"rebate/pkg/types"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ============== 全局指标存储 ==============

// MetricsStore 指标存储
type MetricsStore struct {
	mu sync.RWMutex

	// 区块指标 (blockNumber -> metrics)
	blockMetrics map[uint64]*BlockMevMetrics

	// Validator 指标 (address -> metrics)
	validatorMetrics map[common.Address]*ValidatorMetrics

	// 搜索者指标 (address -> metrics)
	searcherMetrics map[common.Address]*SearcherMetrics

	// 全局统计
	globalStats *GlobalMetrics

	// 当前区块跟踪
	currentBlock uint64
}

// NewMetricsStore 创建指标存储
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

// StartNewBlock 开始新区块的指标收集
func (m *MetricsStore) StartNewBlock(blockNumber uint64, validator common.Address) *BlockMevMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := NewBlockMevMetrics(blockNumber, validator)
	m.blockMetrics[blockNumber] = metrics
	m.currentBlock = blockNumber

	// 初始化或更新 validator 指标
	if _, exists := m.validatorMetrics[validator]; !exists {
		m.validatorMetrics[validator] = NewValidatorMetrics(validator, blockNumber)
		m.globalStats.UniqueValidators++
	}

	return metrics
}

// FinalizeBlock 结束区块指标收集
func (m *MetricsStore) FinalizeBlock(blockNumber uint64, blockGasLimit uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.blockMetrics[blockNumber]
	if !exists {
		return
	}

	metrics.Finalize(blockGasLimit)

	// 更新 validator 指标
	if validator, ok := m.validatorMetrics[metrics.ValidatorAddress]; ok {
		validator.UpdateWithBlock(metrics)
	}

	// 更新全局统计
	m.globalStats.TotalBlocks++
	m.globalStats.TotalBundles += uint64(metrics.BundleCount)
	m.globalStats.TotalMevProfit.Add(m.globalStats.TotalMevProfit, metrics.TotalMevProfit)
	m.globalStats.TotalRefunded.Add(m.globalStats.TotalRefunded, metrics.TotalRefundable)
	m.globalStats.UpdatedAt = time.Now()
}

// RecordBundleResult 记录 bundle 执行结果
func (m *MetricsStore) RecordBundleResult(blockNumber uint64, result *types.SimMevBundleResponse, builder string, searcher common.Address) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新区块指标
	if block, exists := m.blockMetrics[blockNumber]; exists {
		block.AddBundleResult(result, builder)
	}

	// 更新搜索者指标
	m.updateSearcherMetrics(searcher, result, blockNumber)
}

// updateSearcherMetrics 更新搜索者指标
func (m *MetricsStore) updateSearcherMetrics(searcher common.Address, result *types.SimMevBundleResponse, blockNumber uint64) {
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

		// 更新平均利润
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

// GetBlockMetrics 获取区块指标
func (m *MetricsStore) GetBlockMetrics(blockNumber uint64) (*BlockMevMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.blockMetrics[blockNumber]
	return metrics, exists
}

// GetValidatorMetrics 获取 Validator 指标
func (m *MetricsStore) GetValidatorMetrics(address common.Address) (*ValidatorMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.validatorMetrics[address]
	return metrics, exists
}

// GetAllValidatorMetrics 获取所有 Validator 指标
func (m *MetricsStore) GetAllValidatorMetrics() map[common.Address]*ValidatorMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	result := make(map[common.Address]*ValidatorMetrics, len(m.validatorMetrics))
	for k, v := range m.validatorMetrics {
		result[k] = v
	}
	return result
}

// GetSearcherMetrics 获取搜索者指标
func (m *MetricsStore) GetSearcherMetrics(address common.Address) (*SearcherMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.searcherMetrics[address]
	return metrics, exists
}

// GetAllSearcherMetrics 获取所有搜索者指标
func (m *MetricsStore) GetAllSearcherMetrics() map[common.Address]*SearcherMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[common.Address]*SearcherMetrics, len(m.searcherMetrics))
	for k, v := range m.searcherMetrics {
		result[k] = v
	}
	return result
}

// GetGlobalMetrics 获取全局统计
func (m *MetricsStore) GetGlobalMetrics() *GlobalMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
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

// GetRecentBlocks 获取最近 N 个区块的指标
func (m *MetricsStore) GetRecentBlocks(n int) []*BlockMevMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n <= 0 || len(m.blockMetrics) == 0 {
		return nil
	}

	// 找到最新的 N 个区块
	var blocks []*BlockMevMetrics
	for _, metrics := range m.blockMetrics {
		blocks = append(blocks, metrics)
	}

	// 按区块号排序并取最新的 N 个
	if len(blocks) < n {
		n = len(blocks)
	}

	result := make([]*BlockMevMetrics, 0, n)
	for i := 0; i < n; i++ {
		if block, exists := m.blockMetrics[m.currentBlock-uint64(i)]; exists {
			result = append(result, block)
		}
	}

	return result
}

// CleanupOldBlocks 清理旧区块数据
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

// weightedAverage 计算加权平均
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
