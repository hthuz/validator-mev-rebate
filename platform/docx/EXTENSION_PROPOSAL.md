# Solana 三明治缓解策略扩展方案

> 面向毕业论文的工程化扩展提案。基于 `platform/mitigation` 现有代码、epoch 800–802 实测数据、Solana MEV 生态调研。
> 记录日期：2026-11-20。

---

## 0. 背景与问题

当前 `mitigation/strategy/risk_aware_strategy.py` 的决策只有一个信号：`ValidatorRiskStore.is_risky(leader)` = 历史 epoch（800/801）任一 `sandwich_ratio` 严格 > 0.5。该集合约 31 个验证者。策略只把 victim 从这 31 个 leader 挪走。

实测（epoch 800-801 决策 → 802 评估）：0.98 高 ratio 的 victim 落入非恶意验证者（合谋型缓解有效），但 **25838/32879 笔仍留在原 slot**——这些是不依赖"高 ratio 恶意验证者"的夹击子类型，leader 在策略看来"不 risky"，交易未被挪走。

### 0.1 根因：单一标量信号只能覆盖一个子类型

Solana 上的夹击至少有四类，只有第一类能被现有 ratio 信号抓到：

| 子类型 | 数据特征（库里已有字段） | ratio 信号能否抓到 |
|---|---|---|
| 验证者合谋夹 | leader ratio 高 | ✅ 能（现策略） |
| Jito bundle 夹 | `inbundle=true` | ❌ leader 只是跑 Jito（~92% 质押），ratio 不一定高 |
| 跨块/宽夹（wide sandwich） | `crossBlock=true` | ❌ front/back 落在不同 slot、不同 leader，单 leader 框架不适用 |
| 程序/机器人集中夹 | `programs` 命中 Vpe、`signerSame` | ❌ 由程序/机器人主导，leader 是 incidental |

### 0.2 已采集但未被 mitigation 使用的信号

`SandwichTransaction`：`inbundle` / `crossBlock` / `signerSame` / `oppositeVictim` / `programs` / `fromDiff` / `toDiff` / `fromAmount` / `toAmount` / `fromToken` / `toToken`。
`Validator`：`s_is_jito` / `activatedStake` / `self_stake()` / `skipRate` / `commission` / `stake_gini()` / 地理信息（lat/lon/asn）。
`Reader`（`utils/stake_reader.py`）：可批量加载一个 epoch 全部验证者属性。

→ 这些就是扩展空间。下面所有方案都是把这些已采集信号接入 `mitigation`，不引入新数据采集成本。

---

## 1. Solana MEV 生态关键事实（支撑方案设计）

1. **Jito 占 ~92% 质押**，bundle（≤5 笔原子执行 + tip）是主 MEV 通道。`s_is_jito` 字段库里已有。
2. **2024-03 Jito 关闭公共 mempool 后，夹击转移到私有/许可 mempool**（DeezNode 等），未消失。"等安全 leader"对私有 mempool 夹无效。
3. **跨块宽夹**：2025 研究 23 个验证者做 wide sandwich，front/back 分跨区块，单块检测失效，月抽 30k–60k SOL，2025-01 峰值 87k SOL。映射 `crossBlock` 字段。
4. **Vpe 单一程序**占 Solana 近一半夹击量（30 天 155 万次、88.9% 成功率）。映射 `programs`。
5. **机器人幂律集中**：top 3 bot 占 60%+，E6Y 单家 42%。映射 `signerSame`。
6. **bloXroute 2025-10 上线 leader-aware scoring**：给 leader 实时打分、动态改路由——本质是本平台 risk-aware 思路的商业黑盒版。学术价值：做可解释、可复现的版本。
7. **Solana Foundation / JitoSOL 用质押手段惩罚夹击运营者**——质押维度是真实治理杠杆。

---

## 2. 扩展框架总览

```
Leader 风险向量                Victim 吸引力向量(新增)
(ratio/crossBlock_ratio/        (fromAmount/slippage/流动性/
 inbundle_ratio/is_jito/        programs/signerSame/
 stake/skipRate/gini)            fromDiff/toDiff)
            │                          │
            └─────────┬────────────────┘
                      ▼
            子类型分类器(新增)
   validator-collusion / bundle / cross-block / program-concentrated
                      │
       ┌────────┬─────┴──────┬───────────┐
       ▼        ▼            ▼           ▼
   RiskAware  CrossBlockHold BundleProtect PassThrough
   (delay)    (逃出back-run) (不可观测/拆单)(低风险直发)
       └────────┴─────┬──────┴───────────┘
                      ▼
            成本均衡 EV 门控(新增)
   E[夹击损失|现在发] vs E[延迟成本]+E[损失|之后发]
                      │
                      ▼
            立即发 / 短延迟 / 长持有 / 跳过保护
```

