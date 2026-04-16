package main

import (
	"encoding/base64"
	"strings"
)

// ============== Hint 提取常量 ==============

const (
	HintPriorityFeePrecisionDigits = 3 // Priority fee 精度 (保留3位有效数字)
	HintCUPrecisionDigits          = 2 // Compute Units 精度 (保留2位有效数字)
)

// 特殊日志签名 (Solana DEX 相关事件)
// 这些是在 Solana 上常见的 DEX 程序地址
var SpecialLogPrograms = map[string]string{
	"675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8": "Raydium AMM",    // Raydium
	"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P": "Pump.fun",       // Pump.fun
	"JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4": "Jupiter",        // Jupiter
	"PhoeNiXZ8ByJGLkxNfZRnkUfjvmuYqLR89jjFHGqdP4": "Phoenix",        // Phoenix
	"LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo": "Meteora",        // Meteora
}

// ============== Hint 提取函数 ==============

// ExtractHints 从 Bundle 和模拟结果中提取 Hints
func ExtractHints(bundle *SendMevBundleArgs, simResult *SimMevBundleResponse) *Hint {
	if bundle.Privacy == nil || bundle.Privacy.Hints == HintNone {
		return nil
	}

	hints := bundle.Privacy.Hints
	hint := &Hint{
		Hash: bundle.Metadata.MatchingHash,
	}

	// 提取交易 Hints
	hint.Txs = extractTxHints(bundle.Body, hints)

	// 提取日志 Hints (从模拟结果)
	if simResult != nil && (hints.Has(HintLogs) || hints.Has(HintSpecialLogs)) {
		hint.Logs = extractLogHints(simResult.BodyLogs, hints)
	}

	// 提取 Priority Fee 和 CU 信息 (降低精度保护隐私)
	if simResult != nil {
		if simResult.PriorityFee > 0 {
			hint.PriorityFee = reducePrecision(simResult.PriorityFee, HintPriorityFeePrecisionDigits)
		}
		if simResult.ComputeUnitsUsed > 0 {
			hint.ComputeUnits = reducePrecision(simResult.ComputeUnitsUsed, HintCUPrecisionDigits)
		}
	}

	return hint
}

// extractTxHints 提取交易相关的 Hints
func extractTxHints(body []MevBundleBody, hints HintIntent) []TxHint {
	var txHints []TxHint

	for _, elem := range body {
		if elem.Tx == "" {
			continue
		}

		// 解码交易
		txBytes, err := base64.StdEncoding.DecodeString(elem.Tx)
		if err != nil {
			continue
		}

		txHint := TxHint{}

		// 交易哈希 (使用数据哈希作为标识)
		if hints.Has(HintTxHash) || hints.Has(HintHash) {
			hash := base64.StdEncoding.EncodeToString(txBytes[:min(32, len(txBytes))])
			txHint.Hash = hash
		}

		// 尝试识别程序 ID (简化版)
		if hints.Has(HintProgramID) {
			// 在真实实现中，需要解析 Solana 交易格式
			// 这里简化处理，从交易中提取可能的程序地址
			txHint.ProgramID = extractProgramID(txBytes)
		}

		// 指令类型
		if hints.Has(HintInstructionType) {
			txHint.InstructionType = identifyInstructionType(txBytes)
		}

		// 完整数据 (通常不建议，因为数据量大)
		if hints.Has(HintTransactionData) {
			// 只返回前 64 字节
			if len(txBytes) > 64 {
				txHint.Data = base64.StdEncoding.EncodeToString(txBytes[:64])
			} else {
				txHint.Data = elem.Tx
			}
		}

		txHints = append(txHints, txHint)
	}

	return txHints
}

// extractLogHints 提取日志相关的 Hints
func extractLogHints(bodyLogs []SimMevBodyLogs, hints HintIntent) []CleanLog {
	var cleanLogs []CleanLog

	for _, logs := range bodyLogs {
		for _, log := range logs.TxLogs {
			// 如果只要求 SpecialLogs, 过滤非特殊日志
			if hints.Has(HintSpecialLogs) && !hints.Has(HintLogs) {
				if !isSpecialLog(log) {
					continue
				}
				// 对于特殊日志, 清理敏感信息
				cleanLogs = append(cleanLogs, cleanSpecialLog(log))
			} else if hints.Has(HintLogs) {
				// 完整日志
				cleanLogs = append(cleanLogs, CleanLog{
					Program: log.Program,
					Events:  log.Events,
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
func isSpecialLog(log SimLog) bool {
	_, isSpecial := SpecialLogPrograms[log.Program]
	return isSpecial
}

// cleanSpecialLog 清理特殊日志 (只保留程序地址和事件类型)
func cleanSpecialLog(log SimLog) CleanLog {
	return CleanLog{
		Program: log.Program,
		Events:  log.Events,
		// 不包含 Data (隐藏具体交易数量)
	}
}

// extractProgramID 从交易数据中提取程序 ID (简化版)
func extractProgramID(txBytes []byte) string {
	// 在真实实现中，需要解析 Solana 交易结构
	// 这里简化处理，检查是否包含已知的 DEX 程序地址片段
	knownPrograms := []string{
		"675kPX9MHTjS2zt1qfr1",
		"6EF8rrecthR5Dkzon8N",
		"JUP6LkbZbjS1jKKwapd",
	}

	txStr := string(txBytes)
	for _, program := range knownPrograms {
		if strings.Contains(txStr, program) {
			return program + "..."
		}
	}

	return "unknown"
}

// identifyInstructionType 识别指令类型 (简化版)
func identifyInstructionType(txBytes []byte) string {
	// 简化实现：根据交易大小和模式猜测指令类型
	if len(txBytes) < 200 {
		return "transfer"
	} else if len(txBytes) < 500 {
		return "swap"
	} else {
		return "complex"
	}
}

// reducePrecision 降低数值精度 (保护隐私)
func reducePrecision(n uint64, precision int) uint64 {
	if n == 0 {
		return 0
	}

	str := itoa(n)
	if len(str) <= precision {
		return n
	}

	// 保留前 precision 位, 其余置零
	factor := uint64(1)
	for i := 0; i < len(str)-precision; i++ {
		factor *= 10
	}

	return (n / factor) * factor
}

// itoa uint64 转字符串
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============== Hint 广播 (简化版) ==============

// HintBroadcaster Hint 广播器接口
type HintBroadcaster interface {
	Broadcast(hint *Hint) error
}

// LogHintBroadcaster 简单的日志广播器 (用于 demo)
type LogHintBroadcaster struct{}

func (b *LogHintBroadcaster) Broadcast(hint *Hint) error {
	logger.Info().
		Str("matchingHash", hint.Hash).
		Int("txCount", len(hint.Txs)).
		Int("logCount", len(hint.Logs)).
		Msg("Broadcasting hint")

	// 输出详细的 hint 信息
	for _, tx := range hint.Txs {
		logger.Info().
			Str("program", tx.ProgramID).
			Str("type", tx.InstructionType).
			Msg("  Transaction hint")
	}

	for _, log := range hint.Logs {
		logger.Info().
			Str("program", log.Program).
			Strs("events", log.Events).
			Msg("  Log hint")
	}

	return nil
}
