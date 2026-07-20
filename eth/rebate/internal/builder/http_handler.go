package builder

import (
	"encoding/json"
	"math/big"
	"net/http"
	"rebate/mylog"
	"time"
)

type HTTPHandler struct {
	registry *Registry
	strategy StrategyConfig
}

type ObserveBuilderRequest struct {
	Builder           string   `json:"builder"`
	DispatchAttempts  uint64   `json:"dispatchAttempts,omitempty"`
	DispatchSuccesses uint64   `json:"dispatchSuccesses,omitempty"`
	SandwichAttacks   uint64   `json:"sandwichAttacks,omitempty"`
	WellBehavedEvents uint64   `json:"wellBehavedEvents,omitempty"`
	ValueCreatedWei   string   `json:"valueCreatedWei,omitempty"`
	Reward            *float64 `json:"reward,omitempty"`
}

type BuilderScoreView struct {
	Name                 string       `json:"name"`
	URL                  string       `json:"url"`
	BaseScore            float64      `json:"baseScore"`
	Score                float64      `json:"score"`
	Stats                BuilderStats `json:"stats"`
	RegisteredAt         string       `json:"registeredAt"`
	ExplorationCandidate bool         `json:"explorationCandidate"`
}

func NewHTTPHandler(registry *Registry, strategy StrategyConfig) *HTTPHandler {
	return &HTTPHandler{registry: registry, strategy: normalizeStrategyConfig(strategy)}
}

func (h *HTTPHandler) GetScores(w http.ResponseWriter, r *http.Request) {
	views := make([]BuilderScoreView, 0)
	for _, builder := range h.registry.All() {
		views = append(views, BuilderScoreView{
			Name:                 builder.Name,
			URL:                  builder.URL,
			BaseScore:            builder.BaseScore,
			Score:                builder.Score,
			Stats:                builder.Stats,
			RegisteredAt:         builder.RegisteredAt.Format(time.RFC3339),
			ExplorationCandidate: h.isExplorationCandidate(builder),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"builders":   views,
		"totalScore": h.registry.TotalScore(),
	})
}

func (h *HTTPHandler) ObserveBuilder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ObserveBuilderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.Builder == "" {
		http.Error(w, "builder is required", http.StatusBadRequest)
		return
	}

	var valueCreated *big.Int
	if req.ValueCreatedWei != "" {
		valueCreated = new(big.Int)
		if _, ok := valueCreated.SetString(req.ValueCreatedWei, 10); !ok {
			http.Error(w, "valueCreatedWei must be a base-10 integer string", http.StatusBadRequest)
			return
		}
	}

	builder, err := h.registry.Observe(req.Builder, BuilderObservation{
		DispatchAttempts:  req.DispatchAttempts,
		DispatchSuccesses: req.DispatchSuccesses,
		SandwichAttacks:   req.SandwichAttacks,
		WellBehavedEvents: req.WellBehavedEvents,
		ValueCreatedWei:   valueCreated,
		Reward:            req.Reward,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mylog.BuilderLogger.Info().
		Str("event", "builder_observation_recorded").
		Str("builder", builder.Name).
		Uint64("dispatch_attempts", req.DispatchAttempts).
		Uint64("dispatch_successes", req.DispatchSuccesses).
		Uint64("sandwich_attacks", req.SandwichAttacks).
		Uint64("well_behaved_events", req.WellBehavedEvents).
		Str("value_created_wei", req.ValueCreatedWei).
		Float64("average_reward", builder.Stats.AverageReward).
		Float64("last_reward", builder.Stats.LastReward).
		Float64("effective_score", builder.Score).
		Msg("builder observation recorded")

	writeJSON(w, http.StatusOK, BuilderScoreView{
		Name:                 builder.Name,
		URL:                  builder.URL,
		BaseScore:            builder.BaseScore,
		Score:                builder.Score,
		Stats:                builder.Stats,
		RegisteredAt:         builder.RegisteredAt.Format(time.RFC3339),
		ExplorationCandidate: h.isExplorationCandidate(builder),
	})
}

func (h *HTTPHandler) isExplorationCandidate(builder *BuilderInfo) bool {
	if builder == nil || !h.strategy.ExplorationEnabled {
		return false
	}
	if builder.Stats.DispatchAttempts < h.strategy.MinExploreDispatches {
		return true
	}
	if h.strategy.NewProducerGracePeriod > 0 && time.Since(builder.RegisteredAt) < h.strategy.NewProducerGracePeriod {
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
