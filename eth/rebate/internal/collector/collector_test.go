package collector

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestBuildRecordIncludesExpectedFields(t *testing.T) {
	privateKey, err := crypto.HexToECDSA("4c0883a6910395b2c7f59f63a2fce9e87f32be8f8b18d76e0c7ab6b5234f6b31")
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}

	chainID := big.NewInt(1)
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	tx := etypes.NewTx(&etypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     7,
		GasTipCap: big.NewInt(2_000_000_000),
		GasFeeCap: big.NewInt(30_000_000_000),
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(123456789),
		Data:      common.FromHex("0xa9059cbb0000000000000000000000000000000000000000000000000000000000000001"),
	})

	signedTx, err := etypes.SignTx(tx, etypes.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}

	header := &etypes.Header{
		Number:  big.NewInt(20_000_000),
		Time:    1_717_171_717,
		BaseFee: big.NewInt(12_000_000_000),
	}
	block := etypes.NewBlockWithHeader(header).WithBody(etypes.Body{
		Transactions: []*etypes.Transaction{signedTx},
	})

	receipt := &etypes.Receipt{
		Status:            etypes.ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		GasUsed:           21000,
		BlockNumber:       header.Number,
		BlockHash:         block.Hash(),
		TxHash:            signedTx.Hash(),
		EffectiveGasPrice: big.NewInt(15_000_000_000),
		Logs: []*etypes.Log{
			{Address: to},
		},
	}

	record, err := buildRecord(block, 0, signedTx, receipt)
	if err != nil {
		t.Fatalf("buildRecord: %v", err)
	}

	if got, want := len(record), len(csvHeader); got != want {
		t.Fatalf("unexpected column count: got %d want %d", got, want)
	}

	if got := record[0]; got != "20000000" {
		t.Fatalf("unexpected block number: %s", got)
	}
	if got := record[5]; got != signedTx.Hash().Hex() {
		t.Fatalf("unexpected tx hash: %s", got)
	}
	if got := record[7]; got == "" {
		t.Fatal("expected sender address to be populated")
	}
	if got := record[18]; got != "0xa9059cbb0000000000000000000000000000000000000000000000000000000000000001" {
		t.Fatalf("unexpected input: %s", got)
	}
	if got := record[19]; got != "0xa9059cbb" {
		t.Fatalf("unexpected method id: %s", got)
	}
	if got := record[22]; got != "1" {
		t.Fatalf("unexpected receipt status: %s", got)
	}
	if got := record[30]; got == "" {
		t.Fatal("expected raw tx to be populated")
	}
}

func TestMethodID(t *testing.T) {
	if got := methodID(nil); got != "" {
		t.Fatalf("expected empty method id, got %s", got)
	}
	if got := methodID([]byte{0x01, 0x02, 0x03}); got != "" {
		t.Fatalf("expected empty method id for short data, got %s", got)
	}
	if got := methodID([]byte{0xde, 0xad, 0xbe, 0xef, 0x01}); got != "0xdeadbeef" {
		t.Fatalf("unexpected method id: %s", got)
	}
}
