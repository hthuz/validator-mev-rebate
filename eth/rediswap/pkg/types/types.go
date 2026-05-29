package types

import "github.com/shopspring/decimal"

// Direction represents swap direction
type Direction string

const (
	DirectionXToY Direction = "X->Y"
	DirectionYToX Direction = "Y->X"
)

// SwapTransaction represents a user swap transaction
type SwapTransaction struct {
	ID        string          `json:"id"`
	Direction Direction       `json:"direction"`
	Input     decimal.Decimal `json:"input"`  // max input willing to pay
	Output    decimal.Decimal `json:"output"` // min output willing to accept
}

// BeliefReport represents an arbitrager's price belief
type BeliefReport struct {
	ArbID  string          `json:"arb_id"`
	Belief decimal.Decimal `json:"belief"` // external price: 1X = belief*Y
}

// AuctionResult represents the result of an auction
type AuctionResult struct {
	TxID    string          `json:"tx_id"`
	Winner  string          `json:"winner"`
	Payment decimal.Decimal `json:"payment"`
}

// SwapOp represents a single swap operation in a bundle
type SwapOp struct {
	Direction Direction       `json:"direction"`
	AmountIn  decimal.Decimal `json:"amount_in"`
	AmountOut decimal.Decimal `json:"amount_out"` // actual output from pool simulation
}

// Bundle represents a sandwich bundle with concrete swap operations
type Bundle struct {
	TxID        string          `json:"tx_id"`
	ArbID       string          `json:"arb_id"`
	FrontRun    *SwapOp         `json:"front_run,omitempty"` // nil if no front-run needed
	UserTx      SwapOp          `json:"user_tx"`
	BackRun     *SwapOp         `json:"back_run,omitempty"` // nil if no back-run needed
	ArbProfit   decimal.Decimal `json:"arb_profit"`         // gross profit before payment
	Payment     decimal.Decimal `json:"payment"`            // second-price payment
	NetProfit   decimal.Decimal `json:"net_profit"`         // arb_profit - payment
}

// Refund represents a payment refund
type Refund struct {
	Receiver string          `json:"receiver"`
	Amount   decimal.Decimal `json:"amount"`
}

// BlockResult represents the final result for a block
type BlockResult struct {
	BlockNumber        uint64          `json:"block_number"`
	Bundles            []Bundle        `json:"bundles"`
	Auctions           []AuctionResult `json:"auctions"`
	Refunds            []Refund        `json:"refunds"`
	RebalancingWinner  string          `json:"rebalancing_winner"`
	RebalancingPayment decimal.Decimal `json:"rebalancing_payment"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      interface{}   `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SendSwapArgs represents arguments for rediswap_sendSwap
type SendSwapArgs struct {
	Direction string  `json:"direction"` // "X->Y" or "Y->X"
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
}

// SendBeliefArgs represents arguments for rediswap_sendBelief
type SendBeliefArgs struct {
	ArbID  string  `json:"arb_id"`
	Belief float64 `json:"belief"`
}
