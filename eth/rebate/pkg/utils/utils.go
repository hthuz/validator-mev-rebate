package utils

import (
	"crypto/ecdsa"
	"encoding/json"

	"rebate/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

// GenerateSigner 生成签名密钥 (用于 MatchingHash)
func GenerateSigner() (*ecdsa.PrivateKey, error) {
	return crypto.GenerateKey()
}

// CalculateBundleHash 计算 Bundle 哈希
func CalculateBundleHash(hashes []common.Hash) common.Hash {
	hasher := sha3.NewLegacyKeccak256()
	for _, h := range hashes {
		hasher.Write(h[:])
	}
	var result common.Hash
	copy(result[:], hasher.Sum(nil))
	return result
}

// CalculateMatchingHash 计算用于 Hint 匹配的哈希
// 这个哈希用于搜索者引用 Bundle 进行 backrun
func CalculateMatchingHash(bundleHash common.Hash, signer *ecdsa.PrivateKey) common.Hash {
	// 使用签名者私钥对 Bundle Hash 签名，然后取哈希
	sig, err := crypto.Sign(bundleHash[:], signer)
	if err != nil {
		return bundleHash // 降级为使用原始哈希
	}
	return crypto.Keccak256Hash(sig)
}

// GetTransactionSender 获取交易签名者地址
func GetTransactionSender(txBytes []byte) (common.Address, error) {
	var tx etypes.Transaction
	if err := rlp.DecodeBytes(txBytes, &tx); err != nil {
		return common.Address{}, err
	}

	signer := etypes.LatestSignerForChainID(tx.ChainId())
	sender, err := etypes.Sender(signer, &tx)
	if err != nil {
		return common.Address{}, err
	}

	return sender, nil
}

// ============== 序列化辅助 ==============

// SerializeBundle 序列化 Bundle (用于存储/传输)
func SerializeBundle(bundle *types.SendMevBundleArgs) ([]byte, error) {
	return json.Marshal(bundle)
}

// DeserializeBundle 反序列化 Bundle
func DeserializeBundle(data []byte) (*types.SendMevBundleArgs, error) {
	var bundle types.SendMevBundleArgs
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}
