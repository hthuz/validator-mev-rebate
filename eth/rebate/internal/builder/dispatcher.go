package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"rebate/mylog"
	"rebate/pkg/types"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const (
	dispatchLayerExploration  = "exploration"
	dispatchLayerExploitation = "exploitation"
)

type StrategyConfig struct {
	ExplorationEnabled     bool
	ExplorationRate        float64
	MinExploreDispatches   uint64
	NewProducerGracePeriod time.Duration
	UncertaintyWeight      float64
	FreshProducerBonus     float64
}

type DispatchDecision struct {
	Layer                 string
	Reason                string
	ExplorationCandidates int
	ExpectedReward        float64
	BanditScore           float64
}

// DispatchRecord 记录一次 bundle 分发
type DispatchRecord struct {
	BundleHash  common.Hash
	BuilderName string
	Layer       string
	SentAt      time.Time
	Success     bool
	Error       string
}

// DispatchLog 线程安全的分发日志
type DispatchLog struct {
	mu      sync.RWMutex
	records []DispatchRecord
}

func newDispatchLog() *DispatchLog {
	return &DispatchLog{}
}

func (l *DispatchLog) append(r DispatchRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
}

// All 返回所有分发记录的快照
func (l *DispatchLog) All() []DispatchRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]DispatchRecord, len(l.records))
	copy(out, l.records)
	return out
}

// ByBuilder 返回指定 builder 的分发记录
func (l *DispatchLog) ByBuilder(name string) []DispatchRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []DispatchRecord
	for _, r := range l.records {
		if r.BuilderName == name {
			out = append(out, r)
		}
	}
	return out
}

// Dispatcher 按 score 加权将 bundle 分发给 builder
type Dispatcher struct {
	registry *Registry
	log      *DispatchLog
	rng      *rand.Rand
	strategy StrategyConfig
	mu       sync.Mutex // 保护 rng
}

// NewDispatcher 创建分发器
func NewDispatcher(registry *Registry, strategy StrategyConfig) *Dispatcher {
	return &Dispatcher{
		registry: registry,
		log:      newDispatchLog(),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		strategy: normalizeStrategyConfig(strategy),
	}
}

// Log 返回分发日志，供外部查询
func (d *Dispatcher) Log() *DispatchLog {
	return d.log
}

