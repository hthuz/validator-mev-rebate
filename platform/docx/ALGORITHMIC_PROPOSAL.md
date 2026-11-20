# Solana 三明治缓解：算法化方案（升级版）

> 替代 `EXTENSION_PROPOSAL.md` 的增量思路。本版聚焦**有名字、有数学、有机制**的算法，不再是在 `risk_store` 上加字段或换 EWMA 平滑。
> 基于 `platform/mitigation` 现有 sim（victim 重放 + slot_leaders 全表 + sandwich 观测）可仿真。
> 记录日期：2026-11-20。

---

## 0. 为什么需要升级

上一版（`EXTENSION_PROPOSAL.md`）的问题：
- 太贴着现有 `risk_store`/`strategy` 做增量（加字段、EWMA、子类型分发），算法本身没有质变。
- EWMA 只是平滑，没有 regret bound、没有最优性、没有博弈分析，撑不起论文"算法贡献"这一章。
- "路由"只停留在"等安全 leader"，没有真正的路径搜索/优化。

本版把缓解建模成三类**正经算法问题**，每个都有数学表述和可仿真评估：

| 类别 | 算法 | 解决的问题 |
|---|---|---|
| 在线学习 | Contextual Bandit (LinUCB/Thompson) | 不知道哪个 leader 安全，边路由边学 |
| 图优化 | 时序图最小费用路 / 最小费用最大流 | 在已知 leader schedule 里找全局最优路由 |
| 序列决策 | MDP 最优停止 (值迭代/Q-learning) | "何时发"是状态相关的最优停止问题 |
| 机制设计 | 博弈论 slashing + tip 分配 | 设计让夹击无利可图的规则 |
| 密码学效果 | 加密排序/批量拍卖仿真 | 量化 fair ordering 的收益与延迟代价 |
| 新范式 | Intent / order flow auction | 把"用户要什么"与"如何执行"解耦 |

所有方案都兼容现有 sim：victim 重放提供交易流，`slot_leaders` 表提供完整 leader schedule（→ 图/MDP 转移），`sandwiches_nobundle` 提供夹击观测（→ reward/label）。

---

## A. Contextual Bandit 在线路由（LinUCB / Thompson Sampling）

### 核心思想
把"这笔 victim 路由到未来哪个 slot 的 leader"建模成**上下文多臂老虎机**。每个候选目标 slot 是一个臂，leader+victim 特征是上下文，是否被夹+延迟成本是奖励。策略边路由边学习，最小化 regret。

### 数学建模
- 臂集 $\mathcal{A}(t) = \{t, t+1, \dots, t+W\}$（$W$ = max_skip 窗口内的候选 slot）
- 上下文 $x_{t,a} \in \mathbb{R}^d$：leader 风险向量（ratio/crossBlock_ratio/inbundle_ratio/is_jito/stake）拼接 victim 吸引力向量（fromAmount/slippage/programs/signerSame）
- 奖励 $r_{t,a} = -\big[\mathbb{1}[\text{夹击}] \cdot \text{loss} + \lambda \cdot (a - t)\big]$
- 目标：最小化 $T$ 轮累积 regret $\sum_t (\mu^*_t - \mu_{a_t})$

### 算法（LinUCB）
```
对每轮 t（每笔 victim）:
  构造候选臂集 A(t) = {t..t+W}
  对每个 a in A(t):
    θ_a = (X_a^T X_a + λI)^{-1} X_a^T r_a      # 岭回归估计
    p_a = θ_a^T x_{t,a} + α sqrt(x_{t,a}^T (X_a^T X_a + λI)^{-1} x_{t,a})  # UCB
  选 a* = argmax p_a，路由到 slot a*
  观测奖励 r（用 802 自身 sandwich 观测判定夹击指示）
  更新 X_{a*}, r_{a*}
```
- LinUCB 有 $\tilde{O}(d\sqrt{T})$ regret bound（Li et al. 2010），可写进论文。
- Thompson Sampling 版本：从后验 $\theta_a \sim \mathcal{N}(\hat\theta_a, \Sigma_a)$ 采样，贝叶斯视角。

### 仿真方法
- 现有 `VictimReplaySource` 提供 32879 笔交易流；`SlotManager.get_leader(slot)` 给候选臂的 leader。
- reward 的夹击指示：路由到 leader L 后，用 `load_sandwich_leaders(802)` 判定 L 是否在该 epoch 夹过 victim（现有 `evaluate_victim_outcomes` 的逻辑）。
- 在线更新：随 sim 推进，每个 epoch 段（如每 4320 slot）更新臂估计。

### 评估
- **累积 regret 曲线**（vs oracle 知道全部 leader 风险的最优路由）
- **收敛速度**（多少笔后 regret 斜率稳定）
- **vs 静态 risk-aware** 的夹击率与延迟成本
- **exploration-exploitation 权衡**（α 参数扫描）

