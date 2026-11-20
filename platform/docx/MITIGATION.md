# Solana 验证者 MEV 交易分发缓解策略（Mitigation）整理

> 本文整理 `platform/mitigation` 模块的问题背景、当前策略、实现方案与验证效果，基于仓库实际代码与 epoch 800–802 的真实数据。
> 记录日期：2026-11-20。

---

## 0. 一句话总结

在交易发出前先看一眼"下一个出块的 leader 历史上是不是爱做 sandwich 的验证者"：是就攥着交易等下一个安全 leader，等太久（超过窗口）就强制发出。整套流程在进程内模拟器里跑，用 epoch 802 的 victim 交易重放，从"验证者风险"和"sandwich 落地结果"两个视角衡量效果。

---

## 1. 问题背景

### 1.1 MEV Sandwich 与验证者

在 Solana 上，搜索者（searcher）检测到一笔待执行的 victim 交易后，会在它前后插入 frontrun / backrun 交易，夹住 victim 从中套利。这类攻击能否成立，最终取决于**打包区块的 leader（验证者）**：leader 决定了区块里交易的排序，也在很大程度上决定了哪些交易能进块。

`utils.sandwich` 基于 ClickHouse 的 `sandwiches_nobundle` 表 + `slot_leaders` 表，为每个验证者计算一个 sandwich ratio：

```
sandwich_ratio(identity) = 观测到 sandwich 的出块 slot 数 / 该验证者的出块 slot 数
```

经验数据表明这个比例在验证者之间差异很大：少数验证者频繁参与 sandwich，多数验证者几乎没有。这就给了一个可操作的切入点——**如果交易发送方能避开高风险 leader 的出块 slot，就能降低被 sandwich 的概率**。

### 1.2 为什么用"交易分发时机"来缓解

我们控制不了验证者，也控制不了搜索者，但能控制**自己的交易什么时候发、由哪个 slot 的 leader 打包**。原生的做法是交易一就绪就发（immediate），此时 leader 是谁完全听天由命。缓解策略的核心自由度就是：**延迟发送，直到遇到一个"安全"的 leader**。

### 1.3 目标

- 打通一条端到端可运行的模拟链路：重放 epoch 802 的 victim 交易，用不同策略分发，观察落地结果。
- 用历史 epoch（800/801）的风险数据做**决策**，用 replay epoch（802）自身的观测做**评估**（时间上干净的留出验证）。
- 指标要能回答两个视角的问题：
  - 验证者视角：交易最终落到高风险 leader 手里的有多少？
  - 交易视角：victim 换到别的 slot 后，是否还观测到 sandwich？

---

## 2. 整体方案

### 2.1 核心思路

```
历史 epoch (800/801) ─┬─ sandwich ratio ─────┐
                      └─ Marinade 质押占比 ─┴─> ValidatorRiskStore（并集）─> 判定 leader 是否 risky
                                                              │
victim 交易重放 ──> slot 循环 ──> 当前 slot 的 leader ──> 分发策略决定发/等/强制发
                                                              │
                                                    落地 slot / validator 记录
                                                              │
                                    ┌─────────────────────────┴────────────────────────┐
                                    │                                                  │
                          验证者风险暴露评估                              三档 sandwich 落地评估
                  (802 自身 ratio ∪ Marinade 风险集合)               (用 802 观测到 sandwich 的 leader)
```

关键设计：**策略输入和评估标签用的是不同 epoch 的风险数据**，刻意不做成同一批，避免"用自己的标签证明自己有效"的循环论证。两者的**判定维度一致**（sandwich ratio ∪ Marinade 质押），但数据取自不同 epoch：策略看历史的 800/801 做决策，评估看 replay epoch 802 自身做留出验证。三档落地评估则是第三个口径——用"802 中观测到 victim sandwich 的 leader"这个更宽的观测集合。

### 2.2 架构分层

