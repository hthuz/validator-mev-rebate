package builder

import (
	"math/big"
	"testing"
)

func TestObserveSandwichAttacksReduceScore(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 2.0); err != nil {
		t.Fatalf("Register: %v", err)
	}

	before := registry.All()[0].Score
	updated, err := registry.Observe("builder-a", BuilderObservation{
		SandwichAttacks: 3,
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	if updated.Score >= before {
		t.Fatalf("expected sandwich attacks to reduce score, before=%f after=%f", before, updated.Score)
	}
}

func TestObserveValueAndWellBehavedIncreaseScore(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 1.5); err != nil {
		t.Fatalf("Register: %v", err)
	}

	before := registry.All()[0].Score
	updated, err := registry.Observe("builder-a", BuilderObservation{
		DispatchAttempts:  5,
		DispatchSuccesses: 5,
		WellBehavedEvents: 4,
		ValueCreatedWei:   new(big.Int).Mul(big.NewInt(3), big.NewInt(1e18)),
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	if updated.Score <= before {
		t.Fatalf("expected positive behavior to increase score, before=%f after=%f", before, updated.Score)
	}
}

func TestObserveFailuresReduceScore(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 2.0); err != nil {
		t.Fatalf("Register: %v", err)
	}

	before := registry.All()[0].Score
	updated, err := registry.Observe("builder-a", BuilderObservation{
		DispatchAttempts: 4,
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	if updated.Score >= before {
		t.Fatalf("expected failed deliveries to reduce score, before=%f after=%f", before, updated.Score)
	}
}
