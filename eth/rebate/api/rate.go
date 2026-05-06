package api

import (
	"sync"
	"time"
)

// ============== 速率限制器 ==============

// RateLimiter 简单的速率限制器
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int
	maxToken int
	interval time.Duration
	lastTime time.Time
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(maxToken int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:   maxToken,
		maxToken: maxToken,
		interval: interval,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime)

	// 补充 tokens
	if elapsed >= r.interval {
		r.tokens = r.maxToken
		r.lastTime = now
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}
