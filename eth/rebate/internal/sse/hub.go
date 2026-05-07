package sse

import (
	"encoding/json"
	"sync"

	"rebate/mylog"
	"rebate/pkg/types"
)

// Hub 管理所有 SSE 订阅者，并实现 HintBroadcaster 接口
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan []byte]struct{}),
	}
}

// Subscribe 注册一个新的订阅者，返回接收 hint 的 channel 和取消订阅的函数
func (h *Hub) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 32)

	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	mylog.Logger.Debug().Int("total", h.clientCount()).Msg("SSE client subscribed")

	cancel := func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		mylog.Logger.Debug().Int("total", h.clientCount()).Msg("SSE client unsubscribed")
	}

	return ch, cancel
}

// Broadcast 实现 hints.HintBroadcaster 接口，将 hint 推送给所有订阅者
func (h *Hub) Broadcast(hint *types.Hint) error {
	data, err := json.Marshal(hint)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// 慢速客户端直接跳过，不阻塞广播
			mylog.Logger.Warn().Msg("SSE client too slow, dropping hint")
		}
	}

	mylog.Logger.Info().
		Str("matchingHash", hint.Hash.Hex()).
		Int("subscribers", len(h.clients)).
		Msg("Hint broadcasted via SSE")

	return nil
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
