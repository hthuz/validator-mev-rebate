package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// ============== API 常量 ==============

const (
	SendBundleMethod   = "mev_sendBundle"
	SimBundleMethod    = "mev_simBundle"
	CancelBundleByHash = "eth_cancelBundleByHash"
)

// ============== API 错误 ==============

var (
	ErrInvalidParams  = errors.New("invalid params")
	ErrBundleNotFound = errors.New("bundle not found")
	ErrRateLimited    = errors.New("rate limited")
)

// ============== API 实现 ==============

// MevShareAPI MEV-Share API
type MevShareAPI struct {
	signer       *ecdsa.PrivateKey
	queue        *SimulationQueue
	store        *BundleStore
	simulator    SimulationBackend
	knownBundles sync.Map // Bundle 缓存, 防止重复处理
	rateLimiter  *RateLimiter
}

// NewMevShareAPI 创建 MEV-Share API
func NewMevShareAPI(
	signer *ecdsa.PrivateKey,
	queue *SimulationQueue,
	store *BundleStore,
	simulator SimulationBackend,
) *MevShareAPI {
	return &MevShareAPI{
		signer:      signer,
		queue:       queue,
		store:       store,
		simulator:   simulator,
		rateLimiter: NewRateLimiter(10, time.Second), // 10 req/s
	}
}

// ============== SendBundle ==============

