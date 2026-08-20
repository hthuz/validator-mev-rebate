package api

import (
	"fmt"
	"net/http"
	"rebate/internal/sse"
)

func NewBlockSSEHandler(hub *sse.BlockHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		blocks, cancel := hub.Subscribe()
		defer cancel()
		fmt.Fprint(w, "event: connected\ndata: {}\n\n")
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case block, ok := <-blocks:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %d\n\n", block)
				flusher.Flush()
			}
		}
	}
}
