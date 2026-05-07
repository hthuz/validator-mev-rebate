package hints

import (
	"math/big"

	"rebate/mylog"
	"rebate/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// ============== Hint 提取常量 ==============

const (
	HintGasPriceNumberPrecisionDigits = 3 // Gas Price 精度 (保留3位有效数字)
	HintGasNumberPrecisionDigits      = 2 // Gas 数量精度 (保留2位有效数字)
)

// 特殊日志签名 (DEX 相关事件)
var SpecialLogTopics = map[common.Hash]bool{
	// Uniswap V2 Swap
	common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"): true,
	// Uniswap V3 Swap
	common.HexToHash("0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"): true,
	// Curve Exchange
	common.HexToHash("0x8b3e96f2b889fa771c53c981b40daf005f63f637f1869f707052d15a3dd97140"): true,
	// Balancer Swap
	common.HexToHash("0x2170c741c41531aec20e7c107c24eecfdd15e69c9bb0a8dd37b1840b9e0b207b"): true,
}

// ============== Hint 提取函数 ==============

// ExtractHints 从 Bundle 和模拟结果中提取 Hints
func ExtractHints(bundle *types.SendMevBundleArgs, simResult *types.SimMevBundleResponse) *types.Hint {
	if bundle.Privacy == nil || bundle.Privacy.Hints == types.HintNone {
		return nil
	}

	hints := bundle.Privacy.Hints
	hint := &types.Hint{
		Hash: bundle.Metadata.MatchingHash,
	}

	// 提取交易 Hints
	hint.Txs = extractTxHints(bundle.Body, hints)

	// 提取日志 Hints (从模拟结果)
	if simResult != nil && (hints.Has(types.HintLogs) || hints.Has(types.HintSpecialLogs)) {
		hint.Logs = extractLogHints(simResult.BodyLogs, hints)
	}

	// 提取 Gas 信息 (降低精度保护隐私)
	if simResult != nil {
		if simResult.MevGasPrice.ToInt().Cmp(big.NewInt(0)) > 0 {
			reducedGasPrice := reduceIntPrecision(simResult.MevGasPrice.ToInt(), HintGasPriceNumberPrecisionDigits)
			hint.MevGasPrice = (*hexutil.Big)(reducedGasPrice)
		}
		if simResult.GasUsed > 0 {
			reducedGas := reduceIntPrecision(big.NewInt(int64(simResult.GasUsed)), HintGasNumberPrecisionDigits)
			gasUsed := hexutil.Uint64(reducedGas.Uint64())
			hint.GasUsed = &gasUsed
		}
	}

	return hint
}

// extractTxHints 提取交易相关的 Hints
func extractTxHints(body []types.MevBundleBody, hints types.HintIntent) []types.TxHint {
	var txHints []types.TxHint

	for _, elem := range body {
		if elem.Tx == nil {
			continue
		}

		var tx etypes.Transaction
		if err := rlp.DecodeBytes(*elem.Tx, &tx); err != nil {
			continue
		}

		txHint := types.TxHint{}

		// 交易哈希
		if hints.Has(types.HintTxHash) || hints.Has(types.HintHash) {
			hash := tx.Hash()
			txHint.Hash = &hash
		}

		// 合约地址
		if hints.Has(types.HintContractAddress) {
			to := tx.To()
			if to != nil {
				txHint.To = to
			}
		}

		// 函数选择器 (前4字节)
		if hints.Has(types.HintFunctionSelector) && len(tx.Data()) >= 4 {
			selector := hexutil.Bytes(tx.Data()[:4])
			txHint.FunctionSelector = &selector
		}

		// 完整调用数据
		if hints.Has(types.HintCallData) && len(tx.Data()) > 0 {
			callData := hexutil.Bytes(tx.Data())
			txHint.CallData = &callData
		}

		txHints = append(txHints, txHint)
	}

	return txHints
}

// extractLogHints 提取日志相关的 Hints
func extractLogHints(bodyLogs []types.SimMevBodyLogs, hints types.HintIntent) []types.CleanLog {
	var cleanLogs []types.CleanLog

	for _, logs := range bodyLogs {
		for _, log := range logs.TxLogs {
			// 如果只要求 SpecialLogs, 过滤非特殊日志
			if hints.Has(types.HintSpecialLogs) && !hints.Has(types.HintLogs) {
				if !isSpecialLog(log) {
					continue
				}
				// 对于特殊日志, 只保留签名和池地址
				cleanLogs = append(cleanLogs, cleanSpecialLog(log))
			} else if hints.Has(types.HintLogs) {
				// 完整日志
				cleanLogs = append(cleanLogs, types.CleanLog{
					Address: log.Address,
					Topics:  log.Topics,
					Data:    log.Data,
				})
			}
		}

		// 递归处理嵌套 Bundle 的日志
		if len(logs.BundleLogs) > 0 {
			nestedLogs := extractLogHints(logs.BundleLogs, hints)
			cleanLogs = append(cleanLogs, nestedLogs...)
		}
	}

	return cleanLogs
}

// isSpecialLog 检查是否为特殊日志 (DEX 事件)
func isSpecialLog(log types.SimLog) bool {
	if len(log.Topics) == 0 {
		return false
	}
	return SpecialLogTopics[log.Topics[0]]
}

// cleanSpecialLog 清理特殊日志 (只保留签名和池地址)
func cleanSpecialLog(log types.SimLog) types.CleanLog {
	clean := types.CleanLog{
		Address: log.Address,
	}

	// 只保留事件签名 (第一个 topic)
	if len(log.Topics) > 0 {
		clean.Topics = []common.Hash{log.Topics[0]}
	}

	// 对于 Balancer, 还保留 Pool ID (第二个 topic)
	balancerSwap := common.HexToHash("0x2170c741c41531aec20e7c107c24eecfdd15e69c9bb0a8dd37b1840b9e0b207b")
	if len(log.Topics) > 1 && log.Topics[0] == balancerSwap {
		clean.Topics = append(clean.Topics, log.Topics[1])
	}

	// 不包含 Data (隐藏交易数量)
	return clean
}

// reduceIntPrecision 降低数值精度 (保护隐私)
// 例如: 123456 保留3位 -> 123000
func reduceIntPrecision(n *big.Int, precision int) *big.Int {
	if n == nil || n.Sign() <= 0 {
		return big.NewInt(0)
	}

	str := n.String()
	if len(str) <= precision {
		return new(big.Int).Set(n)
	}

	// 保留前 precision 位, 其余置零
	result := new(big.Int)
	result.SetString(str[:precision], 10)

	// 乘以 10^(len-precision)
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(str)-precision)), nil)
	result.Mul(result, multiplier)

	return result
}

// ============== Hint 广播 (简化版) ==============

// HintBroadcaster Hint 广播器接口
type HintBroadcaster interface {
	Broadcast(hint *types.Hint) error
}

// LogHintBroadcaster 简单的日志广播器 (用于 demo)
type LogHintBroadcaster struct{}

func (b *LogHintBroadcaster) Broadcast(hint *types.Hint) error {
	mylog.Logger.Info().
		Str("matchingHash", hint.Hash.Hex()).
		Int("txCount", len(hint.Txs)).
		Int("logCount", len(hint.Logs)).
		Msg("📢 Broadcasting hint")
	return nil
}
