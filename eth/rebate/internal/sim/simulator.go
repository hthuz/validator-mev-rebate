package sim

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"sync"
	"time"

	"rebate/internal"
	"rebate/internal/metrics"
	"rebate/internal/queue"
	"rebate/log"
	"rebate/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// ============== 模拟后端接口 ==============

// SimulationBackend 模拟后端接口
type SimulationBackend interface {
	SimulateBundle(ctx context.Context, bundle *types.SendMevBundleArgs, overrides map[string]interface{}) (*types.SimMevBundleResponse, error)
}

// ============== Mock 模拟器 (用于 demo) ==============

// MockSimulator Mock 模拟器
type MockSimulator struct {
	currentBlock uint64
	mu           sync.RWMutex
}

// NewMockSimulator 创建 Mock 模拟器
func NewMockSimulator() *MockSimulator {
	return &MockSimulator{
		currentBlock: 1000000, // 初始块号
	}
}

// SimulateBundle 模拟 Bundle 执行
func (m *MockSimulator) SimulateBundle(ctx context.Context, bundle *types.SendMevBundleArgs, overrides map[string]interface{}) (*types.SimMevBundleResponse, error) {
	m.mu.RLock()
	currentBlock := m.currentBlock
	m.mu.RUnlock()

	// 模拟一些处理延迟
	time.Sleep(50 * time.Millisecond)

	// 计算模拟结果
	gasUsed := uint64(21000 * len(bundle.Body)) // 基础 gas
	profit := big.NewInt(int64(gasUsed * 100))  // 模拟利润
	mevGasPrice := big.NewInt(1000000000)       // 1 Gwei

	// 生成模拟日志
	bodyLogs := m.generateMockLogs(bundle.Body)

	response := &types.SimMevBundleResponse{
		Success:         true,
		StateBlock:      hexutil.Uint64(currentBlock),
		MevGasPrice:     hexutil.Big(*mevGasPrice),
		Profit:          hexutil.Big(*profit),
		RefundableValue: hexutil.Big(*big.NewInt(profit.Int64() / 10)),
		GasUsed:         hexutil.Uint64(gasUsed),
		BodyLogs:        bodyLogs,
	}

	log.Logger.Debug().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("stateBlock", currentBlock).
		Uint64("gasUsed", gasUsed).
		Msg("Bundle simulated")

	return response, nil
}

// generateMockLogs 生成模拟日志
func (m *MockSimulator) generateMockLogs(body []types.MevBundleBody) []types.SimMevBodyLogs {
	var bodyLogs []types.SimMevBodyLogs

	for _, elem := range body {
		logs := types.SimMevBodyLogs{}

		if elem.Tx != nil {
			// 解析交易获取目标地址
			var tx etypes.Transaction
			if err := rlp.DecodeBytes(*elem.Tx, &tx); err == nil && tx.To() != nil {
				// 模拟一个 Uniswap V2 Swap 日志
				logs.TxLogs = []types.SimLog{
					{
						Address: *tx.To(),
						Topics: []common.Hash{
							// Uniswap V2 Swap 事件
							common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"),
							common.HexToHash("0x0000000000000000000000007a250d5630B4cF539739dF2C5dAcb4c659F2488D"),
							common.HexToHash("0x0000000000000000000000007a250d5630B4cF539739dF2C5dAcb4c659F2488D"),
						},
						Data: []byte{0x00, 0x01, 0x02, 0x03}, // 模拟数据
					},
				}
			}
		}

		bodyLogs = append(bodyLogs, logs)
	}

	return bodyLogs
}

// SetBlock 设置当前块号
func (m *MockSimulator) SetBlock(block uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentBlock = block
}

// GetBlock 获取当前块号
func (m *MockSimulator) GetBlock() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentBlock
}

// ============== 模拟工作器 ==============

// SimulationWorker 模拟工作器
type SimulationWorker struct {
	simulator     SimulationBackend
	queue         *queue.SimulationQueue
	store         *BundleStore
	hintBroadcast internal.HintBroadcaster
	signer        *ecdsa.PrivateKey
	metrics       *metrics.MetricsStore
	wg            sync.WaitGroup
	stopCh        chan struct{}
}

// NewSimulationWorker 创建模拟工作器
func NewSimulationWorker(
	simulator SimulationBackend,
	queue *queue.SimulationQueue,
	store *BundleStore,
	hintBroadcast internal.HintBroadcaster,
	signer *ecdsa.PrivateKey,
	metrics *metrics.MetricsStore,
) *SimulationWorker {
	return &SimulationWorker{
		simulator:     simulator,
		queue:         queue,
		store:         store,
		hintBroadcast: hintBroadcast,
		signer:        signer,
		metrics:       metrics,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动工作器
func (w *SimulationWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
	log.Logger.Info().Msg("Simulation worker started")
}

// Stop 停止工作器
func (w *SimulationWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	log.Logger.Info().Msg("Simulation worker stopped")
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
				log.Logger.Error().
					Err(err).
					Str("bundleHash", item.Bundle.Metadata.BundleHash.Hex()).
					Msg("Failed to process bundle")

				// 重新入队
				w.queue.Requeue(item)
			}
		}
	}
}

