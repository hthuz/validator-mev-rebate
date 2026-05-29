package pool

import (
	"math"

	"github.com/shopspring/decimal"
)

// Pool represents a constant product AMM pool (x*y=k)
type Pool struct {
	ReserveX decimal.Decimal
	ReserveY decimal.Decimal
}

// NewPool creates a new constant product pool
func NewPool(x, y decimal.Decimal) *Pool {
	return &Pool{
		ReserveX: x,
		ReserveY: y,
	}
}

// K returns the constant product k = x * y
func (p *Pool) K() decimal.Decimal {
	return p.ReserveX.Mul(p.ReserveY)
}

// SpotPrice returns the current spot price (Y/X)
func (p *Pool) SpotPrice() decimal.Decimal {
	if p.ReserveX.IsZero() {
		return decimal.Zero
	}
	return p.ReserveY.Div(p.ReserveX)
}

// Copy creates a deep copy of the pool
func (p *Pool) Copy() *Pool {
	return &Pool{
		ReserveX: p.ReserveX,
		ReserveY: p.ReserveY,
	}
}

// SwapXForY swaps X tokens for Y tokens
func (p *Pool) SwapXForY(amountX decimal.Decimal) decimal.Decimal {
	k := p.K()
	newX := p.ReserveX.Add(amountX)
	newY := k.Div(newX)
	amountY := p.ReserveY.Sub(newY)

	p.ReserveX = newX
	p.ReserveY = newY

	return amountY
}

// SwapYForX swaps Y tokens for X tokens
func (p *Pool) SwapYForX(amountY decimal.Decimal) decimal.Decimal {
	k := p.K()
	newY := p.ReserveY.Add(amountY)
	newX := k.Div(newY)
	amountX := p.ReserveX.Sub(newX)

	p.ReserveX = newX
	p.ReserveY = newY

	return amountX
}

// Sqrt computes the square root of a decimal number
func Sqrt(d decimal.Decimal) decimal.Decimal {
	f, _ := d.Float64()
	return decimal.NewFromFloat(math.Sqrt(f))
}

// AmountToReachPrice computes the input amount needed to move the pool
// to the target spot price (Y/X = targetPrice).
// Returns (direction, amountIn). If pool is already at target, amountIn = 0.
func (p *Pool) AmountToReachPrice(targetPrice decimal.Decimal) (string, decimal.Decimal) {
	currentPrice := p.SpotPrice() // Y/X

	if currentPrice.Equal(targetPrice) {
		return "", decimal.Zero
	}

	k := p.K()

	if currentPrice.LessThan(targetPrice) {
		// Price too low (Y/X < target) → need to increase Y/X → swap Y in for X out
		// At target: newY = sqrt(k * targetPrice), deltaY = newY - currentY
		newY := Sqrt(k.Mul(targetPrice))
		deltaY := newY.Sub(p.ReserveY)
		if deltaY.LessThanOrEqual(decimal.Zero) {
			return "", decimal.Zero
		}
		return "Y->X", deltaY
	}

	// Price too high (Y/X > target) → need to decrease Y/X → swap X in for Y out
	// At target: newX = sqrt(k / targetPrice), deltaX = newX - currentX
	newX := Sqrt(k.Div(targetPrice))
	deltaX := newX.Sub(p.ReserveX)
	if deltaX.LessThanOrEqual(decimal.Zero) {
		return "", decimal.Zero
	}
	return "X->Y", deltaX
}
