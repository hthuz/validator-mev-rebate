# Mitigation - 交易发送策略模拟平台

## 概述

Mitigation 模块用于模拟不同交易发送策略对 sandwich 攻击的缓解效果。提供两种运行方式：

- **`python -m mitigation.sim`（推荐）**：进程内模拟，直接调用 server 与 client 的业务函数，不启动 HTTP/Socket.IO，由 sim 自己推进 slot，结束后把交易处理概况打印到终端。
- **server + client**：保留原有的 HTTP + Socket.IO 链路，用于交互式验证和联调。

两条入口共用同一份业务函数（`TransactionServer`），策略、阈值、slot 范围全部从 config 读取，CLI 不接受任何参数。

## 架构分层

```
HTTP / Socket.IO 适配层      mitigation/routes.py
                                  │ 解析请求 → 调业务函数 → 包装响应
                                  ▼
业务函数层（唯一实现）        mitigation/server.py  TransactionServer
                              ├─ transaction_manager.py  计数 / 延迟详情 / 报表
                              ├─ slot_manager.py         slot 号 → leader
                              └─ strategy/               策略 + 风险注入
                                  ▲
                                  │ 直接函数调用，无网络
进程内模拟入口                mitigation/sim.py
                              └─ user/replay.py  BlockReplaySource（区块加载，与 client 共用）
```

`TransactionServer` 的业务函数：
- `set_slot(slot)` / `submit_transaction(tx)` / `close_slot(slot)` / `advance_slot()` 驱动 slot 生命周期
- `process()` 按策略判定发送
- `get_report()` / `get_pending()` / `get_risk_summary()` / `get_health()` 返回纯 dict

`with_transport=False` 时 server 不创建 Flask/Socket.IO 对象，供 sim 使用；默认值 `True` 保持原有 server 行为。

## Slot 机制

**slot 就是 slot 号（等于 block 号，整数），跟 blockTime/timestamp 没有任何关系。** 真实链上 400ms 一个 slot，但这里是历史交易回放模拟——一个 slot 的交易处理完就跳到下一个 slot，与时间无关。

slot 范围由 `MITIGATION_START_SLOT` / `MITIGATION_END_SLOT` 决定（两端 inclusive），每次固定 +1。

**server + client 模式**由事件驱动：

```
1. server 进入新 slot（从 MITIGATION_START_SLOT 开始，每次 +1）
       ↓  Socket.IO 广播 new_slot {slot, leader}
2. user 收到广播，从区块数据取该 slot 的交易
       ↓  逐笔 POST /sendTransaction
3. server 收到交易后立即按当前策略处理
       ↓  user 发 end 标识：{"slot": slot, "end": true}
4. server 收到 end → 处理收尾交易 → 推进到下一个 slot → 回到 1
5. MITIGATION_END_SLOT 处理完后只发 simulation_done
```

**sim 模式**没有广播和 end 消息，slot 循环由 `run_simulation()` 自己的 for 循环驱动：

```
for slot in config 的 slot 范围:
    server.set_slot(slot)
    for tx in replay.transactions_for(slot):
        server.submit_transaction(tx)
    server.close_slot(slot)
```

## 配置驱动

**CLI 不接受任何参数**。所有参数从 `config/settings.py` 读取（可由 `.env` 覆盖）。

```env
MITIGATION_START_SLOT=346769956
MITIGATION_END_SLOT=346769958
BLOCK_PATH=/path/to/processed_blocks
SLOT_LEADER_PATH=/path/to/slot_leaders.csv

# immediate / delayed / risk-aware
MITIGATION_STRATEGY=risk-aware
# 仅 delayed 策略使用
MITIGATION_DELAY_SLOTS=1
```

`build_strategy_from_config()` 按 `MITIGATION_STRATEGY` 构造策略；sim、`run.py`、`python -m mitigation.server`、`start.sh` 四条路径都走这条路径。未知策略名会直接抛 `ValueError`。

## RiskAwareStrategy

server 维护一个历史 epoch 范围（默认 800-801）内各 validator 的 sandwich ratio，ratio 计算复用 `utils.sandwich.get_sandwich_ratios()`（即 `SandwichSlotNum / SlotNum`）。

判定规则：
- 任意一个历史 epoch 的 ratio **严格大于** `SANDWICH_RATIO_THRESHOLD`（默认 0.5）→ 该 validator 为高风险。
- NaN / 无数据的 validator 视为安全。
- 当前 slot 的 leader 高风险时继续持有交易；遇到安全 leader 立即发送。
- 一直高风险且达到 `MITIGATION_MAX_SKIP_SLOTS`（默认 10）时，在当前 slot 强制发送。

