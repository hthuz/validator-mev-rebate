package config

import (
	"testing"
	"time"
)

func TestLoadStrategyConfig(t *testing.T) {
	tests := []struct {
		name string
		path string
		mode string
	}{
		{name: "default", path: "config.yaml", mode: "probabilistic"},
		{name: "replay 5 builders", path: "experiment_5builders.yaml", mode: "probabilistic"},
		{name: "mock 5 builders", path: "experiment_mock_5builders.yaml", mode: "alternating"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(tt.path)
			if err != nil {
				t.Fatalf("Load(%s): %v", tt.path, err)
			}

			strategy := cfg.Dispatcher.Strategy
			if !strategy.ExplorationEnabled {
				t.Fatalf("expected exploration to be enabled")
			}
			if strategy.ExplorationMode != tt.mode {
				t.Fatalf("expected exploration mode %q, got %q", tt.mode, strategy.ExplorationMode)
			}
			if strategy.ExplorationRate != 0.20 {
				t.Fatalf("expected exploration rate 0.20, got %f", strategy.ExplorationRate)
			}
			if strategy.NewProducerGracePeriod != 600*time.Second {
				t.Fatalf("expected new producer grace period 600s, got %s", strategy.NewProducerGracePeriod)
			}
		})
	}
}
