package builder

import (
	"encoding/json"
	"net/http"
	"rebate/mylog"
	"rebate/pkg/types"
)

const (
	methodSendMevBundle      = "eth_sendMevBundle"
	methodSendRawTransaction = "eth_sendRawTransaction"
)

// MockBuilder 是一个 mock builder 节点，接受 eth_sendMevBundle 和 eth_sendRawTransaction 请求
type MockBuilder struct {
	addr string
}

// NewMockBuilder 创建 MockBuilder，addr 为监听地址，例如 ":18545"
func NewMockBuilder(addr string) *MockBuilder {
	return &MockBuilder{addr: addr}
}

// Start 启动 HTTP 服务，阻塞直到出错
func (b *MockBuilder) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", b.handleRPC)

	mylog.Logger.Info().Str("addr", b.addr).Msg("MockBuilder listening")
	return http.ListenAndServe(b.addr, mux)
}

func (b *MockBuilder) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, nil, -32700, "parse error")
		return
	}

	switch req.Method {
	case methodSendMevBundle:
		b.handleSendMevBundle(w, req)
	case methodSendRawTransaction:
		b.handleSendRawTransaction(w, req)
	default:
		writeError(w, req.ID, -32601, "method not found")
	}
}

func (b *MockBuilder) handleSendMevBundle(w http.ResponseWriter, req types.JSONRPCRequest) {
	mylog.Logger.Info().
		Interface("id", req.ID).
		RawJSON("params", req.Params).
		Msg("MockBuilder received eth_sendMevBundle")

	writeResult(w, req.ID, true)
}

func (b *MockBuilder) handleSendRawTransaction(w http.ResponseWriter, req types.JSONRPCRequest) {
	mylog.Logger.Info().
		Interface("id", req.ID).
		RawJSON("params", req.Params).
		Msg("MockBuilder received eth_sendRawTransaction")

	// 返回一个占位 tx hash
	writeResult(w, req.ID, "0x0000000000000000000000000000000000000000000000000000000000000000")
}

func writeResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := types.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := types.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &types.JSONRPCError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
