package builder

import (
	"encoding/json"
	"math/big"
	"net/http"
)

type HTTPHandler struct {
	registry *Registry
}

type ObserveBuilderRequest struct {
	Builder           string `json:"builder"`
	DispatchAttempts  uint64 `json:"dispatchAttempts,omitempty"`
	DispatchSuccesses uint64 `json:"dispatchSuccesses,omitempty"`
	SandwichAttacks   uint64 `json:"sandwichAttacks,omitempty"`
	WellBehavedEvents uint64 `json:"wellBehavedEvents,omitempty"`
	ValueCreatedWei   string `json:"valueCreatedWei,omitempty"`
}

type BuilderScoreView struct {
	Name      string       `json:"name"`
	URL       string       `json:"url"`
	BaseScore float64      `json:"baseScore"`
	Score     float64      `json:"score"`
	Stats     BuilderStats `json:"stats"`
}

func NewHTTPHandler(registry *Registry) *HTTPHandler {
	return &HTTPHandler{registry: registry}
}

func (h *HTTPHandler) GetScores(w http.ResponseWriter, r *http.Request) {
	views := make([]BuilderScoreView, 0)
	for _, builder := range h.registry.All() {
		views = append(views, BuilderScoreView{
			Name:      builder.Name,
			URL:       builder.URL,
			BaseScore: builder.BaseScore,
			Score:     builder.Score,
			Stats:     builder.Stats,
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
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, BuilderScoreView{
		Name:      builder.Name,
		URL:       builder.URL,
		BaseScore: builder.BaseScore,
		Score:     builder.Score,
		Stats:     builder.Stats,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
