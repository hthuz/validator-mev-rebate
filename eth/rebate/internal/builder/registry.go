package builder

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// BuilderInfo 描述一个 builder 节点
type BuilderInfo struct {
	Name   string
	URL    string
	Score  float64 // 越高分配越多的 bundle
	client *http.Client
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
		Name:  name,
		URL:   url,
		Score: score,
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
			b.Score = score
			return nil
		}
	}
	return fmt.Errorf("builder %q not found", name)
}

// All 返回所有 builder 的快照（只读副本）
func (r *Registry) All() []*BuilderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*BuilderInfo, len(r.builders))
	copy(out, r.builders)
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
