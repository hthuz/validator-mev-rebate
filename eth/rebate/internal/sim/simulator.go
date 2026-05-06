package sim

import (
	"context"
	"crypto/ecdsa"
	"sync"

	"rebate/internal/hints"
	"rebate/internal/metrics"
	"rebate/internal/queue"
	"rebate/log"
	"rebate/pkg/types"
	"rebate/pkg/utils"

	"github.com/ethereum/go-ethereum/common"
)

// ============== 模拟工作器 ==============

// SimulationWorker 模拟工作器
type SimulationWorker struct {
	simulator     SimulationBackend
	queue         *queue.SimulationQueue
	store         *BundleStore
	hintBroadcast hints.HintBroadcaster
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
	hintBroadcast hints.HintBroadcaster,
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
		if sender, err := utils.GetTransactionSender(*bundle.Body[0].Tx); err == nil {
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
	bundle.Metadata.MatchingHash = utils.CalculateMatchingHash(bundle.Metadata.BundleHash, w.signer)

	// 7. 提取并广播 Hints
	if bundle.Privacy != nil && bundle.Privacy.Hints != types.HintNone {
		hint := hints.ExtractHints(bundle, result)
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
