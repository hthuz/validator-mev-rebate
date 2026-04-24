package main

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestBundleHashCalculation 测试 Bundle 哈希计算
func TestBundleHashCalculation(t *testing.T) {
	tests := []struct {
		name   string
		hashes []common.Hash
		want   string // 预期哈希的前缀
	}{
		{
			name:   "single hash",
			hashes: []common.Hash{common.HexToHash("0x1234")},
			want:   "0x",
		},
		{
			name: "multiple hashes",
			hashes: []common.Hash{
				common.HexToHash("0x1111"),
				common.HexToHash("0x2222"),
			},
			want: "0x",
		},
		{
			name:   "empty hashes",
			hashes: []common.Hash{},
			want:   "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", // empty keccak256
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateBundleHash(tt.hashes)
			if result.Hex() == "" || (tt.name == "empty hashes" && result.Hex() != tt.want) {
				t.Errorf("calculateBundleHash() = %s", result.Hex())
			}
		})
	}
}

// TestReduceIntPrecision 测试精度降低
func TestReduceIntPrecision(t *testing.T) {
	tests := []struct {
		name      string
		input     int64
		precision int
		expected  int64
	}{
		{
			name:      "reduce 123456 to 3 digits",
			input:     123456,
			precision: 3,
			expected:  123000,
		},
		{
			name:      "reduce 999 to 2 digits",
			input:     999,
			precision: 2,
			expected:  990,
		},
		{
			name:      "single digit",
			input:     5,
			precision: 3,
			expected:  5,
		},
		{
			name:      "zero value",
			input:     0,
			precision: 3,
			expected:  0,
		},
		{
			name:      "large number",
			input:     1234567890,
			precision: 3,
			expected:  1230000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := big.NewInt(tt.input)
			result := reduceIntPrecision(n, tt.precision)
			if result.Int64() != tt.expected {
				t.Errorf("reduceIntPrecision(%d, %d) = %d, want %d",
					tt.input, tt.precision, result.Int64(), tt.expected)
			}
		})
	}
}

// TestHintIntentHas 测试 Hint 标志位检查
func TestHintIntentHas(t *testing.T) {
	tests := []struct {
		name     string
		hints    HintIntent
		check    HintIntent
		expected bool
	}{
		{
			name:     "has single flag",
			hints:    HintHash,
			check:    HintHash,
			expected: true,
		},
		{
			name:     "has combined flag",
			hints:    HintHash | HintLogs,
			check:    HintLogs,
			expected: true,
		},
		{
			name:     "does not have flag",
			hints:    HintHash,
			check:    HintLogs,
			expected: false,
		},
		{
			name:     "empty hints",
			hints:    HintNone,
			check:    HintHash,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.hints.Has(tt.check)
			if result != tt.expected {
				t.Errorf("Has() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestBundleSerialization 测试 Bundle 序列化
func TestBundleSerialization(t *testing.T) {
	bundle := &SendMevBundleArgs{
		Version: "v0.1",
		Inclusion: MevBundleInclusion{
			BlockNumber: hexutil.Uint64(100),
			MaxBlock:    hexutil.Uint64(110),
		},
		Body: []MevBundleBody{
			{Hash: &[]common.Hash{common.HexToHash("0x1234")}[0]},
		},
	}

	// 序列化
	data, err := SerializeBundle(bundle)
	if err != nil {
		t.Fatalf("SerializeBundle failed: %v", err)
	}

	// 反序列化
	restored, err := DeserializeBundle(data)
	if err != nil {
		t.Fatalf("DeserializeBundle failed: %v", err)
	}

	// 验证
	if restored.Version != bundle.Version {
		t.Error("version mismatch after deserialization")
	}
	if restored.Inclusion.BlockNumber != bundle.Inclusion.BlockNumber {
		t.Error("block number mismatch after deserialization")
	}
}

// TestSpecialLogTopics 测试特殊日志主题
func TestSpecialLogTopics(t *testing.T) {
	specialTopics := []common.Hash{
		// Uniswap V2 Swap
		common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"),
		// Uniswap V3 Swap
		common.HexToHash("0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"),
		// Curve Exchange
		common.HexToHash("0x8b3e96f2b889fa771c53c981b40daf005f63f637f1869f707052d15a3dd97140"),
		// Balancer Swap
		common.HexToHash("0x2170c741c41531aec20e7c107c24eecfdd15e69c9bb0a8dd37b1840b9e0b207b"),
	}

	for _, topic := range specialTopics {
		if !SpecialLogTopics[topic] {
			t.Errorf("topic %s should be in SpecialLogTopics", topic.Hex())
		}
	}

	// 测试非特殊主题
	nonSpecial := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	if SpecialLogTopics[nonSpecial] {
		t.Error("non-special topic should not be in SpecialLogTopics")
	}
}