// SendBundle 提交 MEV Bundle
func (api *MevShareAPI) SendBundle(ctx context.Context, args SendMevBundleArgs) (*SendMevBundleResponse, error) {
	logger.Info().
		Str("version", args.Version).
		Uint64("block", uint64(args.Inclusion.BlockNumber)).
		Uint64("maxBlock", uint64(args.Inclusion.MaxBlock)).
		Int("bodyLen", len(args.Body)).
		Msg("Received SendBundle request")

	// 1. 获取当前块号
	currentBlock := api.getCurrentBlock()

	// 2. 验证 Bundle
	bundleHash, hasUnmatchedHash, err := ValidateBundle(&args, currentBlock, api.signer)
	if err != nil {
		logger.Warn().Err(err).Msg("Bundle validation failed")
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 3. 检查是否已处理过
	if _, exists := api.knownBundles.LoadOrStore(bundleHash, time.Now()); exists {
		logger.Debug().
			Str("bundleHash", bundleHash.Hex()).
			Msg("Bundle already known, skipping")
		return &SendMevBundleResponse{BundleHash: bundleHash}, nil
	}

	// 4. 设置元数据
	args.Metadata.ReceivedAt = hexutil.Uint64(time.Now().UnixMicro())
	if api.signer != nil {
		args.Metadata.Signer = crypto.PubkeyToAddress(api.signer.PublicKey)
	}

	// 5. 处理 Backrun (如果有未匹配的 Hash 引用)
	if hasUnmatchedHash {
		if err := api.handleBackrun(ctx, &args); err != nil {
			logger.Warn().Err(err).Msg("Failed to handle backrun")
			// 不阻止提交, 可能稍后匹配
		}
	}

	// 6. 存储 Bundle
	api.store.StoreBundle(&args)

	// 7. 加入模拟队列
	priority := false // 可以根据来源或其他条件设置优先级
	api.queue.Push(&args, priority)

	logger.Info().
		Str("bundleHash", bundleHash.Hex()).
		Bool("hasBackrun", hasUnmatchedHash).
		Msg("Bundle accepted")

	return &SendMevBundleResponse{BundleHash: bundleHash}, nil
}

// handleBackrun 处理 Backrun (Bundle 引用)
func (api *MevShareAPI) handleBackrun(ctx context.Context, bundle *SendMevBundleArgs) error {
	if len(bundle.Body) == 0 || bundle.Body[0].Hash == nil {
		return nil
	}

	matchingHash := *bundle.Body[0].Hash

	// 查找被引用的 Bundle
	targetBundle, found := api.store.GetBundleByMatchingHash(matchingHash)
	if !found {
		return fmt.Errorf("referenced bundle not found: %s", matchingHash.Hex())
	}

	// 检查被引用的 Bundle 是否允许 Hash Hint
	if targetBundle.Privacy == nil || !targetBundle.Privacy.Hints.Has(HintHash) {
		return errors.New("referenced bundle does not allow hash hint")
	}

	// 替换 Hash 引用为实际 Bundle
	bundle.Body[0].Hash = nil
	bundle.Body[0].Bundle = targetBundle
	bundle.Metadata.Prematched = true

	// 合并 Inclusion 范围
	minBlock := min(uint64(bundle.Inclusion.BlockNumber), uint64(targetBundle.Inclusion.BlockNumber))
	maxBlock := min(uint64(bundle.Inclusion.MaxBlock), uint64(targetBundle.Inclusion.MaxBlock))
	bundle.Inclusion.BlockNumber = hexutil.Uint64(minBlock)
	bundle.Inclusion.MaxBlock = hexutil.Uint64(maxBlock)

	// 重新计算 Bundle Hash
	bodyHashes := []common.Hash{targetBundle.Metadata.BundleHash}
	bodyHashes = append(bodyHashes, bundle.Metadata.BodyHashes[1:]...)
	bundle.Metadata.BundleHash = calculateBundleHash(bodyHashes)
	bundle.Metadata.BodyHashes = bodyHashes

	logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Str("matchingHash", matchingHash.Hex()).
		Msg("Backrun matched")

	return nil
}

// ============== SimBundle ==============

// SimBundle 模拟 Bundle 执行
func (api *MevShareAPI) SimBundle(ctx context.Context, args SendMevBundleArgs) (*SimMevBundleResponse, error) {
	// 速率限制
	if !api.rateLimiter.Allow() {
		return nil, ErrRateLimited
	}

	logger.Info().
		Str("version", args.Version).
		Int("bodyLen", len(args.Body)).
		Msg("Received SimBundle request")

	// 验证 Bundle
	currentBlock := api.getCurrentBlock()
	_, _, err := ValidateBundle(&args, currentBlock, api.signer)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 调用模拟器
	result, err := api.simulator.SimulateBundle(ctx, &args, nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ============== CancelBundleByHash ==============

// CancelBundleByHash 取消 Bundle
func (api *MevShareAPI) CancelBundleByHash(ctx context.Context, hash common.Hash) (*CancelBundleResponse, error) {
	logger.Info().
		Str("hash", hash.Hex()).
		Msg("Received CancelBundleByHash request")

	cancelled := api.store.Cancel(hash)
	if !cancelled {
		return nil, ErrBundleNotFound
	}

	return &CancelBundleResponse{
		Cancelled: []common.Hash{hash},
	}, nil
}

// getCurrentBlock 获取当前块号
func (api *MevShareAPI) getCurrentBlock() uint64 {
	if sim, ok := api.simulator.(*MockSimulator); ok {
		return sim.GetBlock()
	}
	return 1000000 // 默认值
}

// ============== JSON-RPC 服务器 ==============

// JSONRPCRequest JSON-RPC 请求
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// JSONRPCResponse JSON-RPC 响应
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

// JSONRPCError JSON-RPC 错误
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewJSONRPCHandler 创建 JSON-RPC 处理器
func NewJSONRPCHandler(api *MevShareAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONRPCError(w, nil, -32700, "Parse error", nil)
			return
		}

		ctx := r.Context()
		var result interface{}
		var rpcErr *JSONRPCError

		switch req.Method {
		case SendBundleMethod:
			result, rpcErr = handleSendBundle(ctx, api, req.Params)
		case SimBundleMethod:
			result, rpcErr = handleSimBundle(ctx, api, req.Params)
		case CancelBundleByHash:
			result, rpcErr = handleCancelBundle(ctx, api, req.Params)
		default:
			rpcErr = &JSONRPCError{Code: -32601, Message: "Method not found"}
		}

		writeJSONRPCResponse(w, req.ID, result, rpcErr)
	})
}

func handleSendBundle(ctx context.Context, api *MevShareAPI, params json.RawMessage) (interface{}, *JSONRPCError) {
	var args []SendMevBundleArgs
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "Invalid params"}
	}

	result, err := api.SendBundle(ctx, args[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func handleSimBundle(ctx context.Context, api *MevShareAPI, params json.RawMessage) (interface{}, *JSONRPCError) {
	var args []SendMevBundleArgs
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "Invalid params"}
	}

	result, err := api.SimBundle(ctx, args[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func handleCancelBundle(ctx context.Context, api *MevShareAPI, params json.RawMessage) (interface{}, *JSONRPCError) {
	var args []common.Hash
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "Invalid params"}
	}

	result, err := api.CancelBundleByHash(ctx, args[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func writeJSONRPCResponse(w http.ResponseWriter, id interface{}, result interface{}, rpcErr *JSONRPCError) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	writeJSONRPCResponse(w, id, nil, &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

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

// min 返回较小值
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
