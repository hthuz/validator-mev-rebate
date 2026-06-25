# RediSwap Complete Demo

这是RediSwap的完整demo系统，包含了多个独立运行的实体，类似于rebate系统的架构。

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                       RediSwap Server                        │
│                    (Auto-processing blocks)                   │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │ Transaction  │    │   Belief     │    │   Auction    │  │
│  │    Store     │    │    Store     │    │   Engine     │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│           ▲                  ▲                    │          │
│           │                  │                    ▼          │
└───────────┼──────────────────┼───────────────────────────────┘
            │                  │
            │                  │
    ┌───────┴───────┐   ┌──────┴──────┐
    │               │   │             │
┌───▼────┐    ┌────▼───┐  ┌──────────▼──┐    ┌──────────┐
│ User 1 │    │ User 2 │  │ Arbitrager 1 │    │Arbitrager│
│ (X->Y) │    │ (Y->X) │  │ (belief=4.0) │    │    2     │
└────────┘    └────────┘  └──────────────┘    │(belief=5)│
                                               └──────────┘
```

## 系统组件

### 1. Server（服务器）
- **职责**：中央协调器，接收swap和belief，自动运行拍卖
- **功能**：
  - 管理AMM池状态（x*y=k）
  - 接收用户的swap交易请求
  - 接收套利者的价格belief
  - 定时自动处理区块，运行拍卖
  - 生成bundle和退款

### 2. User（用户）
- **职责**：提交swap交易
- **功能**：
  - 持续发送swap请求（X->Y 或 Y->X）
  - 指定输入量和最小输出量
  - 接收拍卖支付作为退款

### 3. Arbitrager（套利者）
- **职责**：提供价格信号并参与竞拍
- **功能**：
  - 持续更新价格belief（1X = belief*Y）
  - 参与second-price拍卖
  - 赢家获得sandwich bundle构建权

## 快速开始

### 方式1：运行完整demo（推荐）

```bash
./demo.sh
```

这会自动启动：
- 1个Server（自动每10秒处理一次区块）
- 2个User（持续发送不同方向的swap）
- 2个Arbitrager（持续更新不同的belief）

### 方式2：手动启动各个组件

#### 终端1：启动Server（自动处理模式）
```bash
./bin/server -port 8080 -pool-x 4 -pool-y 100 -auto-process -process-interval 10
```

#### 终端2：启动Arbitrager 1
```bash
./bin/arbitrager -server http://localhost:8080 -id arb1 -belief 4.0 -continuous
```

#### 终端3：启动Arbitrager 2
```bash
./bin/arbitrager -server http://localhost:8080 -id arb2 -belief 5.0 -continuous
```

#### 终端4：启动User 1（X->Y swaps）
```bash
./bin/user -server http://localhost:8080 -direction "X->Y" -input 8 -output 25 -continuous
```

#### 终端5：启动User 2（Y->X swaps）
```bash
./bin/user -server http://localhost:8080 -direction "Y->X" -input 30 -output 12 -continuous
```

### 方式3：手动处理区块（原有方式）

如果不使用`-auto-process`标志，可以手动触发区块处理：

```bash
# 启动server（不自动处理）
./bin/server -port 8080 -pool-x 4 -pool-y 100

