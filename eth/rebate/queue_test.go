package main

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestQueueBasicOperations 测试队列基本操作
func TestQueueBasicOperations(t *testing.T) {
	queue := NewSimulationQueue()
	defer queue.Close()

	// 创建测试 bundle
	bundle := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(100),
			MaxBlock:    hexutil.Uint64(110),
		},
		Metadata: &MevBundleMetadata{
			BundleHash: common.HexToHash("0x1234"),
		},
	}

	// 测试入队
	queue.Push(bundle, false)
	if queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", queue.Len())
	}

	// 测试块更新
	queue.UpdateBlock(99)

	// 测试出队
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	item, ok := queue.Pop(ctx)
	if !ok {
		t.Error("expected to pop an item")
	}
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Bundle.Metadata.BundleHash != bundle.Metadata.BundleHash {
		t.Error("bundle hash mismatch")
	}

	// 队列应该为空
	if queue.Len() != 0 {
		t.Errorf("expected empty queue, got %d items", queue.Len())
	}
}

// TestQueuePriority 测试优先级
func TestQueuePriority(t *testing.T) {
	queue := NewSimulationQueue()
	defer queue.Close()
	queue.UpdateBlock(0)

	// 添加普通优先级 bundle
	bundle1 := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(1),
			MaxBlock:    hexutil.Uint64(10),
		},
		Metadata: &MevBundleMetadata{
			BundleHash: common.HexToHash("0x1111"),
		},
	}
	queue.Push(bundle1, false)

	// 添加高优先级 bundle
	bundle2 := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(1),
			MaxBlock:    hexutil.Uint64(10),
		},
		Metadata: &MevBundleMetadata{
			BundleHash: common.HexToHash("0x2222"),
		},
	}
	queue.Push(bundle2, true)

	// 高优先级应该先出队
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	item, _ := queue.Pop(ctx)
	if item.Bundle.Metadata.BundleHash.Hex() != bundle2.Metadata.BundleHash.Hex() {
		t.Error("priority bundle should be popped first")
	}
}

// TestQueueExpiredCleanup 测试过期清理
func TestQueueExpiredCleanup(t *testing.T) {
	queue := NewSimulationQueue()
	defer queue.Close()

	// 添加 bundle，maxBlock = 100
	bundle := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(90),
			MaxBlock:    hexutil.Uint64(100),
		},
		Metadata: &MevBundleMetadata{
			BundleHash: common.HexToHash("0x3333"),
		},
	}
	queue.Push(bundle, false)

	// 更新块到 101，bundle 应该过期被清理
	queue.UpdateBlock(101)

	if queue.Len() != 0 {
		t.Errorf("expected expired bundle to be cleaned, got %d items", queue.Len())
	}
}

// TestQueueRequeue 测试重新入队
func TestQueueRequeue(t *testing.T) {
	queue := NewSimulationQueue()
	defer queue.Close()
	queue.UpdateBlock(0)

	bundle := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(1),
			MaxBlock:    hexutil.Uint64(10),
		},
		Metadata: &MevBundleMetadata{
			BundleHash: common.HexToHash("0x4444"),
		},
	}

	item := &BundleQueueItem{
		Bundle:      bundle,
		TargetBlock: 1,
		Retries:     0,
	}

	queue.Requeue(item)

	if queue.Len() != 1 {
		t.Errorf("expected 1 item after requeue, got %d", queue.Len())
	}

	if item.Retries != 1 {
		t.Errorf("expected retries to be 1, got %d", item.Retries)
	}
}

// TestQueueClose 测试队列关闭
func TestQueueClose(t *testing.T) {
	queue := NewSimulationQueue()
	queue.UpdateBlock(0)

	bundle := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(1),
			MaxBlock:    hexutil.Uint64(10),
		},
		Metadata: &MevBundleMetadata{
			BundleHash: common.HexToHash("0x5555"),
		},
	}
	queue.Push(bundle, false)

	// 关闭队列
	queue.Close()

	// Pop 应该返回 false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 给一点时间让关闭信号传播
	time.Sleep(10 * time.Millisecond)

	_, ok := queue.Pop(ctx)
	if ok {
		t.Error("expected Pop to return false after Close")
	}
}

