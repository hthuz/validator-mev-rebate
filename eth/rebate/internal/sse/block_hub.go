package sse

import "sync"

// BlockHub broadcasts every newly produced block to mock users.
type BlockHub struct {
	mu      sync.RWMutex
	clients map[chan uint64]struct{}
}

func NewBlockHub() *BlockHub {
	return &BlockHub{clients: make(map[chan uint64]struct{})}
}

func (h *BlockHub) Subscribe() (<-chan uint64, func()) {
	ch := make(chan uint64, 256)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, cancel
}

func (h *BlockHub) Broadcast(block uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- block:
		default:
			// The consumer is behind; it can recover from the latest block stream.
		}
	}
}
