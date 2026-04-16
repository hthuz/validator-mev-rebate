package main

import (
	"context"
	"sync"
	"time"
)

// ============== 模拟后端接口 ==============

// SimulationBackend 模拟后端接口
type SimulationBackend interface {
	SimulateBundle(ctx context.Context, bundle *SendMevBundleArgs, overrides map[string]interface{}) (*SimMevBundleResponse, error)
}

// ============== Mock 模拟器 (用于 demo) ==============

// MockSimulator Mock 模拟器
type MockSimulator struct {
	currentSlot uint64
	mu          sync.RWMutex
}

// NewMockSimulator 创建 Mock 模拟器
func NewMockSimulator() *MockSimulator {
	return &MockSimulator{
		currentSlot: 1000000, // 初始 slot
	}
}

// SimulateBundle 模拟 Bundle 执行
func (m *MockSimulator) SimulateBundle(ctx context.Context, bundle *SendMevBundleArgs, overrides map[string]interface{}) (*SimMevBundleResponse, error) {
	m.mu.RLock()
	currentSlot := m.currentSlot
	m.mu.RUnlock()

	// 模拟一些处理延迟
	time.Sleep(10 * time.Millisecond)

	// 计算模拟结果 (Solana 使用 compute units 而不是 gas)
	computeUnitsUsed := uint64(200000 * len(bundle.Body)) // 基础 CU
	profit := int64(10000 * len(bundle.Body))             // 模拟利润 (lamports)
	priorityFee := uint64(10000)                          // 0.00001 SOL

	// 生成模拟日志
	bodyLogs := m.generateMockLogs(bundle.Body)

	response := &SimMevBundleResponse{
		Success:          true,
		StateSlot:        currentSlot,
		PriorityFee:      priorityFee,
		Profit:           profit,
		RefundableValue:  profit / 10, // 10% 退款
		ComputeUnitsUsed: computeUnitsUsed,
		BodyLogs:         bodyLogs,
	}

	logger.Debug().
		Str("bundleHash", bundle.Metadata.BundleHash).
		Uint64("stateSlot", currentSlot).
		Uint64("computeUnits", computeUnitsUsed).
		Msg("Bundle simulated")

	return response, nil
}

// generateMockLogs 生成模拟日志 (Solana 风格)
func (m *MockSimulator) generateMockLogs(body []MevBundleBody) []SimMevBodyLogs {
	var bodyLogs []SimMevBodyLogs

	for _, elem := range body {
		logs := SimMevBodyLogs{}

		if elem.Tx != "" {
			// 模拟 Solana DEX 日志 (Raydium 或 Serum)
			logs.TxLogs = []SimLog{
				{
					Program: "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8", // Raydium AMM
					Events: []string{
						"Swap",           // 事件类型
						"TokenTransfer",  // 转账事件
					},
					Data: "base64_encoded_log_data", // 模拟数据
				},
				{
					Program: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", // SPL Token
					Events:  []string{"Transfer"},
					Data:    "amount=1000000",
				},
			}
		}

		bodyLogs = append(bodyLogs, logs)
	}

	return bodyLogs
}

// SetSlot 设置当前 slot
func (m *MockSimulator) SetSlot(slot uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentSlot = slot
}

// GetSlot 获取当前 slot
func (m *MockSimulator) GetSlot() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentSlot
}

// ============== 模拟工作器 ==============

// SimulationWorker 模拟工作器
type SimulationWorker struct {
	simulator     SimulationBackend
	queue         *SimulationQueue
	store         *BundleStore
	hintBroadcast HintBroadcaster
	wg            sync.WaitGroup
	stopCh        chan struct{}
}

// NewSimulationWorker 创建模拟工作器
func NewSimulationWorker(
	simulator SimulationBackend,
	queue *SimulationQueue,
	store *BundleStore,
	hintBroadcast HintBroadcaster,
) *SimulationWorker {
	return &SimulationWorker{
		simulator:     simulator,
		queue:         queue,
		store:         store,
		hintBroadcast: hintBroadcast,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动工作器
func (w *SimulationWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
	logger.Info().Msg("Simulation worker started")
}

// Stop 停止工作器
func (w *SimulationWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	logger.Info().Msg("Simulation worker stopped")
}

// run 工作器主循环
func (w *SimulationWorker) run(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			// 从队列获取 Bundle
			item, ok := w.queue.Pop(ctx)
			if !ok {
				return
			}

			// 处理 Bundle
			if err := w.process(ctx, item); err != nil {
				logger.Error().
					Err(err).
					Str("bundleHash", item.Bundle.Metadata.BundleHash).
					Msg("Failed to process bundle")

				// 重新入队
				w.queue.Requeue(item)
			}
		}
	}
}

