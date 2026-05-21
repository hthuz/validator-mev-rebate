# RediSwap 分布式架构实现总结

## ✅ 完成情况

已成功将RediSwap改造为类似rebate的分布式架构，包含server、user、arbitrager三个独立组件，通过JSON-RPC通信。

## 架构对比

### RediSwap (新架构)
```
User Client ──rediswap_sendSwap──▶ Server ◀──rediswap_sendBelief── Arbitrager Client
                                      │
                                      ├─ TransactionStore
                                      ├─ BeliefStore
                                      ├─ Pool (AMM)
                                      └─ Auction Engine
```

### rebate (参考架构)
```
User Client ──mev_sendBundle──▶ Server ◀──SSE hints── Searcher Client
                                   │
                                   ├─ BundleStore
                                   ├─ SimulationQueue
                                   ├─ Simulator
                                   └─ MetricsStore
```

## 项目结构

```
rediswap/
├── cmd/
│   ├── server/         # 服务端 (类似 rebate/cmd/server)
│   ├── user/           # 用户客户端 (类似 rebate/cmd/user)
│   └── arbitrager/     # 套利者客户端 (类似 rebate/cmd/searcher)
├── api/
│   └── api.go          # JSON-RPC API处理器
├── internal/
│   ├── auction/        # 拍卖逻辑和MEV计算
│   ├── pool/           # AMM池实现
│   └── store/          # 内存存储 (TransactionStore, BeliefStore)
├── pkg/
│   └── types/          # 共享数据结构
├── bin/                # 编译后的二进制文件
│   ├── server
│   ├── user
│   └── arbitrager
├── test.sh             # 自动化测试脚本
└── go.mod
```

## 核心组件

### 1. Server (`cmd/server/main.go`)
- 监听HTTP端口 (默认8080)
- 接收用户swap交易
- 接收arbitrager的belief
- 运行拍卖并生成bundle
- 管理AMM池状态

### 2. User Client (`cmd/user/main.go`)
- 通过`rediswap_sendSwap`提交swap交易
- 支持单次发送或连续发送模式
- 命令行参数配置

### 3. Arbitrager Client (`cmd/arbitrager/main.go`)
- 通过`rediswap_sendBelief`提交价格belief
- 支持单次发送或连续发送模式
- 参与MEV拍卖

### 4. API Layer (`api/api.go`)
- `rediswap_sendSwap`: 接收swap交易
- `rediswap_sendBelief`: 接收arbitrager belief
- `rediswap_processBlock`: 处理pending交易，运行拍卖

### 5. Storage Layer (`internal/store/`)
- `TransactionStore`: 存储pending swap交易
- `BeliefStore`: 存储arbitrager beliefs
- 内存存储，类似rebate的BundleStore

### 6. Auction Engine (`internal/auction/`)
- MEV计算: `Φ = Δx·q + Δy`
- Rebalancing MEV: `φ = (x·q+y) - (x̂·q+ŷ)`
- Second-price auction机制

## JSON-RPC API

### rediswap_sendSwap
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_sendSwap",
  "params": [{
    "direction": "X->Y",
    "input": 8.0,
    "output": 25.0
  }],
  "id": 1
}
```

### rediswap_sendBelief
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_sendBelief",
  "params": [{
    "arb_id": "arb1",
    "belief": 4.0
  }],
  "id": 1
}
```

### rediswap_processBlock
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_processBlock",
  "params": [],
  "id": 1
}
```

## 使用方法

### 编译
```bash
go mod tidy
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/user ./cmd/user
go build -o bin/arbitrager ./cmd/arbitrager
```

### 运行Example 1
```bash
# 方式1: 使用测试脚本
./test.sh

# 方式2: 手动运行
# Terminal 1: 启动server
./bin/server -port 8080

# Terminal 2-3: 注册arbitragers
./bin/arbitrager -id arb1 -belief 4.0
./bin/arbitrager -id arb2 -belief 1.0

# Terminal 4-6: 发送交易
./bin/user -direction "X->Y" -input 8 -output 25
./bin/user -direction "X->Y" -input 30 -output 12
./bin/user -direction "Y->X" -input 20 -output 10

# Terminal 7: 处理区块
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}'
```

## 验证结果

✅ **完全匹配论文Example 1:**
```json
{
  "auctions": [
    {"tx_id": "TX1", "winner": "arb1", "payment": "0"},
    {"tx_id": "TX2", "winner": "arb1", "payment": "18"},
    {"tx_id": "TX3", "winner": "arb2", "payment": "0"}
  ],
  "rebalancing_winner": "arb2",
  "rebalancing_payment": "36",
  "refunds": [
    {"receiver": "user_TX2", "amount": "18"},
    {"receiver": "LP", "amount": "36"}
  ]
}
```

## 与rebate的相似之处

1. **架构模式**: Server + Client分离
2. **通信协议**: JSON-RPC 2.0
3. **内存存储**: 使用in-memory stores
4. **日志系统**: 使用zerolog
5. **命令行工具**: 独立的可执行文件

## 与rebate的差异

1. **MEV机制**:
   - RediSwap: 基于price belief的second-price auction
   - rebate: 基于bundle simulation的MEV-Share

2. **数据流**:
   - RediSwap: User → Server ← Arbitrager → ProcessBlock
   - rebate: User → Server → SimQueue → Worker → SSE hints

3. **分配方式**:
   - RediSwap: 直接refund给user和LP
   - rebate: 通过hints让searcher backrun

4. **复杂度**:
   - RediSwap: 更简单，专注于拍卖机制
   - rebate: 更复杂，包含simulation、metrics、builder dispatch

## 技术栈

- **语言**: Go 1.21
- **依赖**:
  - `github.com/shopspring/decimal`: 精确数值计算
  - `github.com/rs/zerolog`: 结构化日志
- **协议**: JSON-RPC 2.0
- **存储**: In-memory (无持久化)

## 后续可扩展方向

1. **自动区块处理**: 类似rebate的blockUpdater，定时自动处理
2. **SSE事件流**: 实时推送auction结果
3. **Metrics系统**: 记录MEV统计数据
4. **Builder集成**: 将bundle发送给builder
5. **持久化**: 添加数据库存储
6. **Web UI**: 添加可视化界面

## 总结

成功将RediSwap从单体应用改造为分布式架构，与rebate保持相似的设计模式，便于后续对比和评估。核心拍卖机制完整实现，验证结果与论文完全一致。
