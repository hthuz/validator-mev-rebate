package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// ============== 验证常量 ==============

const (
	MaxSlotOffset   = 100      // Bundle 不能计划太远的未来 (约 40 秒)
	MaxSlotRange    = 300      // maxSlot - slot 不能超过 300 (约 2 分钟)
	MaxBodySize     = 50       // Bundle 最多 50 个元素
	MaxNestingLevel = 1        // Bundle 最多嵌套 1 层
)

// ============== 验证错误 ==============

var (
	ErrInvalidVersion          = errors.New("invalid version, must be 'solana-v0.1'")
	ErrInvalidInclusion        = errors.New("invalid inclusion: maxSlot must be >= slot")
	ErrSlotRangeTooLarge       = errors.New("slot range too large")
	ErrSlotTooFarInFuture      = errors.New("slot too far in future")
	ErrBundleTooOld            = errors.New("bundle is too old, maxSlot already passed")
	ErrEmptyBody               = errors.New("bundle body cannot be empty")
	ErrBodyTooLarge            = errors.New("bundle body too large")
	ErrInvalidBodyElement      = errors.New("invalid body element: must have exactly one of tx, hash, or bundle")
	ErrHashNotFirstElement     = errors.New("hash reference must be the first element")
	ErrMultipleHashElements    = errors.New("only one hash reference allowed")
	ErrNestingTooDeep          = errors.New("bundle nesting too deep")
	ErrInvalidTransaction      = errors.New("invalid transaction")
	ErrInvalidRefundPercent    = errors.New("invalid refund percent, must be 0-100")
	ErrRefundPercentExceeds100 = errors.New("total refund percent exceeds 100")
)

// ============== 验证函数 ==============

// ValidateBundle 验证 Bundle 并计算 Hash
// 返回: (bundleHash, hasUnmatchedHash, error)
func ValidateBundle(bundle *SendMevBundleArgs, currentSlot uint64) (string, bool, error) {
	return validateBundleInner(bundle, currentSlot, 0)
}

func validateBundleInner(bundle *SendMevBundleArgs, currentSlot uint64, level int) (string, bool, error) {
	// 检查嵌套层级
	if level > MaxNestingLevel {
		return "", false, ErrNestingTooDeep
	}

	// 1. 验证版本 (Solana 版本)
	if bundle.Version != "solana-v0.1" && bundle.Version != "solana-beta-1" {
		return "", false, ErrInvalidVersion
	}

	// 2. 验证 Inclusion
	if err := validateInclusion(&bundle.Inclusion, currentSlot); err != nil {
		return "", false, err
	}

	// 3. 验证 Body 并计算 Hash
	bundleHash, hasUnmatchedHash, err := validateBody(bundle.Body, currentSlot, level)
	if err != nil {
		return "", false, err
	}

	// 4. 验证 Validity (退款配置)
	if err := validateValidity(&bundle.Validity, len(bundle.Body)); err != nil {
		return "", false, err
	}

	return bundleHash, hasUnmatchedHash, nil
}

// validateInclusion 验证 Inclusion 范围
func validateInclusion(inclusion *MevBundleInclusion, currentSlot uint64) error {
	// maxSlot 必须 >= slot
	if inclusion.MaxSlot < inclusion.Slot {
		return ErrInvalidInclusion
	}

	// 范围不能太大
	if inclusion.MaxSlot-inclusion.Slot > MaxSlotRange {
		return ErrSlotRangeTooLarge
	}

	// 不能太远的未来
	if inclusion.Slot > currentSlot+MaxSlotOffset {
		return ErrSlotTooFarInFuture
	}

	// 不能已过期
	if inclusion.MaxSlot < currentSlot {
		return ErrBundleTooOld
	}

	return nil
}

