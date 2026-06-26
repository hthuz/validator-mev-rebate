package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"rebate/mylog"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const DefaultPublicRPCURL = "https://ethereum-rpc.publicnode.com"

var csvHeader = []string{
	"block_number",
	"block_hash",
	"block_timestamp",
	"block_base_fee_per_gas_wei",
	"tx_index",
	"tx_hash",
	"tx_type",
	"from",
	"to",
	"is_contract_creation",
	"nonce",
	"value_wei",
	"gas_limit",
	"gas_price_wei",
	"max_fee_per_gas_wei",
	"max_priority_fee_per_gas_wei",
	"max_fee_per_blob_gas_wei",
	"chain_id",
	"input",
	"method_id",
	"access_list_len",
	"blob_versioned_hashes",
	"receipt_status",
	"receipt_gas_used",
	"receipt_cumulative_gas_used",
	"receipt_effective_gas_price_wei",
	"receipt_contract_address",
	"receipt_blob_gas_used",
	"receipt_blob_gas_price_wei",
	"logs_count",
	"raw_tx",
}

type Collector struct {
	client *ethclient.Client
}

type indexedRecord struct {
	index int
	row   []string
}

func New(client *ethclient.Client) *Collector {
	return &Collector{client: client}
}

func (c *Collector) LatestBlockNumber(ctx context.Context) (uint64, error) {
	header, err := c.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("get latest header: %w", err)
	}
	return header.Number.Uint64(), nil
}

func (c *Collector) WriteCSV(ctx context.Context, startBlock, endBlock uint64, w io.Writer, receiptWorkers int) error {
	if startBlock > endBlock {
		return fmt.Errorf("invalid block range: start %d > end %d", startBlock, endBlock)
	}
	if receiptWorkers <= 0 {
		return fmt.Errorf("receiptWorkers must be > 0")
	}

	writer := csv.NewWriter(w)
	if err := writer.Write(csvHeader); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	totalTxs := 0
	for blockNumber := startBlock; blockNumber <= endBlock; blockNumber++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		block, err := c.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		if err != nil {
			return fmt.Errorf("fetch block %d: %w", blockNumber, err)
		}

		rows, err := c.collectBlock(ctx, block, receiptWorkers)
		if err != nil {
			return fmt.Errorf("collect block %d: %w", blockNumber, err)
		}

		for _, row := range rows {
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("write csv row for block %d: %w", blockNumber, err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("flush csv writer for block %d: %w", blockNumber, err)
		}

		totalTxs += len(rows)
		mylog.Logger.Info().
			Uint64("block", blockNumber).
			Int("txs", len(rows)).
			Int("totalTxs", totalTxs).
			Msg("Collected Ethereum block")
	}

	return nil
}

func (c *Collector) collectBlock(ctx context.Context, block *etypes.Block, receiptWorkers int) ([][]string, error) {
	txs := block.Transactions()
	if len(txs) == 0 {
		return nil, nil
	}

	jobCh := make(chan int)
	resultCh := make(chan indexedRecord, len(txs))
	errCh := make(chan error, 1)

	worker := func() {
		for idx := range jobCh {
			tx := txs[idx]
			receipt, err := c.client.TransactionReceipt(ctx, tx.Hash())
			if err != nil {
				select {
				case errCh <- fmt.Errorf("fetch receipt for tx %s: %w", tx.Hash().Hex(), err):
				default:
				}
				return
			}

			row, err := buildRecord(block, idx, tx, receipt)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("build row for tx %s: %w", tx.Hash().Hex(), err):
				default:
				}
				return
			}

			select {
			case resultCh <- indexedRecord{index: idx, row: row}:
			case <-ctx.Done():
				return
			}
		}
	}

	workerCount := receiptWorkers
	if workerCount > len(txs) {
		workerCount = len(txs)
	}
	for i := 0; i < workerCount; i++ {
		go worker()
	}

	go func() {
		defer close(jobCh)
		for idx := range txs {
			select {
			case jobCh <- idx:
			case <-ctx.Done():
				return
			}
		}
	}()

	records := make([]indexedRecord, 0, len(txs))
	for len(records) < len(txs) {
		select {
		case err := <-errCh:
			if err != nil {
				return nil, err
			}
		case record := <-resultCh:
			records = append(records, record)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].index < records[j].index
	})

	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, record.row)
	}

	return rows, nil
}

func buildRecord(block *etypes.Block, txIndex int, tx *etypes.Transaction, receipt *etypes.Receipt) ([]string, error) {
	rawTx, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal raw tx: %w", err)
	}

	from := ""
	if sender, err := etypes.Sender(etypes.LatestSignerForChainID(tx.ChainId()), tx); err == nil {
		from = sender.Hex()
	}

	to := ""
	if tx.To() != nil {
		to = tx.To().Hex()
	}

	return []string{
		strconv.FormatUint(block.NumberU64(), 10),
		block.Hash().Hex(),
		strconv.FormatUint(block.Time(), 10),
		formatBigInt(block.BaseFee()),
		strconv.Itoa(txIndex),
		tx.Hash().Hex(),
		strconv.FormatUint(uint64(tx.Type()), 10),
		from,
		to,
		strconv.FormatBool(tx.To() == nil),
		strconv.FormatUint(tx.Nonce(), 10),
		formatBigInt(tx.Value()),
		strconv.FormatUint(tx.Gas(), 10),
		formatBigInt(tx.GasPrice()),
		formatBigInt(tx.GasFeeCap()),
		formatBigInt(tx.GasTipCap()),
		formatBigInt(tx.BlobGasFeeCap()),
		formatBigInt(tx.ChainId()),
		hexutil.Encode(tx.Data()),
		methodID(tx.Data()),
		strconv.Itoa(len(tx.AccessList())),
		joinHashes(tx.BlobHashes()),
		strconv.FormatUint(uint64(receipt.Status), 10),
		strconv.FormatUint(receipt.GasUsed, 10),
		strconv.FormatUint(receipt.CumulativeGasUsed, 10),
		formatBigInt(receipt.EffectiveGasPrice),
		formatAddress(receipt.ContractAddress),
		formatBlobGasUsed(tx.Type(), receipt.BlobGasUsed),
		formatBigInt(receipt.BlobGasPrice),
		strconv.Itoa(len(receipt.Logs)),
		hexutil.Encode(rawTx),
	}, nil
}

func formatBigInt(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

func formatAddress(addr common.Address) string {
	if addr == (common.Address{}) {
		return ""
	}
	return addr.Hex()
}

func formatBlobGasUsed(txType uint8, blobGasUsed uint64) string {
	if txType != etypes.BlobTxType && blobGasUsed == 0 {
		return ""
	}
	return strconv.FormatUint(blobGasUsed, 10)
}

func joinHashes(hashes []common.Hash) string {
	if len(hashes) == 0 {
		return ""
	}

	values := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		values = append(values, hash.Hex())
	}
	return strings.Join(values, ";")
}

func methodID(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	return hexutil.Encode(data[:4])
}
