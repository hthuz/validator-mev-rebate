package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"rebate/mylog"
	"rebate/pkg/types"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// DispatchRecord 记录一次 bundle 分发
type DispatchRecord struct {
	BundleHash  common.Hash
	BuilderName string
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
	mu       sync.Mutex // 保护 rng
}

// NewDispatcher 创建分发器
func NewDispatcher(registry *Registry) *Dispatcher {
	return &Dispatcher{
		registry: registry,
		log:      newDispatchLog(),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Log 返回分发日志，供外部查询
func (d *Dispatcher) Log() *DispatchLog {
	return d.log
}

// Dispatch 将 bundle 按 score 加权分发给一个 builder。
// 如果 bundle.Privacy.Builders 非空，则只在该列表内的 builder 中选择。
func (d *Dispatcher) Dispatch(ctx context.Context, bundle *types.SendMevBundleArgs) error {
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

	target := d.weightedPick(candidates)
	bundleHash := bundle.Metadata.BundleHash

	mylog.Logger.Info().
		Str("bundleHash", bundleHash.Hex()).
		Str("builder", target.Name).
		Str("url", target.URL).
		Float64("score", target.Score).
		Float64("totalScore", d.registry.TotalScore()).
		Msg("Dispatching bundle to builder")

	err := d.send(ctx, target, bundle)

	rec := DispatchRecord{
		BundleHash:  bundleHash,
		BuilderName: target.Name,
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
	} else {
		mylog.Logger.Info().
			Str("bundleHash", bundleHash.Hex()).
			Str("builder", target.Name).
			Msg("Bundle dispatched successfully")
	}
	d.log.append(rec)
	return err
}

// weightedPick 按 score 加权随机选择一个 builder
func (d *Dispatcher) weightedPick(builders []*BuilderInfo) *BuilderInfo {
	var total float64
	for _, b := range builders {
		total += b.Score
	}

	d.mu.Lock()
	pick := d.rng.Float64() * total
	d.mu.Unlock()

	var cumulative float64
	for _, b := range builders {
		cumulative += b.Score
		if pick < cumulative {
			return b
		}
	}
	return builders[len(builders)-1]
}

// send 通过 JSON-RPC 将 bundle 发送给指定 builder
func (d *Dispatcher) send(ctx context.Context, b *BuilderInfo, bundle *types.SendMevBundleArgs) error {
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendMevBundle",
		ID:      1,
	}

	params, err := json.Marshal([]interface{}{bundle})
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
