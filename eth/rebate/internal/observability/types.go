package observability

import (
	"math/big"
	"rebate/pkg/types"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// BlockMevMetrics stores per-block MEV metrics.
type BlockMevMetrics struct {
	BlockNumber      uint64         `json:"blockNumber"`
	ValidatorAddress common.Address `json:"validatorAddress"`
	Timestamp        time.Time      `json:"timestamp"`

	TotalMevProfit  *big.Int `json:"totalMevProfit"`
	TotalRefundable *big.Int `json:"totalRefundable"`
	TotalGasUsed    uint64   `json:"totalGasUsed"`

	MevGasPrice    *big.Int `json:"mevGasPrice"`
	BlockSpaceUsed float64  `json:"blockSpaceUsed"`

	BundleCount  int `json:"bundleCount"`
	SuccessCount int `json:"successCount"`
	FailedCount  int `json:"failedCount"`

	BuilderDistribution map[string]int `json:"builderDistribution"`
}

// BlockMevSummary stores compact validator block history.
type BlockMevSummary struct {
	BlockNumber uint64   `json:"blockNumber"`
	MevProfit   *big.Int `json:"mevProfit"`
	BundleCount int      `json:"bundleCount"`
	Timestamp   int64    `json:"timestamp"`
}

// ValidatorMetrics stores validator-level MEV metrics.
type ValidatorMetrics struct {
	Address        common.Address `json:"address"`
	FirstSeenBlock uint64         `json:"firstSeenBlock"`
	LastSeenBlock  uint64         `json:"lastSeenBlock"`
	UpdatedAt      time.Time      `json:"updatedAt"`

	TotalBlocks uint64 `json:"totalBlocks"`
	MevBlocks   uint64 `json:"mevBlocks"`

	TotalMevRevenue   *big.Int `json:"totalMevRevenue"`
	TotalRefunded     *big.Int `json:"totalRefunded"`
	AvgMevPerBlock    *big.Int `json:"avgMevPerBlock"`
	AvgMevPerMevBlock *big.Int `json:"avgMevPerMevBlock"`

	MevCaptureRate    float64 `json:"mevCaptureRate"`
	ParticipationRate float64 `json:"participationRate"`

	TotalBundles   uint64  `json:"totalBundles"`
	SuccessBundles uint64  `json:"successBundles"`
	SuccessRate    float64 `json:"successRate"`

	RecentBlocks []BlockMevSummary `json:"recentBlocks,omitempty"`
}

// SearcherMetrics stores searcher-level MEV metrics.
type SearcherMetrics struct {
	Address        common.Address `json:"address"`
	TotalBundles   uint64         `json:"totalBundles"`
	SuccessBundles uint64         `json:"successBundles"`
	FailedBundles  uint64         `json:"failedBundles"`
	SuccessRate    float64        `json:"successRate"`

	TotalProfit    *big.Int `json:"totalProfit"`
	AvgProfit      *big.Int `json:"avgProfit"`
	BackrunCount   uint64   `json:"backrunCount"`
	BackrunSuccess uint64   `json:"backrunSuccess"`
	BackrunProfit  *big.Int `json:"backrunProfit"`

	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// GlobalMetrics stores global aggregate MEV metrics.
type GlobalMetrics struct {
	TotalBlocks      uint64   `json:"totalBlocks"`
	TotalBundles     uint64   `json:"totalBundles"`
	TotalMevProfit   *big.Int `json:"totalMevProfit"`
	TotalRefunded    *big.Int `json:"totalRefunded"`
	UniqueValidators uint64   `json:"uniqueValidators"`
	UniqueSearchers  uint64   `json:"uniqueSearchers"`

	StartTime time.Time `json:"startTime"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BundleSimulationEvent is the append-only experiment event for bundle simulation.
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

// BuilderDispatchEvent is the append-only experiment event for builder dispatch.
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

// BuilderSnapshotEvent is the append-only experiment event for builder state snapshots.
type BuilderSnapshotEvent struct {
	RecordedAt        time.Time `json:"recorded_at"`
	BlockNumber       uint64    `json:"block_number,omitempty"`
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

// BlockSummaryEvent is the append-only experiment event for finalized block metrics.
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

func newBlockMevMetrics(blockNumber uint64, validator common.Address) *BlockMevMetrics {
	return &BlockMevMetrics{
		BlockNumber:         blockNumber,
		ValidatorAddress:    validator,
		Timestamp:           time.Now(),
		TotalMevProfit:      big.NewInt(0),
		TotalRefundable:     big.NewInt(0),
		MevGasPrice:         big.NewInt(0),
		BuilderDistribution: make(map[string]int),
	}
}

func (b *BlockMevMetrics) addBundleResult(result *types.SimMevBundleResponse, builder string) {
	b.BundleCount++

	if result.Success {
		b.SuccessCount++
		profit := result.Profit.ToInt()
		refundable := result.RefundableValue.ToInt()
		gasUsed := uint64(result.GasUsed)

		b.TotalMevProfit.Add(b.TotalMevProfit, profit)
		b.TotalRefundable.Add(b.TotalRefundable, refundable)
		b.TotalGasUsed += gasUsed

		if b.TotalGasUsed > 0 && gasUsed > 0 {
			mevGasPrice := new(big.Int).Div(profit, big.NewInt(int64(gasUsed)))
			b.MevGasPrice = weightedAverage(b.MevGasPrice, mevGasPrice, int64(b.TotalGasUsed-gasUsed), int64(gasUsed))
		}
	} else {
		b.FailedCount++
	}

	if builder != "" {
		b.BuilderDistribution[builder]++
	}
}

func (b *BlockMevMetrics) finalize(blockGasLimit uint64) {
	if blockGasLimit > 0 {
		b.BlockSpaceUsed = float64(b.TotalGasUsed) / float64(blockGasLimit)
	}
}

func newValidatorMetrics(address common.Address, firstBlock uint64) *ValidatorMetrics {
	return &ValidatorMetrics{
		Address:         address,
		FirstSeenBlock:  firstBlock,
		LastSeenBlock:   firstBlock,
		UpdatedAt:       time.Now(),
		TotalMevRevenue: big.NewInt(0),
		TotalRefunded:   big.NewInt(0),
		AvgMevPerBlock:  big.NewInt(0),
		RecentBlocks:    make([]BlockMevSummary, 0, 100),
	}
}

func (v *ValidatorMetrics) updateWithBlock(block *BlockMevMetrics) {
	v.LastSeenBlock = block.BlockNumber
	v.UpdatedAt = time.Now()
	v.TotalBlocks++

	if block.BundleCount > 0 {
		v.MevBlocks++
		v.TotalMevRevenue.Add(v.TotalMevRevenue, block.TotalMevProfit)
		v.TotalRefunded.Add(v.TotalRefunded, block.TotalRefundable)
		v.TotalBundles += uint64(block.BundleCount)
		v.SuccessBundles += uint64(block.SuccessCount)
	}

	if v.TotalBlocks > 0 {
		v.AvgMevPerBlock = new(big.Int).Div(v.TotalMevRevenue, big.NewInt(int64(v.TotalBlocks)))
	}
	if v.MevBlocks > 0 {
		v.AvgMevPerMevBlock = new(big.Int).Div(v.TotalMevRevenue, big.NewInt(int64(v.MevBlocks)))
	}
	if v.TotalBundles > 0 {
		v.SuccessRate = float64(v.SuccessBundles) / float64(v.TotalBundles)
	}
	v.ParticipationRate = float64(v.MevBlocks) / float64(v.TotalBlocks)

	summary := BlockMevSummary{
		BlockNumber: block.BlockNumber,
		MevProfit:   new(big.Int).Set(block.TotalMevProfit),
		BundleCount: block.BundleCount,
		Timestamp:   block.Timestamp.Unix(),
	}
	v.RecentBlocks = append(v.RecentBlocks, summary)
	if len(v.RecentBlocks) > 100 {
		v.RecentBlocks = v.RecentBlocks[1:]
	}
}
