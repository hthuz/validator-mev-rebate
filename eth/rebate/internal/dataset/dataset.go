package dataset

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
)

type Transaction struct {
	BlockNumber       uint64
	BlockHash         common.Hash
	BlockTimestamp    uint64
	BlockBaseFee      *big.Int
	Index             int
	Hash              common.Hash
	From              common.Address
	To                *common.Address
	Nonce             uint64
	GasLimit          uint64
	EffectiveGasPrice *big.Int
	ReceiptGasUsed    uint64
	LogsCount         int
	MethodID          string
	RawTx             hexutil.Bytes
	DecodedTx         *etypes.Transaction
}

type Block struct {
	Number       uint64
	Hash         common.Hash
	Timestamp    uint64
	BaseFee      *big.Int
	Transactions []*Transaction
}

type Dataset struct {
	blocks         []*Block
	blocksByNumber map[uint64]*Block
	txByHash       map[common.Hash]*Transaction
	transactions   []*Transaction
}

func LoadCSV(path string) (*Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open dataset %q: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("dataset %q is empty", path)
	}

	header := indexHeader(records[0])
	requiredColumns := []string{
		"block_number",
		"block_hash",
		"block_timestamp",
		"block_base_fee_per_gas_wei",
		"tx_index",
		"tx_hash",
		"from",
		"to",
		"nonce",
		"gas_limit",
		"receipt_gas_used",
		"receipt_effective_gas_price_wei",
		"logs_count",
		"method_id",
		"raw_tx",
	}
	for _, column := range requiredColumns {
		if _, ok := header[column]; !ok {
			return nil, fmt.Errorf("dataset missing required column %q", column)
		}
	}

	ds := &Dataset{
		blocksByNumber: make(map[uint64]*Block),
		txByHash:       make(map[common.Hash]*Transaction),
	}

	for rowIdx, row := range records[1:] {
		tx, err := parseTransactionRow(header, row)
		if err != nil {
			return nil, fmt.Errorf("parse row %d: %w", rowIdx+2, err)
		}

		block, ok := ds.blocksByNumber[tx.BlockNumber]
		if !ok {
			block = &Block{
				Number:    tx.BlockNumber,
				Hash:      tx.BlockHash,
				Timestamp: tx.BlockTimestamp,
				BaseFee:   cloneBigInt(tx.BlockBaseFee),
			}
			ds.blocksByNumber[tx.BlockNumber] = block
			ds.blocks = append(ds.blocks, block)
		}

		block.Transactions = append(block.Transactions, tx)
		ds.transactions = append(ds.transactions, tx)
		ds.txByHash[tx.Hash] = tx
	}

	sort.Slice(ds.blocks, func(i, j int) bool {
		return ds.blocks[i].Number < ds.blocks[j].Number
	})
	for _, block := range ds.blocks {
		sort.Slice(block.Transactions, func(i, j int) bool {
			return block.Transactions[i].Index < block.Transactions[j].Index
		})
	}

	return ds, nil
}

func (d *Dataset) Blocks() []*Block {
	return d.blocks
}

func (d *Dataset) Transactions() []*Transaction {
	return d.transactions
}

func (d *Dataset) TransactionByHash(hash common.Hash) (*Transaction, bool) {
	tx, ok := d.txByHash[hash]
	return tx, ok
}

func (d *Dataset) FirstBlock() (*Block, bool) {
	if len(d.blocks) == 0 {
		return nil, false
	}
	return d.blocks[0], true
}

func (d *Dataset) LatestBlock() (*Block, bool) {
	if len(d.blocks) == 0 {
		return nil, false
	}
	return d.blocks[len(d.blocks)-1], true
}

func (d *Dataset) BlockByNumber(number uint64) (*Block, bool) {
	block, ok := d.blocksByNumber[number]
	return block, ok
}

func (d *Dataset) NextBlockAtOrAfter(number uint64) (*Block, bool) {
	if len(d.blocks) == 0 {
		return nil, false
	}

	idx := sort.Search(len(d.blocks), func(i int) bool {
		return d.blocks[i].Number >= number
	})
	if idx >= len(d.blocks) {
		return nil, false
	}
	return d.blocks[idx], true
}

func indexHeader(header []string) map[string]int {
	indices := make(map[string]int, len(header))
	for idx, name := range header {
		indices[name] = idx
	}
	return indices
}

func parseTransactionRow(header map[string]int, row []string) (*Transaction, error) {
	rawTx, err := hexutil.Decode(getColumn(header, row, "raw_tx"))
	if err != nil {
		return nil, fmt.Errorf("decode raw_tx: %w", err)
	}

	decodedTx := new(etypes.Transaction)
	if err := decodedTx.UnmarshalBinary(rawTx); err != nil {
		return nil, fmt.Errorf("decode transaction: %w", err)
	}

	var to *common.Address
	if value := getColumn(header, row, "to"); value != "" {
		addr := common.HexToAddress(value)
		to = &addr
	}

	return &Transaction{
		BlockNumber:       mustParseUint64(getColumn(header, row, "block_number")),
		BlockHash:         common.HexToHash(getColumn(header, row, "block_hash")),
		BlockTimestamp:    mustParseUint64(getColumn(header, row, "block_timestamp")),
		BlockBaseFee:      mustParseBigInt(getColumn(header, row, "block_base_fee_per_gas_wei")),
		Index:             mustParseInt(getColumn(header, row, "tx_index")),
		Hash:              common.HexToHash(getColumn(header, row, "tx_hash")),
		From:              common.HexToAddress(getColumn(header, row, "from")),
		To:                to,
		Nonce:             mustParseUint64(getColumn(header, row, "nonce")),
		GasLimit:          mustParseUint64(getColumn(header, row, "gas_limit")),
		EffectiveGasPrice: mustParseBigInt(getColumn(header, row, "receipt_effective_gas_price_wei")),
		ReceiptGasUsed:    mustParseUint64(getColumn(header, row, "receipt_gas_used")),
		LogsCount:         mustParseInt(getColumn(header, row, "logs_count")),
		MethodID:          getColumn(header, row, "method_id"),
		RawTx:             rawTx,
		DecodedTx:         decodedTx,
	}, nil
}

func getColumn(header map[string]int, row []string, key string) string {
	idx, ok := header[key]
	if !ok || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func mustParseUint64(value string) uint64 {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseUint(value, 10, 64)
	return parsed
}

func mustParseInt(value string) int {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func mustParseBigInt(value string) *big.Int {
	if value == "" {
		return big.NewInt(0)
	}
	result := new(big.Int)
	if _, ok := result.SetString(value, 10); !ok {
		return big.NewInt(0)
	}
	return result
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(value)
}