### 论文分量
高。是在线学习在 MEV 路由的应用，有 regret bound，可对照 AdaChain（contextual MAB 选区块链架构）和 The Bidding Games（PPO 做 MEV bidding）。

---

## B. 时序图最小费用路 / 最小费用最大流路由

### 核心思想
leader schedule 是**已知的时序结构**（`slot_leaders` 表完整给出每个 slot 的 leader）。把它建成时序有向图，每笔交易在图上找一条"风险+延迟"最小的路径。多笔交易有 leader 容量约束时升级为最小费用最大流。

### 数学建模
时序图 $G = (V, E)$：
- 节点 $v = (\text{slot}, \text{leader})$，外加源点 $s$、汇点 $t'$
- 边 $e = ((i, L_i) \to (j, L_j))$ 对所有 $i < j \le i+W$（交易可从 slot i 持有到 j）
- 边权 $w(e) = \alpha \cdot \text{risk}(L_j) + \beta \cdot (j - i) + \gamma \cdot \text{attract}(tx, L_j)$
  - $\text{risk}(L)$ = leader 多信号风险分
  - $\text{attract}(tx, L)$ = victim 在 L 下的吸引力风险（夹击可榨取量）

### 算法
1. **单源最短路（Dijkstra）**：每笔交易独立，从其原 slot 出发到任意"安全"节点的最短路。复杂度 $O(|E|\log|V|)$。
2. **最小费用最大流（MCMF）**：当 leader 有容量约束（一个 leader 单位时间能处理的交易有限），多笔交易争抢同一安全 leader。用 SPFA 版 MCMF 求全局最优分配。复杂度 $O(F \cdot |V||E|)$。

```
建图:
  for slot i in [start, end]:
    for j in [i+1, min(i+W, end)]:
      add_edge((i, leader[i]) -> (j, leader[j]), weight=risk(leader[j]) + β(j-i))
路由一笔 tx (原 slot = i0):
  path = dijkstra(G, (i0, leader[i0]), 目标=sink)
  target_slot = path 上第一个 risk < 阈值 的节点
```

### 仿真方法
- `SlotManager` 已加载完整 slot→leader 映射，直接建图。
- 风险分用历史 800/801 的多信号向量；评估用 802 自身观测。
- MCMF 版：每个 leader 设容量 = 该 slot 实际交易数上限（从区块数据估）。

### 评估
- **全局最优 vs 贪心 risk-aware**：MCMF 解 vs 当前"遇安全就发"的贪心，量化次优性差距。
- **延迟-风险 Pareto 前沿**：扫 $(\alpha, \beta)$ 权重，画出风险暴露 vs 平均延迟的权衡曲线。
- **leader 容量约束的影响**：有/无容量下的路由质量。

### 论文分量
中高。图论优化，有明确复杂度、有 Pareto 分析、能证明贪心 risk-aware 是 MCMF 的特例（次优）。把"路由"从启发式升级为组合优化。

---

## C. MDP 最优停止（值迭代 / Q-learning）

### 核心思想
"这笔交易现在发还是再等一个 slot"是**最优停止问题**（optimal stopping）。当前 `max_skip_slots=10` 是固定上限；MDP 给出**状态相关**的最优策略：在某些状态该早发，某些状态该死等。

### 数学建模
MDP $\langle S, A, P, R, \gamma\rangle$：
- 状态 $s = (\text{current\_slot}, d, \text{leader\_feature}, \text{tx\_attract})$，$d$ = 已延迟 slot 数
- 动作 $a \in \{\text{send}, \text{wait}\}$
- 转移 $P$：leader schedule 已知 → $P$ **完全已知**（这是 model-based 的关键优势）
- 奖励 $R(s, \text{send}) = -[\Pr(\text{夹击}|s) \cdot \text{loss} + 0]$
        $R(s, \text{wait}) = -c_{\text{delay}}(d)$（延迟成本）
- 目标：最优策略 $\pi^*$ 最小化期望累计成本

### 算法（值迭代，因 P 已知）
```
V(s) = 0 for all s
repeat until converge:
  for each s:
    Q(s, send) = R(s, send)
    Q(s, wait) = R(s, wait) + γ · Σ_{s'} P(s'|s, wait) · V(s')
    V(s) = min(Q(s, send), Q(s, wait))
π*(s) = send if Q(s, send) ≤ Q(s, wait) else wait
```
- 因 leader schedule 确定已知，$P$ 是确定转移，值迭代收敛快。
- 也可做 model-free Q-learning 版本做对照（假装不知 schedule）。

### 仿真方法
- 状态里的 leader_feature 用 800/801 历史风险向量。
- $\Pr(\text{夹击}|s)$ 从历史数据拟合（leader 风险分 → 夹击频率）。
- Bellman 方程可解析简化（确定性转移）。

