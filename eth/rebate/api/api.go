package api

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"rebate/internal/queue"
	"rebate/internal/sim"
	"rebate/log"
	"rebate/pkg/types"
	"rebate/pkg/utils"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var logger = log.Logger

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
	queue        *queue.SimulationQueue
	store        *sim.BundleStore
	simulator    sim.SimulationBackend
	knownBundles sync.Map // Bundle 缓存, 防止重复处理
	rateLimiter  *RateLimiter
}

// NewMevShareAPI 创建 MEV-Share API
func NewMevShareAPI(
	signer *ecdsa.PrivateKey,
	queue *queue.SimulationQueue,
	store *sim.BundleStore,
	simulator sim.SimulationBackend,
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
func (api *MevShareAPI) SendBundle(ctx context.Context, args types.SendMevBundleArgs) (*types.SendMevBundleResponse, error) {
	log.Logger.Info().
		Str("version", args.Version).
		Uint64("block", uint64(args.Inclusion.BlockNumber)).
		Uint64("maxBlock", uint64(args.Inclusion.MaxBlock)).
		Int("bodyLen", len(args.Body)).
		Msg("Received SendBundle request")

	// 1. 获取当前块号
	currentBlock := api.getCurrentBlock()

	// 2. 验证 Bundle
	bundleHash, hasUnmatchedHash, err := utils.ValidateBundle(&args, currentBlock, api.signer)
	if err != nil {
		log.Logger.Warn().Err(err).Msg("Bundle validation failed")
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 3. 检查是否已处理过
	if _, exists := api.knownBundles.LoadOrStore(bundleHash, time.Now()); exists {
		log.Logger.Debug().
			Str("bundleHash", bundleHash.Hex()).
			Msg("Bundle already known, skipping")
		return &types.SendMevBundleResponse{BundleHash: bundleHash}, nil
	}

	// 4. 设置元数据
	args.Metadata.ReceivedAt = hexutil.Uint64(time.Now().UnixMicro())
	if api.signer != nil {
		args.Metadata.Signer = crypto.PubkeyToAddress(api.signer.PublicKey)
	}

	// 5. 处理 Backrun (如果有未匹配的 Hash 引用)
	if hasUnmatchedHash {
		if err := api.handleBackrun(ctx, &args); err != nil {
			log.Logger.Warn().Err(err).Msg("Failed to handle backrun")
			// 不阻止提交, 可能稍后匹配
		}
	}

	// 6. 存储 Bundle
	api.store.StoreBundle(&args)

	// 7. 加入模拟队列
	priority := false // 可以根据来源或其他条件设置优先级
	api.queue.Push(&args, priority)

	log.Logger.Info().
		Str("bundleHash", bundleHash.Hex()).
		Bool("hasBackrun", hasUnmatchedHash).
		Msg("Bundle accepted")

	return &types.SendMevBundleResponse{BundleHash: bundleHash}, nil
}

// handleBackrun 处理 Backrun (Bundle 引用)
func (api *MevShareAPI) handleBackrun(ctx context.Context, bundle *types.SendMevBundleArgs) error {
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
	if targetBundle.Privacy == nil || !targetBundle.Privacy.Hints.Has(types.HintHash) {
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
	bundle.Metadata.BundleHash = utils.CalculateBundleHash(bodyHashes)
	bundle.Metadata.BodyHashes = bodyHashes

	log.Logger.Info().
		Str("bundleHash", bundle.Metadata.BundleHash.Hex()).
		Str("matchingHash", matchingHash.Hex()).
		Msg("Backrun matched")

	return nil
}

// ============== SimBundle ==============

// SimBundle 模拟 Bundle 执行
func (api *MevShareAPI) SimBundle(ctx context.Context, args types.SendMevBundleArgs) (*types.SimMevBundleResponse, error) {
	// 速率限制
	if !api.rateLimiter.Allow() {
		return nil, ErrRateLimited
	}

	log.Logger.Info().
		Str("version", args.Version).
		Int("bodyLen", len(args.Body)).
		Msg("Received SimBundle request")

	// 验证 Bundle
	currentBlock := api.getCurrentBlock()
	_, _, err := utils.ValidateBundle(&args, currentBlock, api.signer)
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
func (api *MevShareAPI) CancelBundleByHash(ctx context.Context, hash common.Hash) (*types.CancelBundleResponse, error) {
	logger.Info().
		Str("hash", hash.Hex()).
		Msg("Received CancelBundleByHash request")

	cancelled := api.store.Cancel(hash)
	if !cancelled {
		return nil, ErrBundleNotFound
	}

	return &types.CancelBundleResponse{
		Cancelled: []common.Hash{hash},
	}, nil
}

// getCurrentBlock 获取当前块号
func (api *MevShareAPI) getCurrentBlock() uint64 {
	if sim, ok := api.simulator.(*sim.MockSimulator); ok {
		return sim.GetBlock()
	}
	return 1000000 // 默认值
}