// Dispatch 将 bundle 按动态 score 加权分发给一个 builder。
// 如果 bundle.Privacy.Builders 非空，则只在该列表内的 builder 中选择。
func (d *Dispatcher) Dispatch(ctx context.Context, bundle *types.SendMevBundleArgs, result *types.SimMevBundleResponse) error {
	candidates := d.registry.All()
	if len(candidates) == 0 {
		return fmt.Errorf("no builders registered")
	}

	// 如果 bundle 指定了目标 builder，过滤候选列表
	if bundle.Privacy != nil && len(bundle.Privacy.Builders) > 0 {
		allowed := make(map[string]bool, len(bundle.Privacy.Builders))
		for _, name := range bundle.Privacy.Builders {
			allowed[name] = true
		}
		var filtered []*BuilderInfo
		for _, b := range candidates {
			if allowed[b.Name] {
				filtered = append(filtered, b)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
		// 若过滤后为空（指定的 builder 均未注册），回退到全部候选
	}

	target, decision := d.selectTarget(candidates, time.Now())
	bundleHash := bundle.Metadata.BundleHash

	mylog.Logger.Info().
		Str("bundleHash", bundleHash.Hex()).
		Str("builder", target.Name).
		Str("layer", decision.Layer).
		Str("reason", decision.Reason).
		Str("url", target.URL).
		Float64("baseScore", target.BaseScore).
		Float64("score", target.Score).
		Float64("expectedReward", decision.ExpectedReward).
		Float64("banditScore", decision.BanditScore).
		Float64("totalScore", d.registry.TotalScore()).
		Msg("Dispatching bundle to builder")
	mylog.BuilderLogger.Info().
		Str("event", "builder_dispatch_selected").
		Str("bundle_hash", bundleHash.Hex()).
		Str("builder", target.Name).
		Str("layer", decision.Layer).
		Str("reason", decision.Reason).
		Str("url", target.URL).
		Float64("base_score", target.BaseScore).
		Float64("effective_score", target.Score).
		Float64("expected_reward", decision.ExpectedReward).
		Float64("bandit_score", decision.BanditScore).
		Float64("total_score", d.registry.TotalScore()).
		Int("exploration_candidates", decision.ExplorationCandidates).
		Msg("builder dispatch selected")

	err := d.send(ctx, target, bundle)

	rec := DispatchRecord{
		BundleHash:  bundleHash,
		BuilderName: target.Name,
		Layer:       decision.Layer,
		SentAt:      time.Now(),
		Success:     err == nil,
	}
	if err != nil {
		rec.Error = err.Error()
		mylog.Logger.Warn().
			Err(err).
			Str("bundleHash", bundleHash.Hex()).
			Str("builder", target.Name).
			Msg("Bundle dispatch failed")
		mylog.BuilderLogger.Warn().
			Str("event", "builder_dispatch_result").
			Str("bundle_hash", bundleHash.Hex()).
			Str("builder", target.Name).
			Str("layer", decision.Layer).
			Bool("success", false).
			Str("error", err.Error()).
			Msg("builder dispatch failed")
	} else {
		mylog.Logger.Info().
			Str("bundleHash", bundleHash.Hex()).
			Str("builder", target.Name).
			Msg("Bundle dispatched successfully")
		mylog.BuilderLogger.Info().
			Str("event", "builder_dispatch_result").
			Str("bundle_hash", bundleHash.Hex()).
			Str("builder", target.Name).
			Str("layer", decision.Layer).
			Bool("success", true).
			Msg("builder dispatch succeeded")
	}

	observation := BuilderObservation{
		DispatchAttempts: 1,
	}
	if err == nil {
		observation.DispatchSuccesses = 1
		if valueCreated := extractValueCreated(result); valueCreated.Sign() > 0 {
			observation.ValueCreatedWei = valueCreated
		}
	}
	updatedBuilder, observeErr := d.registry.Observe(target.Name, observation)
	if observeErr != nil {
		mylog.Logger.Warn().Err(observeErr).Str("builder", target.Name).Msg("Failed to update builder dynamic score")
	} else {
		mylog.Logger.Info().
			Str("builder", updatedBuilder.Name).
			Float64("baseScore", updatedBuilder.BaseScore).
			Float64("effectiveScore", updatedBuilder.Score).
			Uint64("attempts", updatedBuilder.Stats.DispatchAttempts).
			Uint64("successes", updatedBuilder.Stats.DispatchSuccesses).
			Uint64("sandwichAttacks", updatedBuilder.Stats.SandwichAttacks).
			Uint64("wellBehavedEvents", updatedBuilder.Stats.WellBehavedEvents).
			Msg("Builder score updated")
		mylog.BuilderLogger.Info().
			Str("event", "builder_score_updated").
			Str("builder", updatedBuilder.Name).
			Float64("base_score", updatedBuilder.BaseScore).
			Float64("effective_score", updatedBuilder.Score).
			Uint64("attempts", updatedBuilder.Stats.DispatchAttempts).
			Uint64("successes", updatedBuilder.Stats.DispatchSuccesses).
			Uint64("dispatch_failures", updatedBuilder.Stats.DispatchFailures).
			Uint64("sandwich_attacks", updatedBuilder.Stats.SandwichAttacks).
			Uint64("well_behaved_events", updatedBuilder.Stats.WellBehavedEvents).
			Uint64("valuable_order_flow", updatedBuilder.Stats.ValuableOrderFlow).
			Uint64("reward_samples", updatedBuilder.Stats.RewardSamples).
			Float64("average_reward", updatedBuilder.Stats.AverageReward).
			Float64("last_reward", updatedBuilder.Stats.LastReward).
			Msg("builder score updated")
	}
	d.log.append(rec)
	return err
}

func normalizeStrategyConfig(cfg StrategyConfig) StrategyConfig {
	if cfg.ExplorationRate < 0 {
		cfg.ExplorationRate = 0
	}
	if cfg.ExplorationRate > 1 {
		cfg.ExplorationRate = 1
	}
	if cfg.MinExploreDispatches == 0 {
		cfg.MinExploreDispatches = 5
	}
	if cfg.NewProducerGracePeriod < 0 {
		cfg.NewProducerGracePeriod = 0
	}
	if cfg.UncertaintyWeight <= 0 {
		cfg.UncertaintyWeight = 1.25
	}
	if cfg.FreshProducerBonus < 0 {
		cfg.FreshProducerBonus = 0
	}
	return cfg
}

func (d *Dispatcher) selectTarget(builders []*BuilderInfo, now time.Time) (*BuilderInfo, DispatchDecision) {
	decision := DispatchDecision{
		Layer:  dispatchLayerExploitation,
		Reason: "expected_reward",
	}
	if len(builders) == 1 {
		decision.Reason = "single_candidate"
		decision.ExpectedReward = d.expectedReward(builders[0])
		decision.BanditScore = decision.ExpectedReward
		return builders[0], decision
	}

	explorationCandidates := d.explorationCandidates(builders, now)
	decision.ExplorationCandidates = len(explorationCandidates)
	if !d.strategy.ExplorationEnabled || len(explorationCandidates) == 0 {
		target := d.weightedPick(builders, func(b *BuilderInfo) float64 { return d.expectedReward(b) })
		decision.ExpectedReward = d.expectedReward(target)
		decision.BanditScore = decision.ExpectedReward
		return target, decision
	}

	if d.nextFloat64() >= d.strategy.ExplorationRate {
		target := d.weightedPick(builders, func(b *BuilderInfo) float64 { return d.expectedReward(b) })
		decision.ExpectedReward = d.expectedReward(target)
		decision.BanditScore = decision.ExpectedReward
		return target, decision
	}

	decision.Layer = dispatchLayerExploration
	decision.Reason = "ucb_reward_plus_uncertainty"
	target := d.weightedPick(explorationCandidates, func(b *BuilderInfo) float64 {
		return d.banditScore(b, now)
	})
	decision.ExpectedReward = d.expectedReward(target)
	decision.BanditScore = d.banditScore(target, now)
	return target, decision
}

func (d *Dispatcher) explorationCandidates(builders []*BuilderInfo, now time.Time) []*BuilderInfo {
	var out []*BuilderInfo
	for _, b := range builders {
		if d.isExplorationCandidate(b, now) {
			out = append(out, b)
		}
	}
	return out
}

func (d *Dispatcher) isExplorationCandidate(builder *BuilderInfo, now time.Time) bool {
	if builder == nil {
		return false
	}
	if builder.Stats.DispatchAttempts < d.strategy.MinExploreDispatches {
		return true
	}
	if d.strategy.NewProducerGracePeriod > 0 && now.Sub(builder.RegisteredAt) < d.strategy.NewProducerGracePeriod {
		return true
	}
	return false
}

func (d *Dispatcher) explorationWeight(builder *BuilderInfo, now time.Time) float64 {
	weight := math.Max(d.expectedReward(builder), minEffectiveScore)

	if d.strategy.MinExploreDispatches > 0 && builder.Stats.DispatchAttempts < d.strategy.MinExploreDispatches {
		missing := float64(d.strategy.MinExploreDispatches - builder.Stats.DispatchAttempts)
		weight *= 1 + d.strategy.UncertaintyWeight*(missing/float64(d.strategy.MinExploreDispatches))
	}

	if d.strategy.NewProducerGracePeriod > 0 && now.After(builder.RegisteredAt) {
		age := now.Sub(builder.RegisteredAt)
		if age < d.strategy.NewProducerGracePeriod {
			freshness := 1 - (float64(age) / float64(d.strategy.NewProducerGracePeriod))
			weight *= 1 + d.strategy.FreshProducerBonus*freshness
		}
	}

	weight *= 1 + d.strategy.UncertaintyWeight/math.Sqrt(float64(builder.Stats.DispatchAttempts)+1)
	return weight
}

func (d *Dispatcher) expectedReward(builder *BuilderInfo) float64 {
	if builder == nil {
		return minEffectiveScore
	}

	reputationComponent := math.Max(builder.Score, minEffectiveScore)
	rewardMean := clamp(builder.Stats.AverageReward, minObservedReward, maxObservedReward)
	rewardMultiplier := 1 + rewardMean
	if rewardMultiplier < 0.10 {
		rewardMultiplier = 0.10
	}
	return reputationComponent * rewardMultiplier
}

func (d *Dispatcher) banditScore(builder *BuilderInfo, now time.Time) float64 {
	score := d.explorationWeight(builder, now)
	totalPulls := d.totalDispatchAttempts() + 1
	ucbBonus := d.strategy.UncertaintyWeight * math.Sqrt(math.Log(float64(totalPulls)+1)/(float64(builder.Stats.RewardSamples)+1))
	return score + ucbBonus
}

// weightedPick 按 score 加权随机选择一个 builder
func (d *Dispatcher) weightedPick(builders []*BuilderInfo, scoreFn func(*BuilderInfo) float64) *BuilderInfo {
	var total float64
	for _, b := range builders {
		total += math.Max(scoreFn(b), minEffectiveScore)
	}

	if total <= 0 {
		return builders[len(builders)-1]
	}

	pick := d.nextFloat64() * total

	var cumulative float64
	for _, b := range builders {
		cumulative += math.Max(scoreFn(b), minEffectiveScore)
		if pick < cumulative {
			return b
		}
	}
	return builders[len(builders)-1]
}

func (d *Dispatcher) nextFloat64() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rng.Float64()
}

func (d *Dispatcher) totalDispatchAttempts() uint64 {
	var total uint64
	for _, builder := range d.registry.All() {
		total += builder.Stats.DispatchAttempts
	}
	return total
}

func extractValueCreated(result *types.SimMevBundleResponse) *big.Int {
	if result == nil {
		return big.NewInt(0)
	}

	total := big.NewInt(0)
	total.Add(total, result.Profit.ToInt())
	total.Add(total, result.RefundableValue.ToInt())
	if total.Sign() < 0 {
		return big.NewInt(0)
	}
	return total
}

// send 通过 JSON-RPC 将 bundle 发送给指定 builder
func (d *Dispatcher) send(ctx context.Context, b *BuilderInfo, bundle *types.SendMevBundleArgs) error {
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendMevBundle",
		ID:      1,
	}

	params, err := json.Marshal([]any{bundle})
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	req.Params = params

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("builder returned status %d", resp.StatusCode)
	}

	var rpcResp types.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return nil
}