| 层 | 文件 | 职责 |
|---|---|---|
| 业务核心 | `mitigation/server.py` | `TransactionServer` 提供 `set_slot` / `submit_transaction` / `close_slot` / `advance_slot` / `process` 等纯业务函数，不碰 HTTP |
| 传输适配 | `mitigation/routes.py` | HTTP 路由 + Socket.IO 事件，只做"解包 → 调业务函数 → 包装响应" |
| 进程内模拟 | `mitigation/sim.py` | 自己循环推进 slot，直接调业务函数，无网络、无 client 同步 |
| 策略 | `mitigation/strategy/` | `immediate` / `delayed` / `risk-aware` 三种，由 `factory.py` 按 config 构造 |
| 风险存储 | `mitigation/risk_store.py` | 加载历史 epoch 的 sandwich ratio 与 Marinade 质押占比，两条判定线取并集判定 risky |
| Marinade 质押 | `mitigation/marinade_store.py` | 复用 `utils.stake_reader` 加载 Marinade 质押占比，`voteAccountPubkey` → `identityPubkey` 映射 |
| slot 管理 | `mitigation/slot_manager.py` | 从 ClickHouse `slot_leaders` 表加载 slot → leader |
| 数据源 | `mitigation/user/replay.py`、`user/victim_source.py` | `BlockReplaySource`（全量区块交易）/ `VictimReplaySource`（victim 交易），接口一致可切换 |
| 评估 | `mitigation/evaluation.py` | 验证者风险暴露 + 三档落地标签 |
| 基础设施 | `utils/ch.py`、`utils/sandwich.py` | ClickHouse 查询封装、ratio / victim 查询 |

**双入口兼容**：`run.py`、`python -m mitigation.server`、`start.sh` 三条老路径照旧走 HTTP + Socket.IO；`python -m mitigation.sim` 是新增的进程内入口。两者调用**同一套业务函数**，有测试保证两条路径产出的 report 一致。

### 2.3 数据链路

1. `sim` 从 config 读 `MITIGATION_TX_SOURCE` 决定数据源，读 `MITIGATION_START_SLOT/END_SLOT` 决定 slot 范围。
2. 每个 slot：`set_slot(slot)` → 取出该 slot 的交易逐笔 `submit_transaction` → `close_slot(slot)`。
3. `process()` 里遍历待发送交易，调策略的 `should_send(tx, current_slot)`；决定发送就生成 `TransactionRecord` 计入统计。
4. slot 结束后打印报表 + 两套评估。

---

## 3. 策略设计

### 3.1 三种发送策略

| 策略 | 行为 | 配置项 |
|---|---|---|
| `immediate` | 交易就绪即发给当前 slot 的 leader（基线） | — |
| `delayed` | 固定延迟 N 个 slot 再发 | `MITIGATION_DELAY_SLOTS`（默认 1） |
| `risk-aware` | 当前 leader 高风险则持有，遇到安全 leader 立即发送；达到最大窗口强制发送 | `MITIGATION_MAX_SKIP_SLOTS`（默认 10） |

策略通过 config 的 `MITIGATION_STRATEGY` 选择（默认 `immediate`），**没有 CLI 参数**——策略、阈值、epoch 范围、窗口全部走配置，保证三条启动路径行为一致。

### 3.2 RiskAwareStrategy 判定逻辑

`should_send(tx, current_slot)`：

1. `current_slot < tx.slot`：不发（交易还没"到"）。
2. 无上下文（未经 server 注入）：退化为立即发送。
3. 当前 leader 无风险数据或不 risky：发送。
4. 当前 leader risky：`(current_slot - tx.slot) >= max_skip_slots` 才发送（强制兜底），否则继续持有。

风险判定规则（`ValidatorRiskStore`）——两条判定线取并集：

- 加载范围：`SANDWICH_HISTORY_START_EPOCH ~ END_EPOCH`（默认 800–801，两端 inclusive）每个 epoch 的数据。
- **判定线 1（sandwich）**：**任意一个** epoch 的 `sandwich_ratio` 严格 > `SANDWICH_RATIO_THRESHOLD`（默认 0.5）即 risky。
- **判定线 2（Marinade 质押）**：**任意一个** epoch 的 `m_activated_stake / activatedStake` 严格 > `MARINADE_STAKE_THRESHOLD`（默认 0.5）即 risky。质押数据复用 `utils.stake_reader.Reader`，其 dict 的 key 是 `voteAccountPubkey`，需映射到 `identityPubkey` 才能与 slot leader 对上。
- **并集**：`is_risky()` 对两条线的并集生效；`summary()` 用 `sandwich_risky_validators` / `marinade_risky_validators` / `both_risky_validators` 分列来源，`risky_validators` 是并集。
- NaN / 缺失视为安全；单个 epoch 加载失败只 warning、不中断，降级为空集合。

