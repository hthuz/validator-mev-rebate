package sim

import (
	"rebate/pkg/types"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// ============== Bundle 存储 (内存版) ==============

// BundleStore Bundle 存储
type BundleStore struct {
	mu         sync.RWMutex
	bundles    map[common.Hash]*types.SendMevBundleArgs
	simResults map[common.Hash]*types.SimMevBundleResponse
	cancelled  map[common.Hash]bool
	matchIndex map[common.Hash]common.Hash // matchingHash -> bundleHash
}

// NewBundleStore 创建 Bundle 存储
func NewBundleStore() *BundleStore {
	return &BundleStore{
		bundles:    make(map[common.Hash]*types.SendMevBundleArgs),
		simResults: make(map[common.Hash]*types.SimMevBundleResponse),
		cancelled:  make(map[common.Hash]bool),
		matchIndex: make(map[common.Hash]common.Hash),
	}
}

// StoreBundle 存储 Bundle
func (s *BundleStore) StoreBundle(bundle *types.SendMevBundleArgs) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundles[bundle.Metadata.BundleHash] = bundle
}

// GetBundle 获取 Bundle
func (s *BundleStore) GetBundle(hash common.Hash) (*types.SendMevBundleArgs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bundles[hash]
	return b, ok
}

// GetBundleByMatchingHash 通过 MatchingHash 获取 Bundle (用于 backrunning)
func (s *BundleStore) GetBundleByMatchingHash(matchingHash common.Hash) (*types.SendMevBundleArgs, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bundleHash, ok := s.matchIndex[matchingHash]
	if !ok {
		return nil, false
	}
	return s.bundles[bundleHash], true
}

// IndexMatchingHash 索引 MatchingHash
func (s *BundleStore) IndexMatchingHash(matchingHash, bundleHash common.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchIndex[matchingHash] = bundleHash
}

// StoreSimResult 存储模拟结果
func (s *BundleStore) StoreSimResult(hash common.Hash, result *types.SimMevBundleResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.simResults[hash] = result
}

// GetSimResult 获取模拟结果
func (s *BundleStore) GetSimResult(hash common.Hash) (*types.SimMevBundleResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.simResults[hash]
	return r, ok
}

// Cancel 取消 Bundle
func (s *BundleStore) Cancel(hash common.Hash) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.bundles[hash]; exists {
		s.cancelled[hash] = true
		return true
	}
	return false
}

// IsCancelled 检查是否已取消
func (s *BundleStore) IsCancelled(hash common.Hash) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancelled[hash]
}
