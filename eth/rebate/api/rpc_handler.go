package api

import (
	"context"
	"encoding/json"
	"net/http"
	"rebate/pkg/types"

	"github.com/ethereum/go-ethereum/common"
)

// NewJSONRPCHandler 创建 JSON-RPC 处理器
func NewJSONRPCHandler(api *MevShareAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req types.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONRPCError(w, nil, -32700, "Parse error", nil)
			return
		}

		ctx := r.Context()
		var result interface{}
		var rpcErr *types.JSONRPCError

		switch req.Method {
		case SendBundleMethod:
			result, rpcErr = handleSendBundle(ctx, api, req.Params)
		case SimBundleMethod:
			result, rpcErr = handleSimBundle(ctx, api, req.Params)
		case CancelBundleByHash:
			result, rpcErr = handleCancelBundle(ctx, api, req.Params)
		default:
			rpcErr = &types.JSONRPCError{Code: -32601, Message: "Method not found"}
		}

		writeJSONRPCResponse(w, req.ID, result, rpcErr)
	})
}

func handleSendBundle(ctx context.Context, api *MevShareAPI, params json.RawMessage) (interface{}, *types.JSONRPCError) {
	var args []types.SendMevBundleArgs
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, &types.JSONRPCError{Code: -32602, Message: "Invalid params"}
	}

	result, err := api.SendBundle(ctx, args[0])
	if err != nil {
		return nil, &types.JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func handleSimBundle(ctx context.Context, api *MevShareAPI, params json.RawMessage) (interface{}, *types.JSONRPCError) {
	var args []types.SendMevBundleArgs
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, &types.JSONRPCError{Code: -32602, Message: "Invalid params"}
	}

	result, err := api.SimBundle(ctx, args[0])
	if err != nil {
		return nil, &types.JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func handleCancelBundle(ctx context.Context, api *MevShareAPI, params json.RawMessage) (interface{}, *types.JSONRPCError) {
	var args []common.Hash
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, &types.JSONRPCError{Code: -32602, Message: "Invalid params"}
	}

	result, err := api.CancelBundleByHash(ctx, args[0])
	if err != nil {
		return nil, &types.JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func writeJSONRPCResponse(w http.ResponseWriter, id interface{}, result interface{}, rpcErr *types.JSONRPCError) {
	resp := types.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	writeJSONRPCResponse(w, id, nil, &types.JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	})
}
