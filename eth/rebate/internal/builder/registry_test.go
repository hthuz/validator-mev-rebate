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

func TestScoreUsesHundredPointScale(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 100); err != nil {
		t.Fatalf("Register: %v", err)
	}

	updated, err := registry.Observe("builder-a", BuilderObservation{
		DispatchAttempts:  10,
		DispatchSuccesses: 10,
		WellBehavedEvents: 100,
		ValueCreatedWei:   new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)),
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if updated.Score < 0.5 || updated.Score > 100 {
		t.Fatalf("expected score in (0, 100], got %f", updated.Score)
	}

	if err := registry.Register("builder-b", "http://builder-b", 100.1); err == nil {
		t.Fatal("expected score above 100 to be rejected")
	}
}

func TestHighDispatchShareGetsBoundedCompetitionPenalty(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := registry.Register(name, "http://"+name, 80); err != nil {
			t.Fatalf("Register %s: %v", name, err)
		}
	}

	for i := 0; i < 90; i++ {
		if _, err := registry.Observe("alpha", BuilderObservation{
			DispatchAttempts:  1,
			DispatchSuccesses: 1,
		}); err != nil {
			t.Fatalf("Observe alpha: %v", err)
		}
	}
	for _, name := range []string{"beta", "gamma"} {
		if _, err := registry.Observe(name, BuilderObservation{
			DispatchAttempts:  5,
			DispatchSuccesses: 5,
		}); err != nil {
			t.Fatalf("Observe %s: %v", name, err)
		}
	}

	scores := registry.All()
	if scores[0].Score >= 81 {
		t.Fatalf("expected concentrated alpha score to be bounded below 81, got %f", scores[0].Score)
	}
	if scores[0].Score < 68 {
		t.Fatalf("competition penalty should be bounded, got %f", scores[0].Score)
	}
}
