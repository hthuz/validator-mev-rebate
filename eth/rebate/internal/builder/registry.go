package builder

import (
	"fmt"
	"math"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const (
	minEffectiveScore         = 0.05
	maxEffectiveScoreBoost    = 3.0
	sandwichPenaltyPerEvent   = 0.20
	wellBehavedBonusPerEvent  = 0.05
	consecutiveFailurePenalty = 0.12
	maxValueReward            = 0.75
	rewardSuccessWeight       = 1.00
	rewardValueWeight         = 0.60
	rewardWellBehavedWeight   = 0.15
	rewardFailureWeight       = 0.70
	rewardSandwichWeight      = 1.10
	minObservedReward         = -2.0
	maxObservedReward         = 2.0
)

// BuilderInfo 描述一个 builder 节点
type BuilderInfo struct {
	Name          string
	URL           string
	RegisteredAt  time.Time
	BaseScore     float64 // 配置分，表示 builder 的初始信誉
	Score         float64 // 动态分，决定实际分发权重
	Stats         BuilderStats
	client        *http.Client
	totalValueWei *big.Int
}

// BuilderStats 描述 builder 的行为画像
type BuilderStats struct {
	DispatchAttempts    uint64
	DispatchSuccesses   uint64
	DispatchFailures    uint64
	SandwichAttacks     uint64
	WellBehavedEvents   uint64
	ValuableOrderFlow   uint64
	ConsecutiveFailures uint64
	RewardSamples       uint64
	TotalReward         float64
	AverageReward       float64
	LastReward          float64
	LastUpdatedAt       time.Time
}

// BuilderObservation 是一次行为观测结果，会影响动态 score。
// 设计原则：
// 1. 三明治攻击等负面事件强惩罚；
// 2. 高成功率、高价值成交、well-behaved 行为给予正向奖励；
// 3. 所有奖励和惩罚都只影响动态 score，不改动配置里的 BaseScore。
type BuilderObservation struct {
	DispatchAttempts  uint64
	DispatchSuccesses uint64
	SandwichAttacks   uint64
	WellBehavedEvents uint64
	ValueCreatedWei   *big.Int
	Reward            *float64
}

// Registry 管理所有已注册的 builder
type Registry struct {
	mu       sync.RWMutex
	builders []*BuilderInfo
}

// NewRegistry 创建空注册表
func NewRegistry() *Registry {
	return &Registry{}
}

// Register 注册一个 builder。score 必须 > 0
func (r *Registry) Register(name, url string, score float64) error {
	if score <= 0 {
		return fmt.Errorf("builder %q: score must be > 0, got %f", name, score)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.builders {
		if b.Name == name {
			return fmt.Errorf("builder %q already registered", name)
		}
	}

	r.builders = append(r.builders, &BuilderInfo{
		Name:          name,
		URL:           url,
		RegisteredAt:  time.Now(),
		BaseScore:     score,
		Score:         score,
		totalValueWei: big.NewInt(0),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	})
	return nil
}

// UpdateScore 更新已注册 builder 的 score
func (r *Registry) UpdateScore(name string, score float64) error {
	if score <= 0 {
		return fmt.Errorf("score must be > 0")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.builders {
		if b.Name == name {
			b.BaseScore = score
			b.Score = computeEffectiveScore(b)
			return nil
		}
	}
	return fmt.Errorf("builder %q not found", name)
}

// Observe 将新的行为观测合并进 builder 画像，并重新计算动态 score。
func (r *Registry) Observe(name string, observation BuilderObservation) (*BuilderInfo, error) {
	if observation.DispatchSuccesses > observation.DispatchAttempts {
		return nil, fmt.Errorf("dispatch successes cannot exceed dispatch attempts")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.builders {
		if b.Name != name {
			continue
		}

		b.Stats.DispatchAttempts += observation.DispatchAttempts
		b.Stats.DispatchSuccesses += observation.DispatchSuccesses
		failures := observation.DispatchAttempts - observation.DispatchSuccesses
		b.Stats.DispatchFailures += failures
		b.Stats.SandwichAttacks += observation.SandwichAttacks
		b.Stats.WellBehavedEvents += observation.WellBehavedEvents

		if failures > 0 && observation.DispatchSuccesses == 0 {
			b.Stats.ConsecutiveFailures += failures
		} else if observation.DispatchSuccesses > 0 {
			b.Stats.ConsecutiveFailures = 0
		}

		if observation.ValueCreatedWei != nil && observation.ValueCreatedWei.Sign() > 0 {
			if b.totalValueWei == nil {
				b.totalValueWei = big.NewInt(0)
			}
			b.totalValueWei.Add(b.totalValueWei, observation.ValueCreatedWei)
			b.Stats.ValuableOrderFlow++
		}

		reward := computeReward(observation)
		b.Stats.RewardSamples++
		b.Stats.TotalReward += reward
		b.Stats.AverageReward = b.Stats.TotalReward / float64(b.Stats.RewardSamples)
		b.Stats.LastReward = reward
		b.Stats.LastUpdatedAt = time.Now()
		b.Score = computeEffectiveScore(b)
		return cloneBuilderInfo(b), nil
	}

	return nil, fmt.Errorf("builder %q not found", name)
}

// All 返回所有 builder 的快照（只读副本）
func (r *Registry) All() []*BuilderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*BuilderInfo, len(r.builders))
	for i, b := range r.builders {
		out[i] = cloneBuilderInfo(b)
	}
	return out
}

// TotalScore 返回所有 builder score 之和
func (r *Registry) TotalScore() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total float64
	for _, b := range r.builders {
		total += b.Score
	}
	return total
}

func cloneBuilderInfo(in *BuilderInfo) *BuilderInfo {
	if in == nil {
		return nil
	}

	out := *in
	if in.totalValueWei != nil {
		out.totalValueWei = new(big.Int).Set(in.totalValueWei)
	} else {
		out.totalValueWei = big.NewInt(0)
	}
	return &out
}

func computeEffectiveScore(builder *BuilderInfo) float64 {
	base := builder.BaseScore
	if base <= 0 {
		base = 1
	}

	successRate := 1.0
	if builder.Stats.DispatchAttempts > 0 {
		successRate = float64(builder.Stats.DispatchSuccesses) / float64(builder.Stats.DispatchAttempts)
	}
	reliabilityFactor := clamp(0.5+successRate, 0.35, 1.50)

	failureFactor := clamp(1.0-consecutiveFailurePenalty*float64(builder.Stats.ConsecutiveFailures), 0.40, 1.0)
	sandwichFactor := clamp(1.0-sandwichPenaltyPerEvent*float64(builder.Stats.SandwichAttacks), 0.10, 1.0)
	wellBehavedFactor := clamp(1.0+wellBehavedBonusPerEvent*float64(builder.Stats.WellBehavedEvents), 1.0, 1.5)
	valueFactor := 1.0 + clamp(valueReward(builder.totalValueWei), 0, maxValueReward)

	score := base * reliabilityFactor * failureFactor * sandwichFactor * wellBehavedFactor * valueFactor
	return clamp(score, minEffectiveScore, base*maxEffectiveScoreBoost)
}

func computeReward(observation BuilderObservation) float64 {
	if observation.Reward != nil {
		return clamp(*observation.Reward, minObservedReward, maxObservedReward)
	}

	successRate := 0.0
	failureRate := 0.0
	if observation.DispatchAttempts > 0 {
		successRate = float64(observation.DispatchSuccesses) / float64(observation.DispatchAttempts)
		failures := observation.DispatchAttempts - observation.DispatchSuccesses
		failureRate = float64(failures) / float64(observation.DispatchAttempts)
	}

	valueComponent := clamp(valueReward(observation.ValueCreatedWei), 0, maxValueReward)
	wellBehavedComponent := clamp(float64(observation.WellBehavedEvents)*wellBehavedBonusPerEvent, 0, 0.50)
	sandwichPenalty := float64(observation.SandwichAttacks) * rewardSandwichWeight

	reward := rewardSuccessWeight*successRate +
		rewardValueWeight*valueComponent +
		rewardWellBehavedWeight*wellBehavedComponent -
		rewardFailureWeight*failureRate -
		sandwichPenalty

	return clamp(reward, minObservedReward, maxObservedReward)
}

func valueReward(valueWei *big.Int) float64 {
	if valueWei == nil || valueWei.Sign() <= 0 {
		return 0
	}

	ethValue, _ := new(big.Float).Quo(new(big.Float).SetInt(valueWei), big.NewFloat(1e18)).Float64()
	return math.Log10(1+ethValue) * 0.20
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
