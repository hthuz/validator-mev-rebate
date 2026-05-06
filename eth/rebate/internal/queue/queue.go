package queue

import (
	"context"
	"rebate/log"
	"rebate/pkg/types"
	"sync"
	"time"
)

// ============== 队列相关类型 ==============

// BundleQueueItem 队列中的 Bundle 项
type BundleQueueItem struct {
	Bundle      *types.SendMevBundleArgs
	TargetBlock uint64
	Priority    bool // 高优先级
	AddedAt     time.Time
	Retries     int
}

// SimulationQueue 简单的内存队列
type SimulationQueue struct {
	mu           sync.Mutex
	items        []*BundleQueueItem
	cond         *sync.Cond
	closed       bool
	currentBlock uint64
}

// NewSimulationQueue 创建新的模拟队列
func NewSimulationQueue() *SimulationQueue {
	q := &SimulationQueue{
		items: make([]*BundleQueueItem, 0),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push 添加 Bundle 到队列
func (q *SimulationQueue) Push(bundle *types.SendMevBundleArgs, priority bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	item := &BundleQueueItem{
		Bundle:      bundle,
		TargetBlock: uint64(bundle.Inclusion.BlockNumber),
		Priority:    priority,
		AddedAt:     time.Now(),
	}

	// 高优先级插入到前面
	if priority {
		q.items = append([]*BundleQueueItem{item}, q.items...)
	} else {
		q.items = append(q.items, item)
	}

	// 唤醒等待的 worker
	q.cond.Signal()

	log.Logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("targetBlock", item.TargetBlock).
		Bool("priority", priority).
		Int("queueSize", len(q.items)).
		Msg("Bundle added to queue")
}

// Pop 从队列取出下一个可处理的 Bundle
func (q *SimulationQueue) Pop(ctx context.Context) (*BundleQueueItem, bool) {
	// 监听 context 取消
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.mu.Lock()
			q.cond.Broadcast()
			q.mu.Unlock()
		case <-done:
		}
	}()
	defer close(done)

	q.mu.Lock()
	defer q.mu.Unlock()

	for {
		// 检查 context 是否取消
		select {
		case <-ctx.Done():
			return nil, false
		default:
		}

		if q.closed {
			return nil, false
		}

		// 查找可处理的项 (targetBlock <= currentBlock)
		for i, item := range q.items {
			if item.TargetBlock <= q.currentBlock+1 {
				// 移除并返回
				q.items = append(q.items[:i], q.items[i+1:]...)
				return item, true
			}
		}

		// 没有可处理的项, 等待
		q.cond.Wait()
	}
}

// UpdateBlock 更新当前块号
func (q *SimulationQueue) UpdateBlock(block uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if block > q.currentBlock {
		q.currentBlock = block
		// 唤醒等待的 worker, 可能有新的 Bundle 可以处理了
		q.cond.Broadcast()

		// 清理过期的 Bundle
		q.cleanExpired()
	}
}

// cleanExpired 清理过期的 Bundle (内部调用, 需要持有锁)
func (q *SimulationQueue) cleanExpired() {
	newItems := make([]*BundleQueueItem, 0, len(q.items))
	for _, item := range q.items {
		maxBlock := uint64(item.Bundle.Inclusion.MaxBlock)
		if maxBlock >= q.currentBlock {
			newItems = append(newItems, item)
		} else {
			log.Logger.Debug().
				Str("bundleHash", item.Bundle.Metadata.BundleHash.Hex()).
				Uint64("maxBlock", maxBlock).
				Uint64("currentBlock", q.currentBlock).
				Msg("Expired bundle removed from queue")
		}
	}
	q.items = newItems
}

// Len 返回队列长度
func (q *SimulationQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Close 关闭队列
func (q *SimulationQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Requeue 重新入队 (模拟失败后重试)
func (q *SimulationQueue) Requeue(item *BundleQueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	item.Retries++
	if item.Retries > 5 {
		log.Logger.Warn().
			Str("bundleHash", item.Bundle.Metadata.BundleHash.Hex()).
			Int("retries", item.Retries).
			Msg("Bundle exceeded max retries, dropping")
		return
	}

	q.items = append(q.items, item)
	q.cond.Signal()
}
