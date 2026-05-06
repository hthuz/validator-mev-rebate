package metrics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// MetricsHandler metrics HTTP 处理器
type MetricsHandler struct {
	store *MetricsStore
}

// NewMetricsHandler 创建 metrics 处理器
func NewMetricsHandler(store *MetricsStore) *MetricsHandler {
	return &MetricsHandler{store: store}
}

// GetBlockMetrics 获取区块指标
// GET /metrics/block/{blockNumber}
func (h *MetricsHandler) GetBlockMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从 URL 提取区块号
	path := strings.TrimPrefix(r.URL.Path, "/metrics/block/")
	blockNumber, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid block number")
		return
	}

	metrics, exists := h.store.GetBlockMetrics(blockNumber)
	if !exists {
		writeJSONError(w, http.StatusNotFound, "Block metrics not found")
		return
	}

	writeJSONResponse(w, metrics)
}

// GetValidatorMetrics 获取 Validator 指标
// GET /metrics/validator/{address}
func (h *MetricsHandler) GetValidatorMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从 URL 提取地址
	path := strings.TrimPrefix(r.URL.Path, "/metrics/validator/")
	if !common.IsHexAddress(path) {
		writeJSONError(w, http.StatusBadRequest, "Invalid address")
		return
	}

	address := common.HexToAddress(path)
	metrics, exists := h.store.GetValidatorMetrics(address)
	if !exists {
		writeJSONError(w, http.StatusNotFound, "Validator metrics not found")
		return
	}

	writeJSONResponse(w, metrics)
}

// GetAllValidators 获取所有 Validators
// GET /metrics/validators
func (h *MetricsHandler) GetAllValidators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	validators := h.store.GetAllValidatorMetrics()

	// 转换为列表格式
	type ValidatorSummary struct {
		Address       string  `json:"address"`
		TotalBlocks   uint64  `json:"totalBlocks"`
		MevBlocks     uint64  `json:"mevBlocks"`
		TotalRevenue  string  `json:"totalRevenue"`
		Participation float64 `json:"participationRate"`
		SuccessRate   float64 `json:"successRate"`
	}

	summaries := make([]ValidatorSummary, 0, len(validators))
	for addr, m := range validators {
		summaries = append(summaries, ValidatorSummary{
			Address:       addr.Hex(),
			TotalBlocks:   m.TotalBlocks,
			MevBlocks:     m.MevBlocks,
			TotalRevenue:  m.TotalMevRevenue.String(),
			Participation: m.ParticipationRate,
			SuccessRate:   m.SuccessRate,
		})
	}

	writeJSONResponse(w, summaries)
}

// GetSearcherMetrics 获取 Searcher 指标
// GET /metrics/searcher/{address}
func (h *MetricsHandler) GetSearcherMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从 URL 提取地址
	path := strings.TrimPrefix(r.URL.Path, "/metrics/searcher/")
	if !common.IsHexAddress(path) {
		writeJSONError(w, http.StatusBadRequest, "Invalid address")
		return
	}

	address := common.HexToAddress(path)
	metrics, exists := h.store.GetSearcherMetrics(address)
	if !exists {
		writeJSONError(w, http.StatusNotFound, "Searcher metrics not found")
		return
	}

	writeJSONResponse(w, metrics)
}

// GetAllSearchers 获取所有 Searchers
// GET /metrics/searchers
func (h *MetricsHandler) GetAllSearchers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	searchers := h.store.GetAllSearcherMetrics()

	// 转换为列表格式
	type SearcherSummary struct {
		Address      string  `json:"address"`
		TotalBundles uint64  `json:"totalBundles"`
		SuccessRate  float64 `json:"successRate"`
		TotalProfit  string  `json:"totalProfit"`
		BackrunCount uint64  `json:"backrunCount"`
	}

	summaries := make([]SearcherSummary, 0, len(searchers))
	for addr, m := range searchers {
		summaries = append(summaries, SearcherSummary{
			Address:      addr.Hex(),
			TotalBundles: m.TotalBundles,
			SuccessRate:  m.SuccessRate,
			TotalProfit:  m.TotalProfit.String(),
			BackrunCount: m.BackrunCount,
		})
	}

	writeJSONResponse(w, summaries)
}

// GetGlobalMetrics 获取全局统计
// GET /metrics/global
func (h *MetricsHandler) GetGlobalMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := h.store.GetGlobalMetrics()

	// 转换为易读格式
	type GlobalMetricsResponse struct {
		TotalBlocks      uint64 `json:"totalBlocks"`
		TotalBundles     uint64 `json:"totalBundles"`
		TotalMevProfit   string `json:"totalMevProfit"`
		TotalRefunded    string `json:"totalRefunded"`
		UniqueValidators uint64 `json:"uniqueValidators"`
		UniqueSearchers  uint64 `json:"uniqueSearchers"`
		StartTime        int64  `json:"startTime"`
		Uptime           int64  `json:"uptimeSeconds"`
	}

	now := metrics.UpdatedAt.Unix()
	start := metrics.StartTime.Unix()

	resp := GlobalMetricsResponse{
		TotalBlocks:      metrics.TotalBlocks,
		TotalBundles:     metrics.TotalBundles,
		TotalMevProfit:   metrics.TotalMevProfit.String(),
		TotalRefunded:    metrics.TotalRefunded.String(),
		UniqueValidators: metrics.UniqueValidators,
		UniqueSearchers:  metrics.UniqueSearchers,
		StartTime:        start,
		Uptime:           now - start,
	}

	writeJSONResponse(w, resp)
}

// GetRecentBlocks 获取最近区块
// GET /metrics/recent
func (h *MetricsHandler) GetRecentBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取 limit 参数，默认 10
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	blocks := h.store.GetRecentBlocks(limit)

	// 转换为简化格式
	type BlockSummary struct {
		BlockNumber    uint64  `json:"blockNumber"`
		Validator      string  `json:"validator"`
		BundleCount    int     `json:"bundleCount"`
		SuccessCount   int     `json:"successCount"`
		TotalProfit    string  `json:"totalProfit"`
		TotalGasUsed   uint64  `json:"totalGasUsed"`
		BlockSpaceUsed float64 `json:"blockSpaceUsed"`
	}

	summaries := make([]BlockSummary, 0, len(blocks))
	for _, b := range blocks {
		summaries = append(summaries, BlockSummary{
			BlockNumber:    b.BlockNumber,
			Validator:      b.ValidatorAddress.Hex(),
			BundleCount:    b.BundleCount,
			SuccessCount:   b.SuccessCount,
			TotalProfit:    b.TotalMevProfit.String(),
			TotalGasUsed:   b.TotalGasUsed,
			BlockSpaceUsed: b.BlockSpaceUsed,
		})
	}

	writeJSONResponse(w, summaries)
}

// 辅助函数

func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func writeJSONError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
