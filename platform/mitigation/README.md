# Mitigation - 交易发送策略模拟平台

## 概述

Mitigation 模块用于模拟不同交易发送策略对 sandwich 攻击的缓解效果。提供两种运行方式：

- **`python -m mitigation.sim`（推荐）**：进程内模拟，直接调用 server 与 client 的业务函数，不启动 HTTP/Socket.IO，由 sim 自己推进 slot，结束后把交易处理概况打印到终端。
- **server + client**：保留原有的 HTTP + Socket.IO 链路，用于交互式验证和联调。

两条入口共用同一份业务函数（`TransactionServer`），策略、阈值、slot 范围、数据源全部从 config 读取，CLI 不接受任何参数。

## 数据源

`MITIGATION_TX_SOURCE` 决定 sim 重放什么交易：

| 值 | 数据源 | 交易类型 | 用途 |
|----|--------|----------|------|
| `block` | 区块 JSON（`BLOCK_PATH`） | `BlockTransaction`，全量原始交易 | 原有模式 |
| `victim` | ClickHouse `sandwiches_nobundle` | `SandwichTransaction`，去重后的 victim 交易 | 看 victim 能否避开恶意 leader |

slot 的 leader 一律从 ClickHouse 的 `slot_leaders` 表按 `MITIGATION_START_SLOT/END_SLOT` 范围加载，不再读本地 CSV。查询失败时降级为空 leader 集合，此时交易不会发送。

两种交易类型只依赖一个最小协议 `RoutableTransaction`（`slot` + `signature`），不需要 ABC，也不改核心业务函数。

## 架构分层

```
HTTP / Socket.IO 适配层      mitigation/routes.py
                                  │ 解析请求 → 调业务函数 → 包装响应
                                  ▼
业务函数层（唯一实现）        mitigation/server.py  TransactionServer
                              ├─ transaction_manager.py  计数 / 延迟详情 / 报表
                              ├─ slot_manager.py         slot 号 → leader（ClickHouse）
                              └─ strategy/               策略 + 风险注入
                                  ▲
                                  │ 直接函数调用，无网络
进程内模拟入口                mitigation/sim.py
                              ├─ user/replay.py       BlockReplaySource（区块 JSON）
                              ├─ user/victim_source.py VictimReplaySource（victim）
                              └─ evaluation.py        victim 三档落地评估
```

## Slot 机制

**slot 就是 slot 号（等于 block 号，整数），跟 blockTime/timestamp 没有任何关系。** 真实链上 400ms 一个 slot，但这里是历史交易回放模拟——一个 slot 的交易处理完就跳到下一个 slot，与时间无关。

slot 范围由 `MITIGATION_START_SLOT` / `MITIGATION_END_SLOT` 决定（两端 inclusive），每次固定 +1。

**sim 模式**：

```
for slot in config 的 slot 范围:
    server.set_slot(slot)
    for tx in replay.transactions_for(slot):
        server.submit_transaction(tx)
    server.close_slot(slot)
```

**server + client 模式**由事件驱动，见下方 Socket.IO 事件说明。

## 配置驱动

**CLI 不接受任何参数**。所有参数从 `config/settings.py` 读取（可由 `.env` 覆盖）。

```env
MITIGATION_START_SLOT=346769956
MITIGATION_END_SLOT=346769958
BLOCK_PATH=/path/to/processed_blocks

# block / victim
MITIGATION_TX_SOURCE=victim
# victim 模式取哪个 epoch 的交易
MITIGATION_REPLAY_EPOCH=802

# immediate / delayed / risk-aware
MITIGATION_STRATEGY=risk-aware
MITIGATION_DELAY_SLOTS=1
```

`MITIGATION_START_SLOT` 不能大于 `MITIGATION_END_SLOT`，否则 `SlotManager` 初始化报错。要按整个 epoch 重放 victim，把 slot 范围设成该 epoch 的边界（epoch N 的 slot 范围是 `N*432000 ~ (N+1)*432000-1`）。

## RiskAwareStrategy

server 维护一个历史 epoch 范围（默认 800-801）内各 validator 的 sandwich ratio，ratio 计算复用 `utils.sandwich.get_sandwich_ratios()`（`SandwichSlotNum / SlotNum`）。

判定规则：
- 任意一个历史 epoch 的 ratio **严格大于** `SANDWICH_RATIO_THRESHOLD`（默认 0.5）→ 高风险。
- NaN / 无数据视为安全。
- 当前 leader 高风险时继续持有，遇到安全 leader 立即发送；达到 `MITIGATION_MAX_SKIP_SLOTS`（默认 10）强制发送。

