package auction

import (
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

// BuildBundle constructs a concrete sandwich bundle for the winning arbitrager.
//
// Sandwich structure:
//  1. Front-run: arb moves pool price to the user's limit price boundary
//     (maximizes MEV extraction without violating user's min-output constraint)
//  2. User tx: executed at the (now worse) pool price, still satisfying their limit
//  3. Back-run: arb moves pool back to external market price (no-arbitrage state)
//
// Pool simulation is done on a copy; the real pool state is not modified here.
func BuildBundle(p *pool.Pool, tx *types.SwapTransaction, winnerID string, belief decimal.Decimal, payment decimal.Decimal) types.Bundle {
	sim := p.Copy()

	bundle := types.Bundle{
		TxID:    tx.ID,
		ArbID:   winnerID,
		Payment: payment,
	}

	// --- Front-run ---
	// The arb pushes the pool to the user's limit price so the user gets exactly
	// their minimum output (worst acceptable execution). This extracts maximum MEV.
	//
	// User limit price (Y/X):
	//   X→Y: user pays Input X, wants at least Output Y → limit price = Output/Input
	//   Y→X: user pays Input Y, wants at least Output X → limit price = Input/Output
	var limitPrice decimal.Decimal
	if tx.Direction == types.DirectionXToY {
		// limit price = min Y per X = Output / Input
		limitPrice = tx.Output.Div(tx.Input)
	} else {
		// limit price = max Y per X the user is willing to give = Input / Output
		limitPrice = tx.Input.Div(tx.Output)
	}

	frontDir, frontAmountIn := sim.AmountToReachPrice(limitPrice)
	if frontAmountIn.GreaterThan(decimal.Zero) {
		var frontAmountOut decimal.Decimal
		if frontDir == "X->Y" {
			frontAmountOut = sim.SwapXForY(frontAmountIn)
			bundle.FrontRun = &types.SwapOp{
				Direction: types.DirectionXToY,
				AmountIn:  frontAmountIn,
				AmountOut: frontAmountOut,
			}
		} else if frontDir == "Y->X" {
			frontAmountOut = sim.SwapYForX(frontAmountIn)
			bundle.FrontRun = &types.SwapOp{
				Direction: types.DirectionYToX,
				AmountIn:  frontAmountIn,
				AmountOut: frontAmountOut,
			}
		}
	}

	// --- User tx ---
	var userAmountOut decimal.Decimal
	if tx.Direction == types.DirectionXToY {
		userAmountOut = sim.SwapXForY(tx.Input)
		bundle.UserTx = types.SwapOp{
			Direction: types.DirectionXToY,
			AmountIn:  tx.Input,
			AmountOut: userAmountOut,
		}
	} else {
		userAmountOut = sim.SwapYForX(tx.Input)
		bundle.UserTx = types.SwapOp{
			Direction: types.DirectionYToX,
			AmountIn:  tx.Input,
			AmountOut: userAmountOut,
		}
	}

	// --- Back-run ---
	// Arb pushes pool back to external market price (no-arbitrage state).
	backDir, backAmountIn := sim.AmountToReachPrice(belief)
	if backAmountIn.GreaterThan(decimal.Zero) {
		var backAmountOut decimal.Decimal
		if backDir == "X->Y" {
			backAmountOut = sim.SwapXForY(backAmountIn)
			bundle.BackRun = &types.SwapOp{
				Direction: types.DirectionXToY,
				AmountIn:  backAmountIn,
				AmountOut: backAmountOut,
			}
		} else if backDir == "Y->X" {
			backAmountOut = sim.SwapYForX(backAmountIn)
			bundle.BackRun = &types.SwapOp{
				Direction: types.DirectionYToX,
				AmountIn:  backAmountIn,
				AmountOut: backAmountOut,
			}
		}
	}

	// --- Arb profit calculation ---
	// Profit = value of tokens received - value of tokens spent (denominated in Y)
	// Front-run: arb spends frontAmountIn, receives frontAmountOut
	// Back-run:  arb spends backAmountIn, receives backAmountOut
	// All values converted to Y using belief (1X = belief Y)
	profit := decimal.Zero

	if bundle.FrontRun != nil {
		fr := bundle.FrontRun
		if fr.Direction == types.DirectionXToY {
			// spent X, received Y: profit += amountOut - amountIn*belief
			profit = profit.Add(fr.AmountOut.Sub(fr.AmountIn.Mul(belief)))
		} else {
			// spent Y, received X: profit += amountOut*belief - amountIn
			profit = profit.Add(fr.AmountOut.Mul(belief).Sub(fr.AmountIn))
		}
	}

	if bundle.BackRun != nil {
		br := bundle.BackRun
		if br.Direction == types.DirectionXToY {
			profit = profit.Add(br.AmountOut.Sub(br.AmountIn.Mul(belief)))
		} else {
			profit = profit.Add(br.AmountOut.Mul(belief).Sub(br.AmountIn))
		}
	}

	bundle.ArbProfit = profit
	bundle.NetProfit = profit.Sub(payment)

	return bundle
}