> 注意"严格大于"：恰好等于阈值不算高风险。实测两条线高度重合：sandwich 命中的 31 个 validator 全部也在 Marinade 命中集合内，并集几乎由 Marinade 线主导（1195 个）。若嫌范围过宽，调高 `MARINADE_STAKE_THRESHOLD`。

### 3.3 两层 epoch 视角（关键）

| 用途 | 数据来源 | 风险集合口径 | 规模（实测） |
|---|---|---|---|
| **决策**（策略跳过谁） | 历史 epoch 800 / 801 | sandwich ratio > 0.5 **或** Marinade 质押占比 > 0.5 | sandwich 命中 31、Marinade 命中 1195、并集 1195 |
| **评估 A**（验证者视角） | replay epoch 802 自身 | sandwich ratio > 阈值 **或** Marinade 质押占比 > 阈值（与策略同口径，取并集） | 当前配置（sandwich 1.1 / Marinade 0.5）：sandwich 0、Marinade 70、**并集 70**；默认 0.5/0.5 下 ratio 口径为 32 个 |
| **评估 B**（交易视角） | replay epoch 802 自身 | 观测到 victim sandwich 的 leader | **817 个** |

策略只认历史（800/801）；评估用 802 自身。所以会出现"策略认为安全、但 802 口径算 risky"的情况——这正是留出验证要暴露的差异，不是 bug。评估 A 和评估 B 也不等同：一个是 ratio/质押超阈值（几十个量级），一个是观测到过 victim sandwich（817 个，口径宽得多）。评估 A 内部再按来源分 `sandwich` / `marinade` / `both`，三者可重叠（不是互斥划分）。

---

## 4. 关键实现

### 4.1 数据源：为什么用 victim 交易重放

原始方案是重放 block JSON 里的全量交易（`BlockTransaction`），但收集一整个 epoch 的原始交易很耗时。改为直接从 `sandwiches_nobundle` 取 `type='victim'` 的交易重放——只看 victim 在不同策略下是否还落到恶意 leader 手里。

- 查询：`get_victim_transactions(epoch)`，按 `epoch*432000 ~ (epoch+1)*432000-1` 取范围。
- 同一 victim 可能出现在多个 sandwich 里，按 `signature` 去重（Python 侧去重）。
- epoch 802 实测：33,560 行 → **32,879 笔唯一 victim**。
- 新类型 `SandwichTransaction`：字段对应 CK 列，没有 `BlockTransaction` 需要的 fee / balances / instructions / message，所以是独立类型而非子类。
- 两者都满足 `RoutableTransaction` Protocol（`slot` + `signature`），策略与 server 只依赖这个最小接口，核心逻辑无需区分交易类型。**没有引入 ABC**（没有共享行为，只有共享形状）。

### 4.2 slot leader 从 ClickHouse 加载

`SlotManager` 已删除本地 CSV 读取，按 config 的 slot 范围查 `slot_leaders` 表：

```sql
SELECT slot, leader FROM slot_leaders WHERE slot BETWEEN <start> AND <end>
```

- 查询失败降级为空字典，此时 `process()` 跳过无 leader 的 slot。
- 支持注入 `leaders` 便于测试。
- 对比：本地 `slot_leaders_small.csv` 只有 2537 个 slot，而一个 epoch 有 432000 个 slot，无法支撑整 epoch 重放；CK 表是完整的。

### 4.3 ClickHouse 查询封装（踩坑记录）

`utils/ch.py` 的 `query_clickhouse(sql)` 统一走：

```
requests.post(url, params={'database': ..., 'query': sql + ' FORMAT JSONEachRow'}, auth=...)
```

原因：本环境 8123 端点只接受 URL 上的 `query` 参数；`clickhouse_connect==1.13.0` 用 POST body 传 query 会拿到 404，导致之前 risk store 加载 0 个 validator。改用这个封装后 ratio / victim / slot leader 三类查询共用，问题消失。

