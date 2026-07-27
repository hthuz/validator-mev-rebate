package experiment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultDir = "logs/experiment"

const (
	bundleEventsFile    = "bundle_events.jsonl"
	builderDispatchFile = "builder_dispatches.jsonl"
	builderSnapshotFile = "builder_snapshots.jsonl"
	blockSummaryFile    = "block_summary.jsonl"
)

type BundleSimulationEvent struct {
	RecordedAt           time.Time `json:"recorded_at"`
	BundleHash           string    `json:"bundle_hash"`
	MatchingHash         string    `json:"matching_hash,omitempty"`
	TargetBlock          uint64    `json:"target_block"`
	MaxBlock             uint64    `json:"max_block"`
	Searcher             string    `json:"searcher,omitempty"`
	RequestedBuilders    []string  `json:"requested_builders,omitempty"`
	WantRefundPercent    *int      `json:"want_refund_percent,omitempty"`
	BodyItemCount        int       `json:"body_item_count"`
	TxCount              int       `json:"tx_count"`
	NestedBundleCount    int       `json:"nested_bundle_count"`
	HashReferenceCount   int       `json:"hash_reference_count"`
	IsBackrun            bool      `json:"is_backrun"`
	SimulationSuccess    bool      `json:"simulation_success"`
	SimulationError      string    `json:"simulation_error,omitempty"`
	ExecutionError       string    `json:"execution_error,omitempty"`
	GasUsed              uint64    `json:"gas_used"`
	ProfitWei            string    `json:"profit_wei"`
	RefundableWei        string    `json:"refundable_wei"`
	MevGasPriceWei       string    `json:"mev_gas_price_wei"`
	SimulatedBlockNumber uint64    `json:"simulated_block_number,omitempty"`
	HistoricalTxCount    uint64    `json:"historical_tx_count,omitempty"`
	BundleInsertionIndex uint64    `json:"bundle_insertion_index,omitempty"`
	DisplacedTxCount     int       `json:"displaced_tx_count"`
}

type BuilderDispatchEvent struct {
	RecordedAt            time.Time `json:"recorded_at"`
	BundleHash            string    `json:"bundle_hash"`
	TargetBlock           uint64    `json:"target_block"`
	Builder               string    `json:"builder"`
	Layer                 string    `json:"layer"`
	Reason                string    `json:"reason"`
	Success               bool      `json:"success"`
	Error                 string    `json:"error,omitempty"`
	ExplorationCandidates int       `json:"exploration_candidates"`
	BuilderBaseScore      float64   `json:"builder_base_score"`
	BuilderEffectiveScore float64   `json:"builder_effective_score"`
	ExpectedReward        float64   `json:"expected_reward"`
	BanditScore           float64   `json:"bandit_score"`
	TotalScore            float64   `json:"total_score"`
	BundleProfitWei       string    `json:"bundle_profit_wei"`
	BundleRefundableWei   string    `json:"bundle_refundable_wei"`
	BundleGasUsed         uint64    `json:"bundle_gas_used"`
}

type BuilderSnapshotEvent struct {
	RecordedAt        time.Time `json:"recorded_at"`
	Source            string    `json:"source"`
	Builder           string    `json:"builder"`
	BaseScore         float64   `json:"base_score"`
	EffectiveScore    float64   `json:"effective_score"`
	DispatchAttempts  uint64    `json:"dispatch_attempts"`
	DispatchSuccesses uint64    `json:"dispatch_successes"`
	DispatchFailures  uint64    `json:"dispatch_failures"`
	SandwichAttacks   uint64    `json:"sandwich_attacks"`
	WellBehavedEvents uint64    `json:"well_behaved_events"`
	ValuableOrderFlow uint64    `json:"valuable_order_flow"`
	RewardSamples     uint64    `json:"reward_samples"`
	AverageReward     float64   `json:"average_reward"`
	LastReward        float64   `json:"last_reward"`
}

type BlockSummaryEvent struct {
	RecordedAt          time.Time      `json:"recorded_at"`
	BlockNumber         uint64         `json:"block_number"`
	Validator           string         `json:"validator"`
	BlockTimestamp      time.Time      `json:"block_timestamp"`
	BundleCount         int            `json:"bundle_count"`
	SuccessCount        int            `json:"success_count"`
	FailedCount         int            `json:"failed_count"`
	SuccessRate         float64        `json:"success_rate"`
	TotalMevProfitWei   string         `json:"total_mev_profit_wei"`
	TotalRefundableWei  string         `json:"total_refundable_wei"`
	TotalGasUsed        uint64         `json:"total_gas_used"`
	MevGasPriceWei      string         `json:"mev_gas_price_wei"`
	BlockSpaceUsed      float64        `json:"block_space_used"`
	UniqueBuilders      int            `json:"unique_builders"`
	BuilderDistribution map[string]int `json:"builder_distribution"`
}

type Recorder struct {
	baseDir string
	mu      sync.Mutex
	files   map[string]*os.File
}

func DefaultDir() string {
	if dir := strings.TrimSpace(os.Getenv("EXPERIMENT_REPORT_DIR")); dir != "" {
		return dir
	}
	return defaultDir
}

func NewRecorder(baseDir string) (*Recorder, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = DefaultDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create experiment dir: %w", err)
	}

	return &Recorder{
		baseDir: baseDir,
		files:   make(map[string]*os.File),
	}, nil
}

func (r *Recorder) BaseDir() string {
	if r == nil {
		return ""
	}
	return r.baseDir
}

func (r *Recorder) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []string
	for name, file := range r.files {
		if err := file.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	r.files = make(map[string]*os.File)
	if len(errs) > 0 {
		return fmt.Errorf("close experiment files: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *Recorder) RecordBundleSimulation(event BundleSimulationEvent) error {
	return r.appendJSONL(bundleEventsFile, event)
}

func (r *Recorder) RecordBuilderDispatch(event BuilderDispatchEvent) error {
	return r.appendJSONL(builderDispatchFile, event)
}

func (r *Recorder) RecordBuilderSnapshot(event BuilderSnapshotEvent) error {
	return r.appendJSONL(builderSnapshotFile, event)
}

func (r *Recorder) RecordBlockSummary(event BlockSummaryEvent) error {
	return r.appendJSONL(blockSummaryFile, event)
}

func (r *Recorder) appendJSONL(name string, value any) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := r.file(name)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	return nil
}

func (r *Recorder) file(name string) (*os.File, error) {
	if file, ok := r.files[name]; ok {
		return file, nil
	}

	path := filepath.Join(r.baseDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	r.files[name] = file
	return file, nil
}
