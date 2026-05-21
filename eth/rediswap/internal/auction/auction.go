package auction

import (
	"math"
	"sort"

	"rediswap/internal/pool"
	"rediswap/pkg/types"

	"github.com/shopspring/decimal"
)

// Bid represents an arbitrager's bid
type Bid struct {
	ArbID string
	TxID  string
	Value decimal.Decimal
}

// ComputeMEV computes the MEV value for a transaction given an arbitrager's belief
// Based on paper formula: Φ = Δx · q + Δy
func ComputeMEV(p *pool.Pool, tx *types.SwapTransaction, belief decimal.Decimal) decimal.Decimal {
	var deltaX, deltaY decimal.Decimal

	if tx.Direction == types.DirectionXToY {
		// X→Y: user pays δ_X^in, receives δ_Y^out
		// Net impact: (Δx, Δy) = (δ_X^in, -δ_Y^out)
		deltaX = tx.Input
		deltaY = tx.Output.Neg()

		// Φ = δ_X^in · q - δ_Y^out
		phi := deltaX.Mul(belief).Add(deltaY)

		if phi.GreaterThan(decimal.Zero) {
			return phi
		}
		return decimal.Zero

	} else {
		// Y→X: user pays δ_Y^in, receives δ_X^out
		// Net impact: (Δx, Δy) = (-δ_X^out, δ_Y^in)
		deltaX = tx.Output.Neg()
		deltaY = tx.Input

		// Φ = -δ_X^out · q + δ_Y^in
		phi := deltaX.Mul(belief).Add(deltaY)

		if phi.GreaterThan(decimal.Zero) {
			return phi
		}
		return decimal.Zero
	}
}

// ComputeRebalancingMEV computes the MEV from rebalancing the pool
// Formula: φ(s, q_i) = (x · q_i + y) - (x̂_i · q_i + ŷ_i)
func ComputeRebalancingMEV(p *pool.Pool, belief decimal.Decimal) decimal.Decimal {
	k := p.K()

	// No-arbitrage state: y/x = belief and x*y = k
	// Solving: x̂ = sqrt(k/belief), ŷ = sqrt(k*belief)
	xHat := pool.Sqrt(k.Div(belief))
	yHat := pool.Sqrt(k.Mul(belief))

	// φ = (x · q + y) - (x̂ · q + ŷ)
	currentValue := p.ReserveX.Mul(belief).Add(p.ReserveY)
	noArbValue := xHat.Mul(belief).Add(yHat)

	phi := currentValue.Sub(noArbValue)

	if phi.GreaterThan(decimal.Zero) {
		return phi
	}
	return decimal.Zero
}

// RunAuction runs a second-price auction
// Returns the winner ID and the payment (second highest bid)
func RunAuction(bids []Bid) (winner string, payment decimal.Decimal) {
	if len(bids) == 0 {
		return "", decimal.Zero
	}

	// Sort bids by value in descending order
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Value.GreaterThan(bids[j].Value)
	})

	// Winner is the highest bidder
	winner = bids[0].ArbID

	// Payment is the second highest bid (second-price auction)
	if len(bids) > 1 {
		payment = bids[1].Value
	} else {
		payment = decimal.Zero
	}

	return winner, payment
}

// CollectBids collects all bids from arbitragers for a transaction
func CollectBids(p *pool.Pool, tx *types.SwapTransaction, beliefs map[string]decimal.Decimal) []Bid {
	var bids []Bid

	for arbID, belief := range beliefs {
		mev := ComputeMEV(p, tx, belief)

		if mev.GreaterThan(decimal.Zero) {
			bids = append(bids, Bid{
				ArbID: arbID,
				TxID:  tx.ID,
				Value: mev,
			})
		}
	}

	return bids
}

// CollectRebalancingBids collects bids for the rebalancing auction
func CollectRebalancingBids(p *pool.Pool, beliefs map[string]decimal.Decimal) []Bid {
	var bids []Bid

	for arbID, belief := range beliefs {
		mev := ComputeRebalancingMEV(p, belief)

		if mev.GreaterThan(decimal.Zero) {
			bids = append(bids, Bid{
				ArbID: arbID,
				TxID:  "rebalancing",
				Value: mev,
			})
		}
	}

	return bids
}

// Sqrt helper (re-export from pool package)
func Sqrt(d decimal.Decimal) decimal.Decimal {
	f, _ := d.Float64()
	return decimal.NewFromFloat(math.Sqrt(f))
}