```env
SANDWICH_HISTORY_START_EPOCH=800
SANDWICH_HISTORY_END_EPOCH=801
SANDWICH_RATIO_THRESHOLD=0.5
MITIGATION_MAX_SKIP_SLOTS=10
```

启动先看风险摘要：sim 直接打印，server 模式访问 `GET /risk/summary`。加载失败只会 warning 并退化为空风险集。质押维度暂未接入。

## Victim 落地评估

策略用历史 epoch（800-801）做决策，效果用**当前 replay epoch（802）自身的观测**做标签，时间上是干净的留出验证。三档标签：

| 标签 | 判定 | 含义 |
|------|------|------|
| `confirmed_sandwiched` | 落地 slot == 原始 slot | 数据里确认被 sandwich |
| `leader_observed_sandwich` | 换了 slot，但落地 leader 在本 epoch 观测到过 victim sandwich | 假定仍有风险 |
| `clean_leader` | 换了 slot，落地 leader 没有观测到 sandwich | 假定规避成功 |

这是代理指标，不是反事实真值：victim 改道后不在历史区块里，无法观测搜索者是否会重新攻击它。`immediate` 策略下所有 victim 都留在原 slot，`confirmed_sandwiched` 应为 100%，可作为基线。

victim 按 signature 去重（同一 victim 可能出现在多个 sandwich 里）；epoch 802 实测 33,560 行 → 32,879 笔唯一 victim。

## 快速开始

```bash
uv sync
uv run python -m mitigation.sim
```

server + client 兼容路径：

终端 1：
```bash
uv run python mitigation/scripts/run.py
# 或 uv run python -m mitigation.server / bash mitigation/scripts/start.sh
```

终端 2：
```bash
uv run python -m mitigation.user.client
```

## 文件结构

```
mitigation/
├── sim.py                  # 进程内模拟入口
├── server.py               # TransactionServer 业务 core
├── routes.py               # HTTP / Socket.IO 适配层
├── slot_manager.py         # slot 号 -> leader（ClickHouse）
├── transaction_manager.py  # 交易计数 / 延迟详情 / 报表
├── risk_store.py           # 历史 epoch sandwich ratio
├── evaluation.py           # victim 三档落地评估
├── strategy/               # immediate / delayed / risk-aware + factory
├── user/
│   ├── client.py           # 用户客户端（广播驱动）
│   ├── replay.py           # BlockReplaySource
│   └── victim_source.py    # VictimReplaySource
├── docx/
├── tests/
└── scripts/
```

## API接口

### GET /health
当前 slot、策略、debug 模式。

### POST /sendTransaction
发送 `BlockTransaction` 完整字段（`slot` 是 slot 号，唯一标识为 `signatures[0]`），或发送 `{"slot": N, "end": true}` 标识本 slot 发完。

### GET /transactions/pending
按 slot 列出待处理交易。

### GET /transactions/sent
返回 `get_report()`：未延迟交易只计数，延迟交易留详情。

```json
{
  "total_processed": 6,
  "immediate_count": 3,
  "delayed_count": 3,
  "delay_stats": {
    "all": {"count": 6, "sum": 8, "avg": 1.333333, "min": 0, "max": 5},
    "delayed": {"count": 3, "sum": 8, "avg": 2.666667, "min": 1, "max": 5}
  },
  "slot_overview": {
    "original": {"10": 1, "11": 1, "12": 1},
    "target": {"12": 1, "13": 1, "16": 1},
    "moves": [{"original_slot": 10, "target_slot": 12, "count": 1}]
  },
  "delayed_records": [
    {
      "signature": "sig_abc", "slot": 10, "target_slot": 12,
      "delay_slots": 2, "validator": "validator_x",
      "leader_risky": true, "strategy": "RiskAwareStrategy(max_skip=10)",
      "sent_at": 1749916801.0
    }
  ]
}
```

### GET /risk/summary
历史 epoch 范围、阈值、各 epoch 加载数量、高风险 validator 列表。

### GET /slot/current
当前 slot 号。

## Socket.IO 事件

| 事件 | 方向 | 数据 | 说明 |
|------|------|------|------|
| `new_slot` | server → user | `{slot, leader}` | 进入新 slot，user 开始发该 slot 的交易 |
| `simulation_done` | server → user | `{last_slot}` | END slot 处理完成，不再有 new_slot |

## 测试

```bash
uv run python -m pytest tests/ mitigation/tests/
```