全部兼容现有 `SendStrategy` 接口与 `StrategyContext`，是增量而非重写。

---

## 3. 六个具体扩展

### 扩展 1：多信号 Leader 风险向量 + EWMA（替换标量 ratio）

- **动机**：`is_risky` 从 bool 升级为特征向量，用上 `crossBlock_ratio` / `inbundle_ratio` / `is_jito` / `activatedStake` / `skipRate` / `stake_gini`。聚合从"任一 epoch 超阈值"改为 EWMA 加权（解决 MITIGATION.md 第 9 节自列的"风险聚合可改进"项）。
- **落点**：
  - `mitigation/risk_store.py` 新增 `MultiSignalRiskStore`（保留旧 `ValidatorRiskStore` 作 baseline，便于对比实验）。
  - `utils/sandwich.py` 的 `get_sandwich_info` 已按 `consecutive`/`backrun` 过滤，扩展查询带 `crossBlock`/`inbundle` 分组。
  - 质押维度走 `utils/stake_reader.py` 的 `Reader`，先做 `voteAccountPubkey → identity` 映射（`labels.get_validator_name` 已具备）。
- **工作量**：中（CH 查询扩展 + 映射 + EWMA）。
- **评估对照**：向量版 vs 标量版的 `on_risky_leader` 占比与子类型命中率。

### 扩展 2：Victim 吸引力评分器（直接打 25838 缺口）

- **动机**：25838 笔未挪走，是因为 leader"不 risky"，但 victim 本身吸引人（大单/高滑点/低流动性/命中 Vpe）。吸引力高的 victim 无论 leader 是谁都该保护；低的直发省延迟。这是当前框架完全没有的维度，直接攻击 25838 缺口。
- **落点**：
  - 新 `mitigation/victim_scorer.py`，输入 `SandwichTransaction`（`fromAmount`/`toAmount`/`fromDiff`/`toDiff`/`fromToken`/`toToken`/`programs`/`signerSame`），输出 `[0,1]` 吸引力分。
  - 全部字段已存在于 `SandwichTransaction`。token 流动性先用 `fromAmount` 近似，后续接链上数据。
- **工作量**：中（打分函数 + 用 802 自身 victim 的 `fromDiff` 分布标定阈值）。
- **评估**：吸引力分 vs 实际被夹率校准曲线（证明打分真能预测被夹）。

### 扩展 3：子类型感知策略集成（SubtypeAwareStrategy）

- **动机**：不同子类型需要不同防御，单一 delay 只对合谋型有效。这是论文"算法即故事"的核心。
- **落点**：
  - 新 `mitigation/strategy/subtype_aware_strategy.py`，`should_send` 先判子类型再分发到四个子策略，复用 `SendStrategy` 基类与 `StrategyContext`。
  - 子策略可先做简化版：RiskAware（现有）/ CrossBlockHold（扩展5）/ BundleProtect（扩展简化）/ PassThrough。
- **工作量**：中高（框架主干，但子策略可渐进）。
- **评估**：按子类型分别报缓解效果，证明"原来抓不到的现在抓到了"。

### 扩展 4：成本均衡 EV 门控（替换硬性 max_skip）

- **动机**：现 `max_skip_slots=10` 是拍脑袋上限，不看交易值不值得等。改成显式期望值：`E[夹击损失|现在发] ≤ E[延迟成本]+E[损失|之后发]` 才发。把延迟成本正式建模，比 ratio 更有理论深度。
- **落点**：
  - 新 `mitigation/strategy/cost_balanced_strategy.py`，或在 EV 门控层统一过所有子策略输出。
  - `E[夹击损失] = P(sandwich|leader,tx) × E[fromDiff]`，`P(sandwich)` 由扩展1+2 的向量估计；延迟成本 = `f(delay_slots, token 波动率)`，参数从 802 拟合。
- **工作量**：中（数学模型是亮点，参数可拟合）。
- **评估**：EV 版 vs 硬上限版的延迟分布（p50/p95/p99）与缓解效果权衡曲线。

### 扩展 5：跨块感知持有（CrossBlockHold）

- **动机**：wide sandwich 的 back-run 可能落在 N+k slot，"等 1-2 slot"逃不掉。需根据 `crossBlock` gap 分布持有足够长以脱离 back-run 射程。
- **落点**：
  - 从 `sandwiches_nobundle` 统计 `crossBlock=true` 的 front→back slot 间距分布。
  - 新 `mitigation/strategy/crossblock_hold_strategy.py`，持有窗口 = gap 分布的高分位点。
- **工作量**：低中（统计 + 一条持有规则）。
- **评估**：跨块型子类型的命中率变化。

### 扩展 6：在线 EWMA + 性能基准（实时 + 性能工作量）

