package sim

import (
	"context"
	"math/big"
	"rebate/mylog"
	"rebate/pkg/types"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

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

	mylog.Logger.Debug().
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