相关配置项：

```env
SANDWICH_HISTORY_START_EPOCH=800
SANDWICH_HISTORY_END_EPOCH=801
SANDWICH_RATIO_THRESHOLD=0.5
MITIGATION_MAX_SKIP_SLOTS=10
```

使用 risk-aware 策略时还需配置 ClickHouse（读历史 sandwich ratio）。加载失败只 warning 并视为空风险集，所以启动时务必看风险摘要——sim 会直接打印，server 模式访问 `GET /risk/summary`。

质押维度暂未接入。

## 快速开始

### 1. 安装依赖并配置

```bash
uv sync
```

`.env` 中至少包含上一节列出的 slot 范围、数据路径与策略配置。`MITIGATION_START_SLOT` 不能大于 `MITIGATION_END_SLOT`，否则 `SlotManager` 初始化会报错。

区块文件名形如 `block_346769956.json`，slot 号直接从文件名解析，与文件内的 blockTime 无关。

### 2. 跑 sim（推荐）

```bash
uv run python -m mitigation.sim
```

sim 不启动 server、不需要 client、不走网络。启动时打印风险数据概况，slot 循环跑完后打印交易处理概况，格式见下方 `GET /transactions/sent`。

### 3. 跑 server + client（兼容路径）

终端 1：
```bash
uv run python mitigation/scripts/run.py
# 或
uv run python -m mitigation.server
# 或
bash mitigation/scripts/start.sh
```

终端 2：
```bash
uv run python -m mitigation.user.client
```

user 连接 server 后会收到当前 slot 广播，自动进入「收广播 → 发该 slot 交易 → 发 end」循环。server 处理完 `MITIGATION_END_SLOT` 后会发出 `simulation_done`，user 自动断开。

## 文件结构

```
mitigation/
├── sim.py                  # 进程内模拟入口
├── server.py               # TransactionServer 业务 core
├── routes.py               # HTTP / Socket.IO 适配层
├── slot_manager.py         # Slot 管理器（slot 号 -> leader）
├── transaction_manager.py  # 交易管理器（计数 / 延迟详情 / 报表）
├── risk_store.py           # 历史 epoch sandwich ratio 存储与风险判定
├── strategy/               # 策略模块
│   ├── base_strategy.py    # 含 StrategyContext（leader 查询 + 风险存储注入）
│   ├── immediate_strategy.py
│   ├── delayed_strategy.py
│   ├── risk_aware_strategy.py
│   └── factory.py          # 按 config 构造策略
├── user/
│   ├── client.py           # 用户客户端（广播驱动）
│   └── replay.py           # BlockReplaySource（区块加载，sim 共用）
├── docx/
├── tests/
└── scripts/
```

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

查看发送概况。**未延迟的交易只累加计数，不逐笔留存；延迟交易保留完整详情。**

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
      "signature": "sig_abc",
      "slot": 10,
      "target_slot": 12,
      "delay_slots": 2,
      "validator": "validator_x",
      "leader_risky": true,
      "strategy": "RiskAwareStrategy(max_skip=10)",
      "sent_at": 1749916801.0
    }
  ]
}
```

- `total_processed`: 总共处理的交易数（含未延迟）
- `delay_stats.all`: 所有交易的延迟概况（未延迟按 0 计）
- `delay_stats.delayed`: 仅延迟交易的延迟概况
- `slot_overview.original` / `target`: 延迟交易的原 slot / 落地 slot 分布
- `slot_overview.moves`: 原 slot → 落地 slot 的迁移统计
- `delayed_records`: 延迟交易逐笔详情

### GET /risk/summary

查看历史 epoch 范围、阈值、各 epoch 加载的 validator 数量，以及高风险 validator 列表。未启用 risk-aware 策略时返回 `{"loaded": false}`。

### GET /slot/current

查看当前 slot 号。

## Socket.IO 事件

| 事件 | 方向 | 数据 | 说明 |
|------|------|------|------|
| `new_slot` | server → user | `{slot, leader}` | 进入新 slot，user 开始发该 slot 的交易 |
| `simulation_done` | server → user | `{last_slot}` | MITIGATION_END_SLOT 处理完成，不再有 new_slot |

## 测试

```bash
uv run python -m pytest tests/ mitigation/tests/
```

`test_sim.py` 覆盖 sim 与 HTTP 路径报告一致性、config slot 范围、风险跳过场景。