### 评估
- **MDP π* vs 固定 max_skip**：同延迟预算下夹击率谁低。
- **状态相关策略可视化**：哪些 (leader风险, 已延迟数) 组合该发/该等——比单一阈值可解释。
- **γ（延迟折扣）敏感性**。

### 论文分量
中高。序列决策，有 Bellman 方程、有最优性、有状态相关策略（vs 固定阈值的根本区别）。比 EWMA 高一个量级。

---

## D. 博弈论机制设计（Nash 均衡 + Slashing）

### 核心思想
前面 A/B/C 都是"我方单边优化路由"。本方向反过来：**设计协议层机制，让夹击对验证者/搜索者无利可图**。把验证者 vs 搜索者建成博弈，设计 tip 分配 + slashing 条件使"诚实"成为 Nash 均衡。

### 数学建模
双人非合作博弈 $\langle \mathcal{V}, \mathcal{S}, u_V, u_S\rangle$：
- 验证者策略 $\mathcal{V} = \{\text{honest}, \text{collude}\}$
- 搜索者策略 $\mathcal{S} = \{\text{attack}, \text{noop}\}$
- 收益：
  - $u_V(\text{collude}, \text{attack}) = \text{tip}_{\text{MEV}} - \Pr(\text{被检测}) \cdot \text{slash}$
  - $u_V(\text{honest}, \cdot) = \text{base\_reward}$
  - $u_S(\text{attack}, \text{collude}) = \text{profit} - \text{tip}_{\text{MEV}}$
- **目标**：找 slashing 阈值 $\text{slash}^*$ 与 tip 分配规则，使 $(\text{honest}, \text{noop})$ 是 Nash 均衡。
- 均衡条件：$u_V(\text{honest}, \text{noop}) \ge u_V(\text{collude}, \text{noop})$ 且 $u_S(\text{noop}, \text{honest}) \ge u_S(\text{attack}, \text{honest})$

### 机制
- **slashing**：被 SFDP/JitoSOL 黑名单 = 失去委托质押（实际发生，见调研）。
- **tip 重分配**：把 MEV tip 的一部分回流给被夹用户（order flow auction 的 refund）。
- **可验证作恶证明**：检测到 sandwich bundle 即触发 slashing（用 `inbundle`/`crossBlock` 字段作证据）。

### 仿真方法
- 用 800-802 的实际 sandwich 利润（`fromDiff`）作 $\text{profit}$ 参数。
- 用实际被 SFDP 移除的验证者质押量作 $\text{slash}$ 量级。
- 扫 slashing 阈值，画出"夹击有利区 vs 诚实有利区"的相变图。

### 评估
- **相变曲线**：slash 量 vs 夹击发生率，找临界点。
- **社会福利**（用户损失 + 验证者收益）随机制参数的变化。
- **均衡分析**：不同 tip 分配比例下的均衡类型。

### 论文分量
高。mechanism design，有 Nash 均衡推导、有相变分析。对应 ScienceDirect《Formalization of MEV》的"role-dependent constrained optimization"框架。不依赖实现密码学，纯博弈分析。

---

## E. 加密排序 / Fair Ordering 效果仿真

### 核心思想
threshold encryption / commit-reveal 让交易内容在排序前不可见，从根上消除夹击的信息不对称。**不实现密码学**，而是仿真"如果搜索者看不到 victim 详情，夹击成功率掉多少"，量化 fair ordering 的收益与延迟代价。

### 数学建模
- 当前搜索者观测：完整 victim 特征 $x$（fromAmount/slippage/token）→ 吸引力分 $a(x)$
- fair ordering 下搜索者只看到 commit $h = H(x)$：无法算 $a(x)$
- 夹击决策从"基于 $a(x)$ 精确判定"退化为"基于先验随机判定"
- 期望夹击率下降 = $\Pr(a(x) > \tau) - \Pr_{\text{prior}}$（先验下无信息）

### 机制对照
| 方案 | 搜索者可见性 | 延迟代价 | 夹击消除率 |
|---|---|---|---|
| 现状（无加密） | 完整 victim 特征 | 0 | 0% |
| Threshold encryption | 排序后解密 | +100-500ms | ~高 |
| Batch auction（统一清算价） | 无序依赖 | +1 slot | ~完全 |
| Commit-reveal | 揭示前不可见 | +1-2 slot | ~高 |

### 仿真方法
- 用 802 victim 的吸引力分分布，模拟"搜索者按 $a(x)>\tau$ 决定夹"的现状夹击率。
- 切到"搜索者只能随机夹"（先验），算新夹击率。
- 对照 batch auction：同 slot 多笔 swap 统一清算价，夹击利润归零。

