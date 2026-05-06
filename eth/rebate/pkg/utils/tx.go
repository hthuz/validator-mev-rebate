package utils

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func CreateTestTx() (hexutil.Bytes, error) {
	// 生成测试私钥
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	to := common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D")
	// 创建交易
	tx := etypes.NewTx(&etypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1000000000), // 1 Gwei
		Gas:      21000,
		To:       &to,
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		Data:     []byte{0xa9, 0x05, 0x9c, 0xbb},  // transfer 函数选择器
	})

	// 签名
	signer := etypes.NewEIP155Signer(big.NewInt(1))
	signedTx, err := etypes.SignTx(tx, signer, privateKey)
	if err != nil {
		return nil, err
	}

	// RLP 编码
	data, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return nil, err
	}

	return data, nil
}