### 4.4 进程内 sim

`python -m mitigation.sim`，无 CLI 参数。启动时打印风险摘要（注意确认加载到了 validator，否则风险集为空、策略不会跳过），结束时打印报表。代码骨架：

```python
server = create_server(strategy=build_strategy_from_config(), with_transport=False)
replay = VictimReplaySource(MITIGATION_REPLAY_EPOCH)   # 或 BlockReplaySource
for slot in server.slot_manager.get_available_slots():
    server.set_slot(slot)
    for tx in replay.transactions_for(slot):
        server.submit_transaction(tx, slot)
    server.close_slot(slot)
print(server.get_report())
```

### 4.5 发送记录与统计口径

`TransactionManager` 的规则是**未延迟只计数、延迟才留详情**：

- `_sent_count`：总发送笔数；`_sent_delay_sum`：延迟总和。
- `_sent_delayed`：只存 `delay_slots > 0` 的逐笔 `TransactionRecord`（含签名、原 slot、落地 slot、延迟量、leader、是否历史 risky）。
- `_landed_validators`：**所有**发送交易按落地 validator 的聚合计数（immediate 也计入），专门为验证者视角评估准备——否则 immediate 交易的落地 leader 信息就丢了。

`GET /transactions/sent` 返回：`total_processed` / `immediate_count` / `delayed_count`、`delay_stats.all` 与 `delay_stats.delayed`（count/sum/avg/min/max）、`slot_overview`（原 slot 分布、落地 slot 分布、迁移明细）、`delayed_records`（延迟逐笔）、`landed_validators`（按 validator 聚合）。空数据时统计值为 0 而非 null。

---

## 5. 评估指标体系

### 5.1 验证者视角：落地 leader 风险暴露

`evaluate_leader_risk_exposure(report, risky_validators, marinade_risky=None, sandwich_risky=None)`，统计对象是**全部已发送交易**：

| 字段 | 含义 |
|---|---|
| `total_sent` | 已发送交易总数 |
| `on_risky_leader` | 落地 leader 在评估 epoch risky **并集**里的笔数 |
| `on_safe_leader` | 落地 leader 不在并集里的笔数 |
| `ratios` | 上述两项占比 |
| `risky_validators_in_epoch` | 评估 epoch 的 risky 并集大小 |
| `by_source`（可选） | 传入分列集合时出现，按 `marinade` / `sandwich` / `both` 给出 `count` / `ratio` / `risky_validators`。三者可重叠，**不是互斥划分** |

risky 集合由 `load_risky_validators(epoch)` 构造：读该 epoch 的 sandwich ratio 与 Marinade 质押占比，各自严格 > 对应阈值（`SANDWICH_RATIO_THRESHOLD` / `MARINADE_STAKE_THRESHOLD`）后取并集；NaN/缺失安全，单个数据源加载失败只 warning 并退化为空集合。需要展示来源时用 `load_risky_validators_detail(epoch)`，它返回分列的集合与计数；`load_risky_validators()` 是只取并集的薄封装。两者都支持注入 `loader` / `marinade_loader` 便于测试。

sim 在 victim 模式下会打印「评估口径 risky 来源」：sandwich / marinade / both / union 的数量，以及 Marinade risky validator 的完整 identity 列表——即"哪些是 marinade 质押导致的 risky"。

### 5.2 逐笔 victim 三档落地标签

`evaluate_victim_outcomes(...)`，对象是每个已提交的 victim，按落地结果分三档：

| 标签 | 判定 | 含义 |
|---|---|---|
| `confirmed_sandwiched` | 落地 slot == 原始 slot | 数据里确认被 sandwich（未改道） |
| `leader_observed_sandwich` | 换了 slot，但落地 leader 在 802 观测到 victim sandwich 的集合里 | 假定仍有风险 |
| `clean_leader` | 换了 slot，落地 leader 没观测到 sandwich | 假定规避成功 |

`immediate` 策略下所有 victim 都留在原 slot，`confirmed_sandwiched` 应为 100%，可作基线。

### 5.3 局限：这是代理指标，不是反事实真值

