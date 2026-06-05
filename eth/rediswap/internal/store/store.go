package store

import (
	"sync"

	"rediswap/pkg/types"

	"github.com/shopspring/decimal"
)

// TransactionStore stores pending swap transactions
type TransactionStore struct {
	mu           sync.RWMutex
	transactions map[string]*types.SwapTransaction
	txList       []*types.SwapTransaction
}

// NewTransactionStore creates a new transaction store
func NewTransactionStore() *TransactionStore {
	return &TransactionStore{
		transactions: make(map[string]*types.SwapTransaction),
		txList:       make([]*types.SwapTransaction, 0),
	}
}

// Add adds a transaction to the store
func (s *TransactionStore) Add(tx *types.SwapTransaction) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate transaction ID
	if _, exists := s.transactions[tx.ID]; exists {
		return false
	}

	s.transactions[tx.ID] = tx
	s.txList = append(s.txList, tx)
	return true
}

// GetAll returns all transactions
func (s *TransactionStore) GetAll() []*types.SwapTransaction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.SwapTransaction, len(s.txList))
	copy(result, s.txList)
	return result
}

// Clear clears all transactions
func (s *TransactionStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transactions = make(map[string]*types.SwapTransaction)
	s.txList = make([]*types.SwapTransaction, 0)
}

// BeliefStore stores arbitrager beliefs
type BeliefStore struct {
	mu      sync.RWMutex
	beliefs map[string]decimal.Decimal // arbID -> belief
}

// NewBeliefStore creates a new belief store
func NewBeliefStore() *BeliefStore {
	return &BeliefStore{
		beliefs: make(map[string]decimal.Decimal),
	}
}

// Set sets an arbitrager's belief, returns true if updated, false if new
func (s *BeliefStore) Set(arbID string, belief decimal.Decimal) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, existed := s.beliefs[arbID]
	s.beliefs[arbID] = belief
	return existed
}

// GetAll returns all beliefs
func (s *BeliefStore) GetAll() map[string]decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]decimal.Decimal)
	for k, v := range s.beliefs {
		result[k] = v
	}
	return result
}

// Clear clears all beliefs
func (s *BeliefStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.beliefs = make(map[string]decimal.Decimal)
}
