package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

// ============== 验证常量 ==============

const (
	MaxBlockOffset  = 5        // Bundle 不能计划太远的未来
	MaxBlockRange   = 30       // maxBlock - blockNumber 不能超过 30
	MaxBodySize     = 50       // Bundle 最多 50 个元素
	MaxNestingLevel = 1        // Bundle 最多嵌套 1 层
)

// ============== 验证错误 ==============

var (
	ErrInvalidVersion          = errors.New("invalid version, must be 'beta-1' or 'v0.1'")
	ErrInvalidInclusion        = errors.New("invalid inclusion: maxBlock must be >= block")
	ErrBlockRangeTooLarge      = errors.New("block range too large")
	ErrBlockTooFarInFuture     = errors.New("block too far in future")
	ErrBundleTooOld            = errors.New("bundle is too old, maxBlock already passed")
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
func ValidateBundle(bundle *SendMevBundleArgs, currentBlock uint64, signer *ecdsa.PrivateKey) (common.Hash, bool, error) {
	return validateBundleInner(bundle, currentBlock, signer, 0)
}

func validateBundleInner(bundle *SendMevBundleArgs, currentBlock uint64, signer *ecdsa.PrivateKey, level int) (common.Hash, bool, error) {
	// 检查嵌套层级
	if level > MaxNestingLevel {
		return common.Hash{}, false, ErrNestingTooDeep
	}

	// 1. 验证版本
	if bundle.Version != "beta-1" && bundle.Version != "v0.1" {
		return common.Hash{}, false, ErrInvalidVersion
	}

	// 2. 验证 Inclusion
	if err := validateInclusion(&bundle.Inclusion, currentBlock); err != nil {
		return common.Hash{}, false, err
	}

	// 3. 验证 Body 并计算 Hash
	bundleHash, bodyHashes, hasUnmatchedHash, err := validateBody(bundle.Body, currentBlock, signer, level)
	if err != nil {
		return common.Hash{}, false, err
	}

	// 4. 验证 Validity (退款配置)
	if err := validateValidity(&bundle.Validity, len(bundle.Body)); err != nil {
		return common.Hash{}, false, err
	}

	// 5. 设置元数据
	if bundle.Metadata == nil {
		bundle.Metadata = &MevBundleMetadata{}
	}
	bundle.Metadata.BundleHash = bundleHash
	bundle.Metadata.BodyHashes = bodyHashes

	return bundleHash, hasUnmatchedHash, nil
}

// validateInclusion 验证 Inclusion 范围
func validateInclusion(inclusion *MevBundleInclusion, currentBlock uint64) error {
	blockNumber := uint64(inclusion.BlockNumber)
	maxBlock := uint64(inclusion.MaxBlock)

	// maxBlock 必须 >= blockNumber
	if maxBlock < blockNumber {
		return ErrInvalidInclusion
	}

	// 范围不能太大
	if maxBlock-blockNumber > MaxBlockRange {
		return ErrBlockRangeTooLarge
	}

	// 不能太远的未来
	if blockNumber > currentBlock+MaxBlockOffset {
		return ErrBlockTooFarInFuture
	}

	// 不能已过期
	if maxBlock < currentBlock {
		return ErrBundleTooOld
	}

	return nil
}

// validateBody 验证 Body 并计算哈希
func validateBody(body []MevBundleBody, currentBlock uint64, signer *ecdsa.PrivateKey, level int) (common.Hash, []common.Hash, bool, error) {
	if len(body) == 0 {
		return common.Hash{}, nil, false, ErrEmptyBody
	}

	if len(body) > MaxBodySize {
		return common.Hash{}, nil, false, ErrBodyTooLarge
	}

	var bodyHashes []common.Hash
	hasUnmatchedHash := false
	seenHash := false

	for i, elem := range body {
		// 检查元素有效性 (必须恰好有一个)
		count := 0
		if elem.Tx != nil {
			count++
		}
		if elem.Hash != nil {
			count++
		}
		if elem.Bundle != nil {
			count++
		}

		if count != 1 {
			return common.Hash{}, nil, false, ErrInvalidBodyElement
		}

		var elemHash common.Hash

		if elem.Tx != nil {
			// 解析并验证交易
			var tx types.Transaction
			if err := rlp.DecodeBytes(*elem.Tx, &tx); err != nil {
				return common.Hash{}, nil, false, fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
			}
			elemHash = tx.Hash()
		} else if elem.Hash != nil {
			// Hash 引用必须是第一个元素
			if i != 0 {
				return common.Hash{}, nil, false, ErrHashNotFirstElement
			}
			if seenHash {
				return common.Hash{}, nil, false, ErrMultipleHashElements
			}
			seenHash = true
			hasUnmatchedHash = true
			elemHash = *elem.Hash
		} else if elem.Bundle != nil {
			// 递归验证嵌套 Bundle
			nestedHash, _, err := validateBundleInner(elem.Bundle, currentBlock, signer, level+1)
			if err != nil {
				return common.Hash{}, nil, false, fmt.Errorf("nested bundle: %w", err)
			}
			elemHash = nestedHash
		}

		bodyHashes = append(bodyHashes, elemHash)
	}

	// 计算 Bundle Hash = keccak256(hash1 || hash2 || ...)
	bundleHash := calculateBundleHash(bodyHashes)

	return bundleHash, bodyHashes, hasUnmatchedHash, nil
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

// calculateBundleHash 计算 Bundle 哈希
func calculateBundleHash(hashes []common.Hash) common.Hash {
	hasher := sha3.NewLegacyKeccak256()
	for _, h := range hashes {
		hasher.Write(h[:])
	}
	var result common.Hash
	copy(result[:], hasher.Sum(nil))
	return result
}

// calculateMatchingHash 计算用于 Hint 匹配的哈希
// 这个哈希用于搜索者引用 Bundle 进行 backrun
func calculateMatchingHash(bundleHash common.Hash, signer *ecdsa.PrivateKey) common.Hash {
	// 使用签名者私钥对 Bundle Hash 签名，然后取哈希
	sig, err := crypto.Sign(bundleHash[:], signer)
	if err != nil {
		return bundleHash // 降级为使用原始哈希
	}
	return crypto.Keccak256Hash(sig)
}

// GetTransactionSender 获取交易签名者地址
func GetTransactionSender(txBytes []byte) (common.Address, error) {
	var tx types.Transaction
	if err := rlp.DecodeBytes(txBytes, &tx); err != nil {
		return common.Address{}, err
	}

	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := types.Sender(signer, &tx)
	if err != nil {
		return common.Address{}, err
	}

	return sender, nil
}
