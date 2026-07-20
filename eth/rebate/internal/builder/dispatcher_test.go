package builder

import (
	"testing"
	"time"
)

func TestSelectTargetUsesExplorationForColdStartBuilder(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 3.0); err != nil {
		t.Fatalf("Register builder-a: %v", err)
	}
	if err := registry.Register("builder-b", "http://builder-b", 0.5); err != nil {
		t.Fatalf("Register builder-b: %v", err)
	}
	if _, err := registry.Observe("builder-a", BuilderObservation{
		DispatchAttempts:  10,
		DispatchSuccesses: 10,
	}); err != nil {
		t.Fatalf("Observe builder-a: %v", err)
	}

	dispatcher := NewDispatcher(registry, StrategyConfig{
		ExplorationEnabled:     true,
		ExplorationRate:        1.0,
		MinExploreDispatches:   5,
		NewProducerGracePeriod: 0,
		UncertaintyWeight:      1.25,
		FreshProducerBonus:     0.75,
	})

	target, decision := dispatcher.selectTarget(registry.All(), time.Now())
	if decision.Layer != dispatchLayerExploration {
		t.Fatalf("expected exploration layer, got %q", decision.Layer)
	}
	if target.Name != "builder-b" {
		t.Fatalf("expected cold-start builder-b to be explored, got %q", target.Name)
	}
}

func TestExplorationCandidatesExcludeMatureBuilders(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 2.0); err != nil {
		t.Fatalf("Register: %v", err)
	}
	builder := registry.All()[0]
	builder.RegisteredAt = time.Now().Add(-2 * time.Hour)
	builder.Stats.DispatchAttempts = 12

	dispatcher := NewDispatcher(registry, StrategyConfig{
		ExplorationEnabled:     true,
		ExplorationRate:        1.0,
		MinExploreDispatches:   5,
		NewProducerGracePeriod: 10 * time.Minute,
		UncertaintyWeight:      1.25,
		FreshProducerBonus:     0.75,
	})

	if dispatcher.isExplorationCandidate(builder, time.Now()) {
		t.Fatalf("expected mature builder to be excluded from exploration")
	}
}

func TestSelectTargetFallsBackToExploitationWhenExplorationDisabled(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 2.0); err != nil {
		t.Fatalf("Register: %v", err)
	}

	dispatcher := NewDispatcher(registry, StrategyConfig{
		ExplorationEnabled: false,
	})

	target, decision := dispatcher.selectTarget(registry.All(), time.Now())
	if decision.Layer != dispatchLayerExploitation {
		t.Fatalf("expected exploitation layer, got %q", decision.Layer)
	}
	if target.Name != "builder-a" {
		t.Fatalf("expected builder-a, got %q", target.Name)
	}
}

func TestExpectedRewardUsesObservedReward(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("builder-a", "http://builder-a", 1.0); err != nil {
		t.Fatalf("Register builder-a: %v", err)
	}
	if err := registry.Register("builder-b", "http://builder-b", 1.0); err != nil {
		t.Fatalf("Register builder-b: %v", err)
	}

	positiveReward := 1.2
	negativeReward := -0.6
	if _, err := registry.Observe("builder-a", BuilderObservation{
		DispatchAttempts:  1,
		DispatchSuccesses: 1,
		Reward:            &positiveReward,
	}); err != nil {
		t.Fatalf("Observe builder-a: %v", err)
	}
	if _, err := registry.Observe("builder-b", BuilderObservation{
		DispatchAttempts:  1,
		DispatchSuccesses: 0,
		Reward:            &negativeReward,
	}); err != nil {
		t.Fatalf("Observe builder-b: %v", err)
	}

	dispatcher := NewDispatcher(registry, StrategyConfig{
		ExplorationEnabled: false,
	})

	builders := registry.All()
	var rewardA, rewardB float64
	for _, builder := range builders {
		switch builder.Name {
		case "builder-a":
			rewardA = dispatcher.expectedReward(builder)
		case "builder-b":
			rewardB = dispatcher.expectedReward(builder)
		}
	}

	if rewardA <= rewardB {
		t.Fatalf("expected builder-a expected reward to exceed builder-b, got a=%f b=%f", rewardA, rewardB)
	}
}
