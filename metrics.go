package main

import (
	"math/big"
	"sync"
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
func (b *BlockMevMetrics) AddBundleResult(result *SimMevBundleResponse, builder string) {
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
		Address:        address,
		FirstSeenBlock: firstBlock,
		LastSeenBlock:  firstBlock,
		UpdatedAt:      time.Now(),
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

// SearcherMetrics 搜索者指标
type SearcherMetrics struct {
	Address        common.Address `json:"address"`
	TotalBundles   uint64         `json:"totalBundles"`
	SuccessBundles uint64         `json:"successBundles"`
	FailedBundles  uint64         `json:"failedBundles"`
	SuccessRate    float64        `json:"successRate"`

	TotalProfit     *big.Int `json:"totalProfit"`
	AvgProfit       *big.Int `json:"avgProfit"`
	BackrunCount    uint64   `json:"backrunCount"`
	BackrunSuccess  uint64   `json:"backrunSuccess"`
	BackrunProfit   *big.Int `json:"backrunProfit"`

	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
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
func (m *MetricsStore) RecordBundleResult(blockNumber uint64, result *SimMevBundleResponse, builder string, searcher common.Address) {
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
func (m *MetricsStore) updateSearcherMetrics(searcher common.Address, result *SimMevBundleResponse, blockNumber uint64) {
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

// ============== 辅助函数 ==============

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