// validateBody 验证 Body 并计算哈希
func validateBody(body []MevBundleBody, currentSlot uint64, level int) (string, bool, error) {
	if len(body) == 0 {
		return "", false, ErrEmptyBody
	}

	if len(body) > MaxBodySize {
		return "", false, ErrBodyTooLarge
	}

	var bodyHashes []string
	hasUnmatchedHash := false
	seenHash := false

	for i, elem := range body {
		// 检查元素有效性 (必须恰好有一个)
		count := 0
		if elem.Tx != "" {
			count++
		}
		if elem.Hash != "" {
			count++
		}
		if elem.Bundle != nil {
			count++
		}

		if count != 1 {
			return "", false, ErrInvalidBodyElement
		}

		var elemHash string

		if elem.Tx != "" {
			// 解析并验证 Solana 交易
			txBytes, err := base64.StdEncoding.DecodeString(elem.Tx)
			if err != nil {
				return "", false, fmt.Errorf("%w: invalid base64: %v", ErrInvalidTransaction, err)
			}

			// 尝试解析为 Solana 交易
			var tx solana.Transaction
			if err := tx.UnmarshalWithDecoder(solana.NewBinDecoder(txBytes)); err != nil {
				return "", false, fmt.Errorf("%w: invalid transaction: %v", ErrInvalidTransaction, err)
			}

			// 计算交易哈希 (签名)
			if len(tx.Signatures) > 0 {
				elemHash = tx.Signatures[0].String()
			} else {
				// 没有签名的交易，使用数据哈希
				hash := sha256.Sum256(txBytes)
				elemHash = base58Encode(hash[:])
			}
		} else if elem.Hash != "" {
			// Hash 引用必须是第一个元素
			if i != 0 {
				return "", false, ErrHashNotFirstElement
			}
			if seenHash {
				return "", false, ErrMultipleHashElements
			}
			seenHash = true
			hasUnmatchedHash = true
			elemHash = elem.Hash
		} else if elem.Bundle != nil {
			// 递归验证嵌套 Bundle
			nestedHash, _, err := validateBundleInner(elem.Bundle, currentSlot, level+1)
			if err != nil {
				return "", false, fmt.Errorf("nested bundle: %w", err)
			}
			elemHash = nestedHash
		}

		bodyHashes = append(bodyHashes, elemHash)
	}

	// 计算 Bundle Hash = SHA256(hash1 || hash2 || ...)
	bundleHash := calculateBundleHash(bodyHashes)

	return bundleHash, hasUnmatchedHash, nil
}

// validateValidity 验证退款配置
func validateValidity(validity *MevBundleValidity, bodyLen int) error {
	if validity == nil {
		return nil
	}

	// 验证 Refund 配置
	for _, r := range validity.Refund {
		if r.Percent < 0 || r.Percent > 100 {
			return ErrInvalidRefundPercent
		}
		if r.BodyIdx < 0 || r.BodyIdx >= bodyLen {
			return fmt.Errorf("invalid refund bodyIdx: %d", r.BodyIdx)
		}
	}

	// 验证 RefundConfig 总和不超过 100%
	total := 0
	for _, r := range validity.RefundConfig {
		if r.Percent < 0 || r.Percent > 100 {
			return ErrInvalidRefundPercent
		}
		total += r.Percent
	}
	if total > 100 {
		return ErrRefundPercentExceeds100
	}

	return nil
}

// calculateBundleHash 计算 Bundle 哈希 (使用 SHA256，然后 base58 编码)
func calculateBundleHash(hashes []string) string {
	hasher := sha256.New()
	for _, h := range hashes {
		hasher.Write([]byte(h))
	}
	return base58Encode(hasher.Sum(nil))
}

// calculateMatchingHash 计算用于 Hint 匹配的哈希
func calculateMatchingHash(bundleHash string) string {
	// 简单实现：对 bundleHash 再哈希一次
	hasher := sha256.New()
	hasher.Write([]byte(bundleHash))
	hasher.Write([]byte("matching_salt"))
	return base58Encode(hasher.Sum(nil))
}

// base58Encode base58 编码 (Solana 标准)
func base58Encode(data []byte) string {
	// 简单的 base58 实现，使用 solana-go 库的编码
	return solana.MustHashFromBase58(hex.EncodeToString(data)).String()
}

// GetTransactionSender 获取交易签名者地址
func GetTransactionSender(txBase64 string) (string, error) {
	txBytes, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return "", err
	}

	var tx solana.Transaction
	if err := tx.UnmarshalWithDecoder(solana.NewBinDecoder(txBytes)); err != nil {
		return "", err
	}

	// 返回第一个签名者的地址
	if len(tx.AccountMeta) > 0 {
		return tx.AccountMeta[0].PublicKey.String(), nil
	}

	return "", errors.New("no signers found")
}