victim 改道到另一个 slot 后，**历史区块里从来没有这笔交易**，`sandwiches_nobundle` 里不会有一行说"它在 slot Y 被 sandwich 了"。要回答"Y 上会不会被 sandwich"，得先模拟搜索者看到交易后会不会 frontrun、leader 会不会打包——那是在建攻击者行为模型，不是打标签。因此两套指标都是**代理指标**：

- 验证者视角假设"落到 risky leader ≈ 有风险"；
- 三档标签假设"换到观测到过 sandwich 的 leader ≈ 仍有风险"。

它们能衡量策略是否成功避开"历史上/本 epoch 里表现恶劣的验证者"，但不能证明"真实世界一定没被 sandwich"。

---

## 6. 配置项

全部在 `config/settings.py`，通过 `.env` 覆盖（`MITIGATION_STRATEGY` 默认 `immediate`，`.env.example` 建议设为 `risk-aware`）：

| 配置 | 默认值 | 说明 |
|---|---|---|
| `MITIGATION_TX_SOURCE` | `block` | `block` / `victim`，replay 数据源 |
| `MITIGATION_REPLAY_EPOCH` | `802` | victim 模式取哪个 epoch 的 victim 交易 |
| `MITIGATION_START_SLOT` / `END_SLOT` | `346769956` / 同左 | slot 范围（inclusive，slot == block），每次 +1 |
| `MITIGATION_STRATEGY` | `immediate` | `immediate` / `delayed` / `risk-aware` |
| `MITIGATION_DELAY_SLOTS` | `1` | delayed 策略的延迟 slot 数 |
| `SANDWICH_HISTORY_START_EPOCH` / `END_EPOCH` | `800` / `801` | 策略输入的历史 epoch 范围 |
| `SANDWICH_RATIO_THRESHOLD` | `0.5` | ratio 严格超过即判 risky |
| `MARINADE_STAKE_THRESHOLD` | `0.5` | Marinade 质押占比严格超过即判 risky（与 sandwich 取并集） |
| `MITIGATION_MAX_SKIP_SLOTS` | `10` | risk-aware 连续跳过的最大 slot 窗口 |

> 注意：victim 模式的 slot 范围仍由 `MITIGATION_START_SLOT/END_SLOT` 控制，**不会自动扩到整个 epoch**。要跑完整 epoch 802，需把这两个值设为 `346464000` / `346895999`。

---

## 7. 具体效果

### 7.1 测试

```
python -m pytest tests/ mitigation/tests/ -q   →  57 passed
```

覆盖：阈值边界（0.5 不算、0.6 算）、NaN / 缺失、epoch 范围、连续跳过、窗口强制发送、无上下文回退、sim 与 HTTP report 一致性、victim 去重、三档标签、验证者风险暴露统计、sandwich/Marinade 并集判定与来源分列、评估侧 risky 集合的 Marinade 并集与 `by_source` 细分等。

> 必须用 `python -m pytest`（把项目根加入 sys.path）。裸 `pytest` 在 `tests/test_block_tx_loader.py` 的 `from type import ...` 上报 collection 错误——这是既有问题，非本次引入。

### 7.2 真实数据冒烟对比

窗口：epoch 802 的 slot `346464132–346464160`，4 笔 victim 交易。当前配置 `SANDWICH_RATIO_THRESHOLD=1.1`（等价关掉 sandwich 线）、`MARINADE_STAKE_THRESHOLD=0.5`，评估口径并集 70 个（全部来自 Marinade）。

| 策略 | total_sent | on_risky_leader（基于 802 ratio ∪ Marinade） | on_safe_leader | risky 占比 |
|---|---|---|---|---|
| `immediate` | 4 | 4 | 0 | **100%** |
| `risk-aware` | 4（全部延迟） | 0 | 4 | **0%** |

`by_source` 细分：`immediate` 的 4 笔全部命中 `marinade`（count=4, ratio=1.0），`sandwich` 和 `both` 均为 0；`risk-aware` 三项均为 0。风险暴露从 100% 降到 0%。

`risk-aware` 的 4 笔延迟交易，`delay_stats` 为 sum 12 / avg 3.0 / min 1 / max 4，即平均多等 3 个 slot 就全部避开了评估口径认定的 risky leader。