# 发送一些交易和belief后，手动处理区块
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}'
```

## 命令行参数

### Server
```
-port int              Server端口 (默认: 8080)
-pool-x float          初始池X储备量 (默认: 4)
-pool-y float          初始池Y储备量 (默认: 100)
-auto-process          启用自动区块处理 (默认: false)
-process-interval int  区块处理间隔（秒） (默认: 10)
```

### User
```
-server string         Server URL (默认: http://localhost:8080)
-direction string      Swap方向 "X->Y" 或 "Y->X" (默认: "X->Y")
-input float           输入数量 (默认: 8)
-output float          最小输出数量 (默认: 25)
-continuous            持续发送交易 (默认: false)
-interval int          交易间隔（秒） (默认: 3)
```

### Arbitrager
```
-server string         Server URL (默认: http://localhost:8080)
-id string             套利者ID (默认: "arb1")
-belief float          价格belief，1X = belief*Y (默认: 4.0)
-continuous            持续发送belief (默认: false)
-interval int          belief更新间隔（秒） (默认: 5)
```

## 日志文件

运行demo时，日志会写入`logs/`目录：

```bash
tail -f logs/server.log    # 查看server日志（包括拍卖结果）
tail -f logs/user1.log     # 查看user 1的交易
tail -f logs/user2.log     # 查看user 2的交易
tail -f logs/arb1.log      # 查看arbitrager 1的belief更新
tail -f logs/arb2.log      # 查看arbitrager 2的belief更新
```

## 预期输出示例

### Server日志
```
2025-01-xx ... Processing block transactions=2 arbitragers=2
2025-01-xx ... Running auction for transaction tx_id=TX1 direction=X->Y
2025-01-xx ... Auction completed tx_id=TX1 winner=arb1 payment=12.50
2025-01-xx ... Bundle generated tx_id=TX1 winner=arb1 operations=3
2025-01-xx ... User refund created user=user_TX1 refund=12.50
2025-01-xx ... Running rebalancing auction
2025-01-xx ... Rebalancing auction completed winner=arb2 payment=36.00
2025-01-xx ... LP refund created refund=36.00
2025-01-xx ... Block processing complete bundles=2 auctions=2 refunds=3 total_refund=48.50
```

### User日志
```
2025-01-xx ... Starting RediSwap User Client...
2025-01-xx ... Configuration server=http://localhost:8080 direction=X->Y input=8 output=25
2025-01-xx ... Running in continuous mode interval_seconds=3
2025-01-xx ... Sending swap transaction tx_count=1
2025-01-xx ... Swap accepted by server tx_id=TX1 status=pending
2025-01-xx ... Swap transaction sent successfully tx_count=1
```

### Arbitrager日志
```
2025-01-xx ... Starting RediSwap Arbitrager Client...
2025-01-xx ... Configuration server=http://localhost:8080 arb_id=arb1 belief=4
2025-01-xx ... Running in continuous mode interval_seconds=5
2025-01-xx ... Sending belief update update_count=1
2025-01-xx ... Belief accepted by server arb_id=arb1 status=registered
2025-01-xx ... Belief registered successfully update_count=1
```

## 核心机制

### Second-Price Auction（二价拍卖）
- 每个交易运行独立的拍卖
- 最高出价者赢得，但支付第二高价
- MEV计算公式：`Φ = Δx·q + Δy`（q是套利者的belief）

### Rebalancing Auction（再平衡拍卖）
- 处理LP的LVR（Loss-Versus-Rebalancing）
- MEV计算：`φ = (x·q + y) - (x̂·q + ŷ)`
- 退款支付给LP

### Bundle Structure（Bundle结构）
```
1. Front-run:  套利者推动价格到用户极限价格
2. User Tx:    用户在恶化的价格执行交易
3. Back-run:   套利者恢复价格到无套利状态
```

## 与Rebate系统的对比

| 特性 | Rebate | RediSwap |
|------|--------|----------|
| 核心机制 | MEV-Share (Hints + Backrunning) | Second-price Auction |
| 参与者 | Searcher（订阅hints） | Arbitrager（竞价） |
| 触发方式 | 自动（后台worker） | 自动或手动 |
| 数据流 | Bundle → Simulate → Hints → SSE | Swap + Belief → Auction → Bundle |
| 隐私保护 | Hint精度降低 | 直接价格信号 |
| 退款方式 | Builder分发 | 直接退款给用户/LP |

## 测试原有功能

原有的`test.sh`脚本仍然可用（重现论文Example 1）：

```bash
./test.sh
```

## 故障排查

### Server无法启动
- 检查端口8080是否被占用：`lsof -i :8080`
- 查看server日志：`cat logs/server.log`

### 客户端连接失败
- 确保server已启动并监听
- 检查防火墙设置
- 验证URL：`curl http://localhost:8080/health`

### 没有拍卖结果
- 确保至少有1个arbitrager注册了belief
- 确保有pending的交易
- 检查是否启用了auto-process或手动调用processBlock

## 构建说明

```bash
# 安装依赖
go mod tidy

# 构建所有组件
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/user ./cmd/user
go build -o bin/arbitrager ./cmd/arbitrager

# 或者使用demo.sh会自动构建
./demo.sh
```

## API Reference

### rediswap_sendSwap
提交swap交易
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_sendSwap",
  "params": [{
    "direction": "X->Y",
    "input": 8,
    "output": 25
  }],
  "id": 1
}
```

### rediswap_sendBelief
提交价格belief
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
手动触发区块处理（仅在未启用auto-process时需要）
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_processBlock",
  "params": [],
  "id": 1
}
```

## 进一步阅读

- 原有文档：[CLAUDE.md](./CLAUDE.md)
- 论文：RediSwap: MEV Redistribution Mechanism for CFMMs (Zhang et al., 2025)
- 测试脚本：[test.sh](./test.sh)
