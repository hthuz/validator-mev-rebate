package api

import (
	"fmt"
	"net/http"

	"rebate/internal/sse"
	"rebate/mylog"
)

// NewSSEHandler 返回一个 HTTP handler，searcher 通过 GET /events 订阅 hint 推流
//
// 事件格式 (text/event-stream):
//
//	event: connected
//	data: {}
//
//	data: {"hash":"0x...","txs":[...],"logs":[...],"mevGasPrice":"0x...","gasUsed":"0x..."}
func NewSSEHandler(hub *sse.Hub) http.HandlerFunc {
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

		ch, cancel := hub.Subscribe()
		defer cancel()

		mylog.Logger.Info().
			Str("remote", r.RemoteAddr).
			Msg("SSE searcher connected")

		// 发送连接确认事件
		fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				mylog.Logger.Info().
					Str("remote", r.RemoteAddr).
					Msg("SSE searcher disconnected")
				return
			case data := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}