补充观察：这 4 笔 `risk-aware` 交易在三档评估里全部落在 `leader_observed_sandwich`（交易视角认为仍有风险）。原因是三档用的集合是"802 里观测到 victim sandwich 的 817 个 leader"，口径远宽于评估并集的 70 个。两套指标不矛盾，只是回答不同问题——这正是保留两套的原因。

### 7.3 关键发现

1. **历史风险 ≠ 当前 epoch 风险**：800/801 判安全的 leader，在 802 自身口径下可能是 risky；策略按历史躲，评估按当前看，差异是预期内的。
2. **延迟成本很低、收益明显**：小窗口内平均延迟 3 个 slot 即可把 risky 暴露从 100% 降到 0%。
3. **数据链路要先看概况**：sim 启动会打印风险概况；如果 epoch 加载 0 个 validator（CH 不通或映射问题），策略实际不会跳过任何 leader，此时报表会"看起来正常但无效"。

---

## 8. 运行方式

```bash
# 进程内模拟（推荐，快）
uv run python -m mitigation.sim

# 跑完整 epoch 802（需覆盖 slot 范围）
MITIGATION_START_SLOT=346464000 MITIGATION_END_SLOT=346895999 \
uv run python -m mitigation.sim

# 原有 server + client 路径不变
uv run python mitigation/scripts/run.py
# 或
bash mitigation/scripts/start.sh
```

切策略：改 `.env` 的 `MITIGATION_STRATEGY=risk-aware`，启动命令固定不变。本地测试环境需带 `HOME=/home/autentico UV_CACHE_DIR=/home/autentico/.cache/uv`。

---

## 9. 局限与后续演进

| 方向 | 现状 | 后续 |
|---|---|---|
| 质押维度（Marinade） | 已接入：`m_activated_stake / activatedStake` 超阈值即 risky，与 sandwich 取并集 | 口径是单一协议占比；可扩展到 Jito / 自质押 / 多协议加权，或改用 `smap` 的协议占比口径 |
| 质押 key 映射 | `voteAccountPubkey` → `identityPubkey`，已封装在 `marinade_store.py` | — |
| 风险聚合 | 任一 epoch 超阈值即可 | 可考虑多 epoch 平均 / 加权 / EWMA |
| 实时反馈 | 静态历史数据 | 当前 epoch 内的观测能否实时拿到待确认；拿到后可做在线更新 |
| 评估口径 | 代理指标 | 严格反事实标签数据本身回答不了（见 5.3），要更准需建攻击者行为模型 |
| 无历史数据 validator | 视为安全 | 冷启动群体可能整批漏过，需要单独策略 |
| 规模 | 进程内 sim | 整 epoch 432000 slot，需关注运行时间与 CH 查询耗时 |

---

## 10. 文件索引

| 文件 | 说明 |
|---|---|
| `config/settings.py` | 所有策略 / 数据 / 阈值配置 |
| `utils/sandwich.py` | `get_sandwich_info` / `get_sandwich_ratios` / `get_victim_transactions` |
| `utils/ch.py` | ClickHouse 查询封装 |
| `mitigation/risk_store.py` | 历史 sandwich ratio + Marinade 质押存储，并集 risky 判定 |
| `mitigation/marinade_store.py` | Marinade 质押占比加载（`utils.stake_reader` + identity 映射） |
| `utils/stake_reader.py` | stake 数据 Reader（本轮补了 `ValidatorMarinade` 的导出） |
| `mitigation/slot_manager.py` | slot leader（ClickHouse `slot_leaders`） |
| `mitigation/strategy/` | 三种策略 + `base_strategy.py`（`StrategyContext`）+ `factory.py` |
| `mitigation/server.py` / `routes.py` | 业务 core / 传输适配 |
| `mitigation/sim.py` | 进程内模拟入口 |
| `mitigation/user/replay.py` / `victim_source.py` | 两种 replay 数据源 |
| `mitigation/transaction_manager.py` | 发送统计与落地 validator 计数 |
| `mitigation/evaluation.py` | 验证者风险暴露 + 三档落地标签 |
| `mitigation/tests/` | 全部相关测试 |
| `mitigation/README.md` | 使用说明与 API 文档 |