- **动机**：现一次性加载 800/801 静态数据。改成随 802 重放在线更新 EWMA，模拟当前 epoch 实时反馈（对应"后续整合实时信息"方向）。同时加吞吐/延迟基准覆盖"交易处理性能"工作量。
- **落点**：
  - `risk_store` 加 `update(slot, observation)` 在线更新方法。
  - 新 `mitigation/perf_benchmark.py`，记录 p50/p95/p99 delay_slots、txs/s、slot 处理速率、整 epoch（432000 slot）运行时。
- **工作量**：低（但论文"性能评估"章由此成立）。
- **评估**：静态 vs 在线 EWMA 的缓解效果差；性能基准表。

---

## 4. 评估体系（多维度）

现有评估只有 `on_risky_leader` 占比 + 三档标签。扩展为：

1. **按子类型分别报缓解效果**（不只是聚合）——合谋型/bundle型/跨块型/程序集中型各降多少。
2. **延迟成本分布**（p50/p95/p99 delay_slots）——证明有效且便宜。
3. **吞吐/性能基准**——sim 的 txs/s、slot 速率、整 epoch 运行时。
4. **消融实验**（ablation）——逐个去掉扩展1/2/3/4，看边际贡献，证明每个组件都有用。
5. **吸引力分校准曲线**——证明扩展2 的打分真能预测被夹。

留出验证口径不变：决策用历史（800/801 或在线 EWMA），评估用 802 自身观测，时间干净。

---

## 5. 论文故事线（推荐最小可行组合）

组合 **扩展1 + 扩展2 + 扩展3 + 扩展4** + 评估体系。

> Solana 三明治不是单一现象。现有验证者视角 risk-aware 策略只用 sandwich_ratio 一个标量，只能覆盖"验证者合谋"子类型——实测 32879 笔 victim 中 25838 笔因 leader 不在高 ratio 集合而未被保护。我们分解出四类夹击子类型（合谋/bundle/跨块/程序集中），构建 **leader 风险向量 + victim 吸引力向量** 双维度信号，用子类型分类器分发到对应防御策略，由期望值门控在"夹击损失"与"延迟成本"间做均衡。评估按子类型分别报效果并做消融，证明各组件边际贡献。

故事优点：
- 诊断有真实数据支撑（25838/32879）。
- 每个组件映射到现有代码字段，可复现。
- 比"调阈值"有理论深度（EV 均衡 + 子类型分类）。
- 天然带工作量（多策略 + 多维评估 + 性能基准 + 消融）。
- 与业界（bloXroute leader scoring）形成对照——本文做可解释学术版。

---

## 6. 实现顺序建议

1. **扩展2 Victim 吸引力评分器**：最快见效，直接打 25838，数据全在库里。
2. **扩展1 多信号向量 + EWMA**：基础设施，后续都依赖。
3. **扩展5 跨块感知持有**：独立、低工作量、效果好讲。
4. **扩展3 子类型集成**：把 1/2/5 串成框架主干。
5. **扩展4 EV 门控**：理论亮点，最后接入统一门控。
6. **扩展6 性能基准**：贯穿全程，最后出报告。

每个扩展完成后单独跑 `python -m mitigation.sim` 出一组 802 评估数据，逐步累加到论文实验章。

---

## 7. 与现有代码的接口约定

- 所有新策略继承 `mitigation/strategy/base_strategy.py` 的 `SendStrategy`，实现 `should_send` + `get_strategy_name`。
- 新的风险/评分器通过 `StrategyContext` 注入（`base_strategy.py` 已定义 `StrategyContext`，可扩展字段）。
- `factory.py` 增加新策略名（`multi-signal` / `subtype-aware` / `cost-balanced` / `crossblock-hold`），由 `MITIGATION_STRATEGY` config 选择，保持"CLI 不接参数、全走 config"的现有约定。
- `evaluation.py` 增加按子类型分桶的评估函数，复用 `load_risky_validators` / `load_sandwich_leaders` 的留出验证口径。
- 遵循 `CLAUDE.md`：直接修改现有文件、不创建重复模块、文档进 `docx/`、脚本进 `scripts/`、测试进 `tests/`。

---

## 8. 参考调研来源

- Helius《Solana MEV Report》——Vpe 程序、私有 mempool、Jito tip 规模。
- ACM IMC 2025《Quantifying the Threat of Sandwiching MEV on Jito》——跨块宽夹、23 验证者、月抽量。
- bloXroute 2025-10 leader-aware scoring 公告——业界对照。
- Dysnix / RPC Fast《Solana MEV Infrastructure 2026》——Jito bundle/ShredStream/SWQoS/多路提交。
- Jito 官方文档——bundle 原子性、`jitodontfront`、BAM（TEE 加密排序，2025-09 主网）。
