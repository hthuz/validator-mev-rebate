package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"rediswap/internal/auction"
	"rediswap/internal/pool"
	"rediswap/internal/store"
	"rediswap/pkg/types"

	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

// RediSwapAPI handles RPC methods
type RediSwapAPI struct {
	txStore       *store.TransactionStore
	beliefStore   *store.BeliefStore
	pool          *pool.Pool
	txCounter     uint64
	processingMu  sync.Mutex // Protects ProcessBlock from concurrent execution
}

// NewRediSwapAPI creates a new API instance
func NewRediSwapAPI(txStore *store.TransactionStore, beliefStore *store.BeliefStore, p *pool.Pool) *RediSwapAPI {
	return &RediSwapAPI{
		txStore:     txStore,
		beliefStore: beliefStore,
		pool:        p,
		txCounter:   0,
	}
}

// HandleRPC handles JSON-RPC requests
func (api *RediSwapAPI) HandleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, req.ID, -32700, "Parse error")
		return
	}

	var result interface{}
	var err error

	switch req.Method {
	case "rediswap_sendSwap":
		result, err = api.SendSwap(req.Params)
	case "rediswap_sendBelief":
		result, err = api.SendBelief(req.Params)
	case "rediswap_processBlock":
		result, err = api.ProcessBlock(req.Params)
	default:
		writeError(w, req.ID, -32601, "Method not found")
		return
	}

	if err != nil {
		writeError(w, req.ID, -32000, err.Error())
		return
	}

	writeSuccess(w, req.ID, result)
}

// SendSwap handles rediswap_sendSwap
func (api *RediSwapAPI) SendSwap(params []interface{}) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("missing parameters")
	}

	paramsJSON, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %v", err)
	}
	var args types.SendSwapArgs
	if err := json.Unmarshal(paramsJSON, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Validate direction
	direction := types.Direction(args.Direction)
	if direction != types.DirectionXToY && direction != types.DirectionYToX {
		return nil, fmt.Errorf("invalid direction: %s", args.Direction)
	}

	// Validate amounts
	if args.Input <= 0 {
		return nil, fmt.Errorf("input amount must be positive, got: %f", args.Input)
	}
	if args.Output <= 0 {
		return nil, fmt.Errorf("output amount must be positive, got: %f", args.Output)
	}

	// Create transaction
	txID := fmt.Sprintf("TX%d", atomic.AddUint64(&api.txCounter, 1))
	tx := &types.SwapTransaction{
		ID:        txID,
		Direction: direction,
		Input:     decimal.NewFromFloat(args.Input),
		Output:    decimal.NewFromFloat(args.Output),
	}

	if !api.txStore.Add(tx) {
		return nil, fmt.Errorf("duplicate transaction ID: %s", txID)
	}

	log.Info().
		Str("tx_id", txID).
		Str("direction", string(direction)).
		Float64("input", args.Input).
		Float64("output", args.Output).
		Msg("Swap transaction received")

	return map[string]interface{}{
		"tx_id":     txID,
		"status":    "pending",
		"direction": args.Direction,
	}, nil
}

// SendBelief handles rediswap_sendBelief
func (api *RediSwapAPI) SendBelief(params []interface{}) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("missing parameters")
	}

	paramsJSON, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %v", err)
	}
	var args types.SendBeliefArgs
	if err := json.Unmarshal(paramsJSON, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Validate belief
	if args.Belief <= 0 {
		return nil, fmt.Errorf("belief must be positive, got: %f", args.Belief)
	}

	belief := decimal.NewFromFloat(args.Belief)
	existed := api.beliefStore.Set(args.ArbID, belief)

	if existed {
		log.Warn().
			Str("arb_id", args.ArbID).
			Float64("belief", args.Belief).
			Msg("Belief updated (overwriting previous value)")
	} else {
		log.Info().
			Str("arb_id", args.ArbID).
			Float64("belief", args.Belief).
			Msg("Belief received")
	}

	return map[string]interface{}{
		"arb_id": args.ArbID,
		"belief": args.Belief,
		"status": "registered",
	}, nil
}

