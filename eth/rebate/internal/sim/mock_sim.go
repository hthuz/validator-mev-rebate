package sim

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"rebate/internal/logging"
	"rebate/pkg/types"
	"rebate/pkg/utils"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// ============== Mock 模拟器 (用于 demo) ==============

// MockSimulator Mock 模拟器
type MockSimulator struct {
	currentBlock uint64
	mu           sync.RWMutex
	random       *rand.Rand
	blockTxCount map[uint64]uint64
}

const mockBlockGasLimit = 30000000

// NewMockSimulator 创建 Mock 模拟器
func NewMockSimulator() *MockSimulator {
	return &MockSimulator{
		currentBlock: 1000000, // 初始块号
		random:       rand.New(rand.NewSource(time.Now().UnixNano())),
		blockTxCount: make(map[uint64]uint64),
	}
}

// SimulateBundle 模拟 Bundle 执行
func (m *MockSimulator) SimulateBundle(ctx context.Context, bundle *types.SendMevBundleArgs, overrides map[string]interface{}) (*types.SimMevBundleResponse, error) {
	m.mu.RLock()
	currentBlock := m.currentBlock
	m.mu.RUnlock()

	// 模拟一些处理延迟
	time.Sleep(50 * time.Millisecond)

	historicalTxCount, insertionIndex, baseFee := m.blockContext(currentBlock)
	gasUsed := uint64(21000 * len(bundle.Body))
	profit := new(big.Int).Mul(big.NewInt(int64(gasUsed)), baseFee)
	profit.Div(profit, big.NewInt(100))
	mevGasPrice := big.NewInt(1000000000) // 1 Gwei

	// 生成模拟日志
	bodyLogs := m.generateMockLogs(bundle.Body)
	blockHash := common.BytesToHash(crypto.Keccak256([]byte(fmt.Sprintf("mock-block-%d", currentBlock))))

	response := &types.SimMevBundleResponse{
		Success:         true,
		StateBlock:      hexutil.Uint64(currentBlock),
		MevGasPrice:     hexutil.Big(*mevGasPrice),
		Profit:          hexutil.Big(*profit),
		RefundableValue: hexutil.Big(*big.NewInt(profit.Int64() / 10)),
		GasUsed:         hexutil.Uint64(gasUsed),
		BodyLogs:        bodyLogs,
		Block: &types.SimulatedBlockContext{
			BlockNumber:          hexutil.Uint64(currentBlock),
			BlockHash:            blockHash,
			BlockTimestamp:       hexutil.Uint64(time.Now().Unix()),
			BaseFee:              hexutil.Big(*baseFee),
			HistoricalTxCount:    hexutil.Uint64(historicalTxCount),
			BundleInsertionIndex: hexutil.Uint64(insertionIndex),
		},
	}

	logging.Logger.Debug().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Uint64("stateBlock", currentBlock).
		Uint64("gasUsed", gasUsed).
		Msg("Bundle simulated")

	return response, nil
}

func (m *MockSimulator) blockContext(block uint64) (uint64, uint64, *big.Int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count, ok := m.blockTxCount[block]
	if !ok {
		if m.random.Intn(20) == 0 {
			count = uint64(1000 + m.random.Intn(4001))
		} else {
			count = uint64(100 + m.random.Intn(401))
		}
		m.blockTxCount[block] = count
	}

	insertionIndex := uint64(0)
	if count > 0 {
		insertionIndex = uint64(m.random.Intn(int(count)))
	}
	baseFee := big.NewInt(int64(10_000_000_000 + m.random.Intn(90_000_000_001)))
	return count, insertionIndex, baseFee
}

// generateMockLogs 生成模拟日志
func (m *MockSimulator) generateMockLogs(body []types.MevBundleBody) []types.SimMevBodyLogs {
	var bodyLogs []types.SimMevBodyLogs

	for _, elem := range body {
		logs := types.SimMevBodyLogs{}

		if elem.Tx != nil {
			// 解析交易获取目标地址
			tx, err := utils.DecodeTransaction(*elem.Tx)
			if err == nil && tx.To() != nil {
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

func (m *MockSimulator) CurrentBlock() uint64 {
	return m.GetBlock()
}

func (m *MockSimulator) AdvanceBlock() (uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentBlock++
	return m.currentBlock, true
}

func (m *MockSimulator) BlockGasLimit() uint64 {
	return mockBlockGasLimit
}