// process 处理单个 Bundle
func (w *SimulationWorker) process(ctx context.Context, item *BundleQueueItem) error {
	bundle := item.Bundle

	logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash).
		Uint64("targetSlot", item.TargetSlot).
		Int("retry", item.Retries).
		Msg("Processing bundle")

	// 1. 检查是否已取消
	if w.store.IsCancelled(bundle.Metadata.BundleHash) {
		logger.Info().
			Str("bundleHash", bundle.Metadata.BundleHash).
			Msg("Bundle was cancelled, skipping")
		return nil
	}

	// 2. 调用模拟后端
	result, err := w.simulator.SimulateBundle(ctx, bundle, nil)
	if err != nil {
		return err
	}

	// 3. 获取 builder 信息
	builder := "jito"
	if bundle.Privacy != nil && len(bundle.Privacy.Builders) > 0 {
		builder = bundle.Privacy.Builders[0]
	}

	// 4. 检查模拟结果
	if !result.Success {
		logger.Warn().
			Str("bundleHash", bundle.Metadata.BundleHash).
			Str("error", result.Error).
			Str("execError", result.ExecError).
			Msg("Bundle simulation failed")

		w.store.StoreSimResult(bundle.Metadata.BundleHash, result)
		return nil
	}

	// 5. 计算并设置 MatchingHash (用于 Hint)
	matchingHash := calculateMatchingHash(bundle.Metadata.BundleHash)
	bundle.Metadata.MatchingHash = matchingHash
	w.store.IndexMatchingHash(matchingHash, bundle.Metadata.BundleHash)

	// 6. 提取并广播 Hints
	if bundle.Privacy != nil && bundle.Privacy.Hints != HintNone {
		hint := ExtractHints(bundle, result)
		if hint != nil {
			if err := w.hintBroadcast.Broadcast(hint); err != nil {
				logger.Error().Err(err).Msg("Failed to broadcast hint")
			}
		}
	}

	// 7. 存储模拟结果
	w.store.StoreSimResult(bundle.Metadata.BundleHash, result)

	// 8. 发送给 Builder (简化版: 只记录日志)
	w.sendToBuilders(bundle, result, builder)

	logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash).
		Uint64("computeUnits", result.ComputeUnitsUsed).
		Int64("profit", result.Profit).
		Msg("Bundle processed successfully")

	return nil
}

// sendToBuilders 发送给 Builders (简化版)
func (w *SimulationWorker) sendToBuilders(bundle *SendMevBundleArgs, result *SimMevBundleResponse, builder string) {
	builders := []string{"jito"} // 默认 Builder (Solana 上 Jito 是主要的)
	if bundle.Privacy != nil && len(bundle.Privacy.Builders) > 0 {
		builders = bundle.Privacy.Builders
	}

	for _, b := range builders {
		logger.Info().
			Str("bundleHash", bundle.Metadata.BundleHash).
			Str("builder", b).
			Msg("Sending bundle to builder (simulated)")
	}
}

// ============== Bundle 存储 (内存版) ==============

// BundleStore Bundle 存储
type BundleStore struct {
	mu         sync.RWMutex
	bundles    map[string]*SendMevBundleArgs
	simResults map[string]*SimMevBundleResponse
	cancelled  map[string]bool
	matchIndex map[string]string // matchingHash -> bundleHash
}

// NewBundleStore 创建 Bundle 存储
func NewBundleStore() *BundleStore {
	return &BundleStore{
		bundles:    make(map[string]*SendMevBundleArgs),
		simResults: make(map[string]*SimMevBundleResponse),
		cancelled:  make(map[string]bool),
		matchIndex: make(map[string]string),
	}
}

// StoreBundle 存储 Bundle
func (s *BundleStore) StoreBundle(bundle *SendMevBundleArgs) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundles[bundle.Metadata.BundleHash] = bundle
}

// GetBundle 获取 Bundle
func (s *BundleStore) GetBundle(hash string) (*SendMevBundleArgs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bundles[hash]
	return b, ok
}

// GetBundleByMatchingHash 通过 MatchingHash 获取 Bundle (用于 backrunning)
func (s *BundleStore) GetBundleByMatchingHash(matchingHash string) (*SendMevBundleArgs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bundleHash, ok := s.matchIndex[matchingHash]
	if !ok {
		return nil, false
	}
	return s.bundles[bundleHash], true
}

// IndexMatchingHash 索引 MatchingHash
func (s *BundleStore) IndexMatchingHash(matchingHash, bundleHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchIndex[matchingHash] = bundleHash
}

// StoreSimResult 存储模拟结果
func (s *BundleStore) StoreSimResult(hash string, result *SimMevBundleResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.simResults[hash] = result
}

// GetSimResult 获取模拟结果
func (s *BundleStore) GetSimResult(hash string) (*SimMevBundleResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.simResults[hash]
	return r, ok
}

// Cancel 取消 Bundle
func (s *BundleStore) Cancel(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.bundles[hash]; exists {
		s.cancelled[hash] = true
		return true
	}
	return false
}

// IsCancelled 检查是否已取消
func (s *BundleStore) IsCancelled(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancelled[hash]
}