// ProcessBlock handles rediswap_processBlock - runs auctions and generates bundles
func (api *RediSwapAPI) ProcessBlock(params []interface{}) (interface{}, error) {
	// Prevent concurrent block processing
	api.processingMu.Lock()
	defer api.processingMu.Unlock()

	transactions := api.txStore.GetAll()
	beliefs := api.beliefStore.GetAll()

	if len(transactions) == 0 {
		return map[string]interface{}{
			"message": "no pending transactions",
		}, nil
	}

	if len(beliefs) == 0 {
		return map[string]interface{}{
			"message": "no arbitragers registered",
		}, nil
	}

	log.Info().
		Int("transactions", len(transactions)).
		Int("arbitragers", len(beliefs)).
		Msg("Processing block")

	var result types.BlockResult
	result.Bundles = make([]types.Bundle, 0)
	result.Auctions = make([]types.AuctionResult, 0)
	result.Refunds = make([]types.Refund, 0)

	// Process each transaction
	for _, tx := range transactions {
		log.Info().
			Str("tx_id", tx.ID).
			Str("direction", string(tx.Direction)).
			Str("input", tx.Input.String()).
			Str("output", tx.Output.String()).
			Msg("Running auction for transaction")

		bids := auction.CollectBids(api.pool, tx, beliefs)
		winner, payment := auction.RunAuction(bids)

		auctionResult := types.AuctionResult{
			TxID:    tx.ID,
			Winner:  winner,
			Payment: payment,
		}
		result.Auctions = append(result.Auctions, auctionResult)

		log.Info().
			Str("tx_id", tx.ID).
			Str("winner", winner).
			Str("payment", payment.String()).
			Msg("Auction completed")

		// Generate bundle if there's a winner
		if winner != "" {
			winnerBelief := beliefs[winner]
			bundle := auction.BuildBundle(api.pool, tx, winner, winnerBelief, payment)
			result.Bundles = append(result.Bundles, bundle)

			opsCount := 1 // UserTx always exists
			if bundle.FrontRun != nil {
				opsCount++
			}
			if bundle.BackRun != nil {
				opsCount++
			}

			log.Info().
				Str("tx_id", tx.ID).
				Str("winner", winner).
				Int("operations", opsCount).
				Str("net_profit", bundle.NetProfit.String()).
				Msg("Bundle generated")
		}

		// Refund payment to user
		if payment.GreaterThan(decimal.Zero) {
			refund := types.Refund{
				Receiver: "user_" + tx.ID,
				Amount:   payment,
			}
			result.Refunds = append(result.Refunds, refund)

			log.Info().
				Str("user", refund.Receiver).
				Str("refund", payment.String()).
				Msg("User refund created")
		}
	}

	// Rebalancing auction
	log.Info().Msg("Running rebalancing auction")
	rebalancingBids := auction.CollectRebalancingBids(api.pool, beliefs)
	rebalancingWinner, rebalancingPayment := auction.RunAuction(rebalancingBids)

	result.RebalancingWinner = rebalancingWinner
	result.RebalancingPayment = rebalancingPayment

	log.Info().
		Str("winner", rebalancingWinner).
		Str("payment", rebalancingPayment.String()).
		Msg("Rebalancing auction completed")

	if rebalancingPayment.GreaterThan(decimal.Zero) {
		refund := types.Refund{
			Receiver: "LP",
			Amount:   rebalancingPayment,
		}
		result.Refunds = append(result.Refunds, refund)

		log.Info().
			Str("refund", rebalancingPayment.String()).
			Msg("LP refund created")
	}

	// Clear stores for next block
	api.txStore.Clear()
	api.beliefStore.Clear()

	totalRefund := decimal.Zero
	for _, r := range result.Refunds {
		totalRefund = totalRefund.Add(r.Amount)
	}

	log.Info().
		Int("bundles", len(result.Bundles)).
		Int("auctions", len(result.Auctions)).
		Int("refunds", len(result.Refunds)).
		Str("total_refund", totalRefund.String()).
		Msg("Block processing complete")

	return result, nil
}

func writeSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := types.JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := types.JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &types.RPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON error response")
	}
}
