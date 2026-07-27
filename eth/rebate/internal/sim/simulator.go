package sim

import (
	"context"
	"crypto/ecdsa"
	"rebate/internal/experiment"
	"sync"
	"time"

	"rebate/internal/builder"
	"rebate/internal/hints"
	"rebate/internal/metrics"
	"rebate/internal/queue"
	"rebate/mylog"
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
	dispatcher    *builder.Dispatcher
	recorder      *experiment.Recorder
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
	dispatcher *builder.Dispatcher,
	recorders ...*experiment.Recorder,
) *SimulationWorker {
	var recorder *experiment.Recorder
	if len(recorders) > 0 {
		recorder = recorders[0]
	}
	return &SimulationWorker{
		simulator:     simulator,
		queue:         queue,
		store:         store,
		hintBroadcast: hintBroadcast,
		signer:        signer,
		metrics:       metrics,
		dispatcher:    dispatcher,
		recorder:      recorder,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动工作器
func (w *SimulationWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
	mylog.Logger.Info().Msg("Simulation worker started")
}

// Stop 停止工作器
func (w *SimulationWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	mylog.Logger.Info().Msg("Simulation worker stopped")
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
				mylog.Logger.Error().
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

	mylog.Logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("targetBlock", item.TargetBlock).
		Int("retry", item.Retries).
		Msg("Processing bundle")

	// 1. 检查是否已取消
	if w.store.IsCancelled(bundle.Metadata.BundleHash) {
		mylog.Logger.Info().
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
		mylog.Logger.Warn().
			Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
			Str("error", result.Error).
			Str("execError", result.ExecError).
			Msg("Bundle simulation failed")

		// 存储失败结果
		w.store.StoreSimResult(bundle.Metadata.BundleHash, result)
		w.recordBundleSimulation(bundle, result, searcher)
		return nil
	}

	// 6. 计算并设置 MatchingHash (用于 Hint)
	bundle.Metadata.MatchingHash = utils.CalculateMatchingHash(bundle.Metadata.BundleHash, w.signer)
	w.store.IndexMatchingHash(bundle.Metadata.MatchingHash, bundle.Metadata.BundleHash)

	// 7. 提取并广播 Hints
	if bundle.Privacy != nil && bundle.Privacy.Hints != types.HintNone {
		hint := hints.ExtractHints(bundle, result)
		if hint != nil {
			if err := w.hintBroadcast.Broadcast(hint); err != nil {
				mylog.Logger.Error().Err(err).Msg("Failed to broadcast hint")
			}
		}
	}

	// 8. 存储模拟结果
	w.store.StoreSimResult(bundle.Metadata.BundleHash, result)
	w.recordBundleSimulation(bundle, result, searcher)

	// 9. 发送给 Builder (简化版: 只记录日志)
	w.sendToBuilders(bundle, result)

	mylog.Logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("gasUsed", uint64(result.GasUsed)).
		Str("profit", result.Profit.ToInt().String()).
		Msg("Bundle processed successfully")

	return nil
}

// sendToBuilders 通过 Dispatcher 将 bundle 分发给 builder
func (w *SimulationWorker) sendToBuilders(bundle *types.SendMevBundleArgs, result *types.SimMevBundleResponse) {
	if w.dispatcher == nil {
		return
	}
	ctx := context.Background()
	if err := w.dispatcher.Dispatch(ctx, bundle, result); err != nil {
		mylog.Logger.Error().
			Err(err).
			Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
			Msg("Dispatcher failed to send bundle")
	}
}

func (w *SimulationWorker) recordBundleSimulation(bundle *types.SendMevBundleArgs, result *types.SimMevBundleResponse, searcher common.Address) {
	if w.recorder == nil || bundle == nil || bundle.Metadata == nil || result == nil {
		return
	}

	txCount, nestedCount, hashRefCount := summarizeBundleBody(bundle.Body)
	var requestedBuilders []string
	var wantRefund *int
	if bundle.Privacy != nil {
		if len(bundle.Privacy.Builders) > 0 {
			requestedBuilders = append(requestedBuilders, bundle.Privacy.Builders...)
		}
		wantRefund = bundle.Privacy.WantRefund
	}

	event := experiment.BundleSimulationEvent{
		RecordedAt:         time.Now().UTC(),
		BundleHash:         bundle.Metadata.BundleHash.Hex(),
		TargetBlock:        uint64(bundle.Inclusion.BlockNumber),
		MaxBlock:           uint64(bundle.Inclusion.MaxBlock),
		RequestedBuilders:  requestedBuilders,
		WantRefundPercent:  wantRefund,
		BodyItemCount:      len(bundle.Body),
		TxCount:            txCount,
		NestedBundleCount:  nestedCount,
		HashReferenceCount: hashRefCount,
		IsBackrun:          hashRefCount > 0,
		SimulationSuccess:  result.Success,
		SimulationError:    result.Error,
		ExecutionError:     result.ExecError,
		GasUsed:            uint64(result.GasUsed),
		ProfitWei:          result.Profit.ToInt().String(),
		RefundableWei:      result.RefundableValue.ToInt().String(),
		MevGasPriceWei:     result.MevGasPrice.ToInt().String(),
	}
	if searcher != (common.Address{}) {
		event.Searcher = searcher.Hex()
	}
	if bundle.Metadata.MatchingHash != (common.Hash{}) {
		event.MatchingHash = bundle.Metadata.MatchingHash.Hex()
	}
	if result.Block != nil {
		event.SimulatedBlockNumber = uint64(result.Block.BlockNumber)
		event.HistoricalTxCount = uint64(result.Block.HistoricalTxCount)
		event.BundleInsertionIndex = uint64(result.Block.BundleInsertionIndex)
		event.DisplacedTxCount = len(result.Block.DisplacedTxs)
	}

	if err := w.recorder.RecordBundleSimulation(event); err != nil {
		mylog.Logger.Warn().Err(err).Msg("Failed to record bundle simulation event")
	}
}

func summarizeBundleBody(body []types.MevBundleBody) (txCount int, nestedCount int, hashRefCount int) {
	for _, item := range body {
		if item.Tx != nil {
			txCount++
		}
		if item.Hash != nil {
			hashRefCount++
		}
		if item.Bundle != nil {
			nestedCount++
			subTxCount, subNestedCount, subHashRefCount := summarizeBundleBody(item.Bundle.Body)
			txCount += subTxCount
			nestedCount += subNestedCount
			hashRefCount += subHashRefCount
		}
	}
	return txCount, nestedCount, hashRefCount
}