// process 处理单个 Bundle
func (w *SimulationWorker) process(ctx context.Context, item *queue.BundleQueueItem) error {
	bundle := item.Bundle

	log.Logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("targetBlock", item.TargetBlock).
		Int("retry", item.Retries).
		Msg("Processing bundle")

	// 1. 检查是否已取消
	if w.store.IsCancelled(bundle.Metadata.BundleHash) {
		log.Logger.Info().
			Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
			Msg("Bundle was cancelled, skipping")
		return nil
	}

	// 2. 调用模拟后端
	result, err := w.simulator.SimulateBundle(ctx, bundle, nil)
	if err != nil {
		return err
	}

	// 3. 获取 builder 和 searcher 信息
	builder := "flashbots"
	if bundle.Privacy != nil && len(bundle.Privacy.Builders) > 0 {
		builder = bundle.Privacy.Builders[0]
	}

	var searcher common.Address
	if len(bundle.Body) > 0 && bundle.Body[0].Tx != nil {
		if sender, err := internal.GetTransactionSender(*bundle.Body[0].Tx); err == nil {
			searcher = sender
		}
	}

	// 4. 记录指标 (在检查成功/失败之前，因为要统计两者)
	if w.metrics != nil {
		w.metrics.RecordBundleResult(item.TargetBlock, result, builder, searcher)
	}

	// 5. 检查模拟结果
	if !result.Success {
		log.Logger.Warn().
			Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
			Str("error", result.Error).
			Str("execError", result.ExecError).
			Msg("Bundle simulation failed")

		// 存储失败结果
		w.store.StoreSimResult(bundle.Metadata.BundleHash, result)
		return nil
	}

	// 6. 计算并设置 MatchingHash (用于 Hint)
	bundle.Metadata.MatchingHash = internal.CalculateMatchingHash(bundle.Metadata.BundleHash, w.signer)

	// 7. 提取并广播 Hints
	if bundle.Privacy != nil && bundle.Privacy.Hints != types.HintNone {
		hint := internal.ExtractHints(bundle, result)
		if hint != nil {
			if err := w.hintBroadcast.Broadcast(hint); err != nil {
				log.Logger.Error().Err(err).Msg("Failed to broadcast hint")
			}
		}
	}

	// 8. 存储模拟结果
	w.store.StoreSimResult(bundle.Metadata.BundleHash, result)

	// 9. 发送给 Builder (简化版: 只记录日志)
	w.sendToBuilders(bundle, result)

	log.Logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("gasUsed", uint64(result.GasUsed)).
		Str("profit", result.Profit.ToInt().String()).
		Msg("Bundle processed successfully")

	return nil
}

// sendToBuilders 发送给 Builders (简化版
func (w *SimulationWorker) sendToBuilders(bundle *types.SendMevBundleArgs, result *types.SimMevBundleResponse) {
	builders := []string{"flashbots"} // 默认 Builder
	if bundle.Privacy != nil && len(bundle.Privacy.Builders) > 0 {
		builders = bundle.Privacy.Builders
	}

	for _, builder := range builders {
		log.Logger.Info().
			Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
			Str("builder", builder).
			Msg("Sending bundle to builder (simulated)")
	}
}

// ============== Bundle 存储 (内存版) ==============

// BundleStore Bundle 存储
type BundleStore struct {
	mu         sync.RWMutex
	bundles    map[common.Hash]*types.SendMevBundleArgs
	simResults map[common.Hash]*types.SimMevBundleResponse
	cancelled  map[common.Hash]bool
	matchIndex map[common.Hash]common.Hash // matchingHash -> bundleHash
}

// NewBundleStore 创建 Bundle 存储
func NewBundleStore() *BundleStore {
	return &BundleStore{
		bundles:    make(map[common.Hash]*types.SendMevBundleArgs),
		simResults: make(map[common.Hash]*types.SimMevBundleResponse),
		cancelled:  make(map[common.Hash]bool),
		matchIndex: make(map[common.Hash]common.Hash),
	}
}

// StoreBundle 存储 Bundle
func (s *BundleStore) StoreBundle(bundle *types.SendMevBundleArgs) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundles[bundle.Metadata.BundleHash] = bundle
}

// GetBundle 获取 Bundle
func (s *BundleStore) GetBundle(hash common.Hash) (*types.SendMevBundleArgs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bundles[hash]
	return b, ok
}

// GetBundleByMatchingHash 通过 MatchingHash 获取 Bundle (用于 backrunning)
func (s *BundleStore) GetBundleByMatchingHash(matchingHash common.Hash) (*types.SendMevBundleArgs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bundleHash, ok := s.matchIndex[matchingHash]
	if !ok {
		return nil, false
	}
	return s.bundles[bundleHash], true
}

// IndexMatchingHash 索引 MatchingHash
func (s *BundleStore) IndexMatchingHash(matchingHash, bundleHash common.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchIndex[matchingHash] = bundleHash
}

// StoreSimResult 存储模拟结果
func (s *BundleStore) StoreSimResult(hash common.Hash, result *types.SimMevBundleResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.simResults[hash] = result
}

// GetSimResult 获取模拟结果
func (s *BundleStore) GetSimResult(hash common.Hash) (*types.SimMevBundleResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.simResults[hash]
	return r, ok
}

// Cancel 取消 Bundle
func (s *BundleStore) Cancel(hash common.Hash) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.bundles[hash]; exists {
		s.cancelled[hash] = true
		return true
	}
	return false
}

// IsCancelled 检查是否已取消
func (s *BundleStore) IsCancelled(hash common.Hash) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancelled[hash]
}

// ============== 签名者管理 ==============

// GenerateSigner 生成签名密钥 (用于 MatchingHash)
func GenerateSigner() (*ecdsa.PrivateKey, error) {
	return crypto.GenerateKey()
}