### 评估
- **信息泄露量化**：搜索者看到多少特征 vs 多少 commit。
- **fair ordering 收益-延迟 Pareto**：不同加密方案的夹击消除率 vs 延迟。
- **batch auction 在 pump.fun 代币上的效果**（Vpe 程序场景）。

### 论文分量
中高。把密码学方案的效果**可量化**，对应 Aequitas/Themis/Shutter/FairBlock 学术脉络，但不需要实现密码学——是效果评估贡献。

---

## F. Intent / Order Flow Auction 仿真

### 核心思想
借鉴 MEV-Share / UniswapX：用户提交 intent（"换 X→Y，最低输出 Z"）而非签名交易，求解者竞价承接，用户拿回扣。把"用户要什么"与"如何执行"解耦，夹击者无法插队。

### 数学建模
- 用户提交 intent $I = (\text{from}, \text{to}, \text{min\_out}, \text{deadline})$
- 求解者集 $\{b_k\}$ 竞价，出价 $p_k$（给用户的回扣）
- 中标 $b^* = \arg\max p_k$，用户实得 $\text{min\_out} + p_k$
- 批量清算：同批多 intent 统一撮合，消除排序依赖

### 仿真方法
- 把 802 的 victim 改成 intent，求解者用历史 sandwich 利润作竞价上限。
- 对照：直接发交易（被夹）vs intent 拍卖（拿回扣）的用户净收益。

### 评估
- **用户净收益**：被夹损失 vs intent 回扣。
- **求解者竞争强度**对用户收益的影响。

### 论文分量
中。order flow 新范式，前沿但仿真简化。

---

## 推荐组合与论文故事线

### 推荐主线：A + B + D（在线学习 + 图优化 + 机制设计）

这是"算法分量 × 可仿真度"右上象限的最强组合：

> **故事**：Solana 三明治缓解可建模为三层问题。
> (1) **路由层**——已知 leader schedule 是时序图，用**最小费用流**求全局最优路由（B），贪心 risk-aware 是其特例（次优）。
> (2) **学习层**——leader 风险未知且动态，用 **Contextual Bandit (LinUCB)** 在线路由，最小化 regret（A），有 $\tilde{O}(d\sqrt{T})$ 界。
> (3) **机制层**——单边路由有上限，设计 **slashing + tip 重分配**机制使夹击非 Nash 均衡（D）。
> 三层从"已知最优→未知在线→博弈均衡"递进，每层都有数学与仿真评估。

### 备选：C + E（序列决策 + 密码学效果）
若偏好"决策理论 + 密码学"叙事：MDP 最优停止给出状态相关策略（C），加密排序仿真量化信息不对称消除的收益（E）。

### 评估体系（所有组合共用）
1. 夹击率（按子类型分报）
2. 延迟成本分布（p50/p95/p99）
3. regret 曲线 / Pareto 前沿 / 相变图（按算法）
4. 吞吐与运行时基准
5. 消融（逐层去掉看边际贡献）
6. vs 静态 risk-aware baseline 的全面对照

---

## 与现有平台的接口

- **A/B/C** 都实现成新的 `SendStrategy` 子类（`bandit_strategy.py` / `graph_route_strategy.py` / `mdp_strategy.py`），通过 `StrategyContext` 注入 leader 查询与风险数据。`should_send` 返回是否在当前 slot 发；路由目标通过扩展 `StrategyContext` 增加 `route_target` 字段返回。
- **D** 是机制分析，产出 slashing 阈值表与相变图，不一定改 strategy 代码。
- **E/F** 是仿真模式切换，在 `sim.py` 增加 `ordering_mode ∈ {plain, encrypted, batch, intent}` 参数。
- reward/label 统一走现有 `evaluation.py` 的留出验证口径（决策用历史，评估用 802 自身）。
- 遵循 `CLAUDE.md`：直接修改、不建重复模块、文档进 `docx/`、测试进 `tests/`。

---

## 参考学术脉络

- **在线学习**：Li et al. 2010 (LinUCB), Chapelle & Li 2011 (Thompson Sampling), AdaChain (contextual MAB 选区块链架构, arXiv 2211.01580), The Bidding Games (PPO 做 MEV bidding, Polygon Atlas)。
- **图优化**：Dijkstra, SPFA-MCMF 标准算法。
- **序列决策**：Bellman/Sutton MDP，optimal stopping theory。
- **机制设计/博弈**：ScienceDirect《Formalization of MEV》(2026) role-dependent constrained optimization + PBS/threshold/auction taxonomy; EIP-7732 ePBS; Arbitrum Timeboost; Skip Protocol。
- **公平排序/密码学**：Aequitas, Themis, Wendy, Shutter Network, FairBlock (Momeni et al.), BlindPerm (Kavousi et al.), CowSwap/Lissy batch auction。
- **Intent/order flow**：MEV-Share, UniswapX, SUAVE。
