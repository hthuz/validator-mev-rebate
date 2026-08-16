package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"rebate/mylog"
	"rebate/pkg/utils"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const DefaultPublicRPCURL = "https://ethereum-rpc.publicnode.com"

var csvHeader = []string{
	"block_number", "block_hash", "block_timestamp", "block_base_fee_per_gas_wei",
	"tx_index", "tx_hash", "tx_type", "from", "to", "is_contract_creation",
	"nonce", "value_wei", "gas_limit", "gas_price_wei", "max_fee_per_gas_wei",
	"max_priority_fee_per_gas_wei", "max_fee_per_blob_gas_wei", "chain_id", "input",
	"method_id", "access_list_len", "blob_versioned_hashes", "receipt_status",
	"receipt_gas_used", "receipt_cumulative_gas_used", "receipt_effective_gas_price_wei",
	"receipt_contract_address", "receipt_blob_gas_used", "receipt_blob_gas_price_wei",
	"logs_count", "raw_tx",
}

type Collector struct {
	client *ethclient.Client
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

		block, err := c.client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
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
		mylog.Logger.Info().Uint64("block", blockNumber).Int("txs", len(rows)).
			Int("totalTxs", totalTxs).Msg("Collected Ethereum block")
	}
	return nil
}

// collectBlock uses one full block request and one logs request. Receipt-only
// fields remain empty because they are not available from these RPC methods.
func (c *Collector) collectBlock(ctx context.Context, block *etypes.Block, receiptWorkers int) ([][]string, error) {
	_ = receiptWorkers // Kept for CLI compatibility.
	txs := block.Transactions()
	if len(txs) == 0 {
		return nil, nil
	}

	blockHash := block.Hash()
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &blockHash,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch logs for block %d: %w", block.NumberU64(), err)
	}
	logsByTx := make(map[common.Hash]int)
	for _, logEntry := range logs {
		logsByTx[logEntry.TxHash]++
	}

	rows := make([][]string, 0, len(txs))
	for idx, tx := range txs {
		row, err := buildRecordFromLogs(block, idx, tx, logsByTx[tx.Hash()])
		if err != nil {
			return nil, fmt.Errorf("build row for tx %s: %w", tx.Hash().Hex(), err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func buildRecord(block *etypes.Block, txIndex int, tx *etypes.Transaction, receipt *etypes.Receipt) ([]string, error) {
	if receipt == nil {
		return buildRecordFromLogs(block, txIndex, tx, 0)
	}
	return buildRecordWithReceipt(block, txIndex, tx, receipt, len(receipt.Logs))
}

func buildRecordFromLogs(block *etypes.Block, txIndex int, tx *etypes.Transaction, logsCount int) ([]string, error) {
	return buildRecordWithReceipt(block, txIndex, tx, nil, logsCount)
}

func buildRecordWithReceipt(block *etypes.Block, txIndex int, tx *etypes.Transaction, receipt *etypes.Receipt, logsCount int) ([]string, error) {
	rawTx, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal raw tx: %w", err)
	}
	from := ""
	if sender, err := utils.RecoverTransactionSender(tx); err == nil {
		from = sender.Hex()
	}
	to := ""
	if tx.To() != nil {
		to = tx.To().Hex()
	}

	status, gasUsed, cumulativeGasUsed, effectiveGasPrice := "", "", "", ""
	contractAddress, blobGasUsed, blobGasPrice := "", "", ""
	if receipt != nil {
		status = strconv.FormatUint(uint64(receipt.Status), 10)
		gasUsed = strconv.FormatUint(receipt.GasUsed, 10)
		cumulativeGasUsed = strconv.FormatUint(receipt.CumulativeGasUsed, 10)
		effectiveGasPrice = formatBigInt(receipt.EffectiveGasPrice)
		contractAddress = formatAddress(receipt.ContractAddress)
		blobGasUsed = formatBlobGasUsed(tx.Type(), receipt.BlobGasUsed)
		blobGasPrice = formatBigInt(receipt.BlobGasPrice)
	}

	return []string{
		strconv.FormatUint(block.NumberU64(), 10), block.Hash().Hex(),
		strconv.FormatUint(block.Time(), 10), formatBigInt(block.BaseFee()),
		strconv.Itoa(txIndex), tx.Hash().Hex(), strconv.FormatUint(uint64(tx.Type()), 10),
		from, to, strconv.FormatBool(tx.To() == nil), strconv.FormatUint(tx.Nonce(), 10),
		formatBigInt(tx.Value()), strconv.FormatUint(tx.Gas(), 10), formatBigInt(tx.GasPrice()),
		formatBigInt(tx.GasFeeCap()), formatBigInt(tx.GasTipCap()), formatBigInt(tx.BlobGasFeeCap()),
		formatBigInt(tx.ChainId()), hexutil.Encode(tx.Data()), methodID(tx.Data()),
		strconv.Itoa(len(tx.AccessList())), joinHashes(tx.BlobHashes()), status, gasUsed,
		cumulativeGasUsed, effectiveGasPrice, contractAddress, blobGasUsed, blobGasPrice,
		strconv.Itoa(logsCount), hexutil.Encode(rawTx),
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
