# Mitigation - 交易发送策略模拟平台

## 概述

Mitigation模块是一个模拟交易发送策略的平台，用于研究和测试不同的交易发送时机对sandwich攻击缓解的效果。

## Slot 机制（事件驱动）

**slot 就是 slot 号（等于 block 号，整数），跟 blockTime/timestamp 没有任何关系。** 真实链上 400ms 一个 slot，但这里是历史交易回放模拟——一个 slot 的交易处理完就跳到下一个 slot，与时间无关。

server 记录的 slot 表示「user 需要发送这个 slot 的交易」。整条链路没有定时器，完全由事件驱动：

```
1. server 进入新 slot（从 MITIGATION_START_SLOT 开始，每次 +1）
       ↓  Socket.IO 广播 new_slot {slot, leader}
2. user 收到广播，从 BLOCKS_DATA 取该 slot 的交易
       ↓  逐笔 POST /sendTransaction
3. server 收到交易后立即按当前策略处理
   （immediate 策略：收到就立刻发给验证者，不等 slot 推进）
       ↓  user 发 end 标识：{"slot": slot, "end": true}
4. server 收到 end → 处理收尾交易 → 推进到下一个 slot（slot + 1）→ 回到 1
5. MITIGATION_END_SLOT 处理完后，不再广播 new_slot，只发 simulation_done
```

关键点：
- **slot 就是 slot 号**，范围由 `MITIGATION_START_SLOT` / `MITIGATION_END_SLOT` 决定（两端 inclusive），每次固定 +1，与 blockTime 无关。
- **不再 sleep 固定时间后推进 slot**，收到 `end` 且处理完交易才推进。
- **immediate 策略在收到交易时就发给验证者**，不用等到 slot 推进。
- 非当前 slot 的 `end` 标识会被忽略。
- **END slot 处理完后不再广播 new_slot**，只发 `simulation_done`；slot 也不会再推进。

## 功能特性

- **交易接收与管理**: 通过HTTP API接收用户交易
- **Slot模拟**: 基于 slot 号的事件驱动 slot 推进
- **多策略支持**:
  - ImmediateStrategy: 收到交易立即发送
  - DelayedStrategy: 延迟 N 个 slot 发送
- **Socket.IO 广播**: server 进入新 slot 时广播给 user

## 架构

```
mitigation/
├── server.py               # Flask + Socket.IO 服务器
├── slot_manager.py         # Slot 管理器（slot 号 -> leader）
├── transaction_manager.py  # 交易管理器（含 end 标识）
├── strategy/               # 策略模块
│   ├── base_strategy.py
│   ├── immediate_strategy.py
│   └── delayed_strategy.py
├── user/
│   └── client.py           # 用户客户端（广播驱动 + BLOCKS_DATA）
├── docx/
├── tests/
└── scripts/
```

## 快速开始

### 1. 安装依赖

```bash
uv sync
```

### 2. 配置环境变量

`.env` 中至少包含：

```env
BLOCK_PATH=/path/to/processed_blocks
SLOT_LEADER_PATH=/path/to/slot_leaders.csv
MITIGATION_START_SLOT=346769956
MITIGATION_END_SLOT=346769958
SERVER_HOST=0.0.0.0
SERVER_PORT=5000
```

`MITIGATION_START_SLOT` / `MITIGATION_END_SLOT` 决定 slot 范围（inclusive），slot 每次推进固定 +1。
`MITIGATION_START_SLOT` 不能大于 `MITIGATION_END_SLOT`，否则 `SlotManager` 初始化会报错。

区块文件名形如 `block_346769956.json`，slot 号直接从文件名解析，与文件内的 blockTime 无关。

### 3. 启动 server

```bash
uv run python mitigation/scripts/run.py --strategy immediate
# 或
uv run python -m mitigation.server
```

### 4. 启动 user（另一个终端）

```bash
uv run python -m mitigation.user.client
```

user 连接 server 后会收到当前 slot 广播，然后自动进入「收广播 → 发该 slot 交易 → 发 end」循环。server 处理完 `MITIGATION_END_SLOT` 后会发出 `simulation_done` 事件，user 自动断开。

## API接口

### GET /health

健康检查。返回当前 slot 号、策略、debug 模式（仅信息展示）。

### POST /sendTransaction

发送交易（BlockTransaction 完整字段）：

```json
{
  "slot": 346769956,
  "timestamp": 1749916801,
  "signatures": ["sig_abc"],
  "message": {},
  "fee": 5000,
  "preBalances": [],
  "postBalances": [],
  "instructions": [],
  "logMessages": []
}
```

`slot` 是 slot 号（从广播里拿到的值）。交易唯一标识是 `signatures[0]`。

发送 end 标识（本 slot 交易全部发完）：

```json
{
  "slot": 346769956,
  "end": true
}
```

### GET /transactions/pending

查看待处理交易（按 slot 号列出）。

### GET /transactions/sent

查看已发送给验证者的交易记录。

### GET /slot/current

查看当前 slot 号。

## Socket.IO 事件

| 事件 | 方向 | 数据 | 说明 |
|------|------|------|------|
| `new_slot` | server → user | `{slot, leader}` | 进入新 slot，user 开始发该 slot 的交易 |
| `simulation_done` | server → user | `{last_slot}` | MITIGATION_END_SLOT 处理完成，不再有 new_slot |

## 测试

```bash
uv run pytest mitigation/tests/
```
