# MEV Rebate主哟技术

## 1. 项目目标

一套围绕真实交易数据、隐私提示、重排模拟、builder 分发和收益归因展开的原型平台。

- 将真实以太坊交易接入 MEV rebate 研究闭环；
- 将区块重排模拟与 builder 路由机制结合；
- 将 builder 信誉、reward 学习和去中心化探索统一进同一套分发框架；
- 为后续 contextual bandit、分层订单流路由和 rebate 机制研究提供了可运行基础。

## 2. 整体架构

系统由五个核心层组成：

1. 数据集层  
   从真实 Ethereum 公共节点采集交易，保存为 CSV，并在运行时加载为 replay dataset。

2. 交互层  
   `user` 基于数据集构造真实 bundle，`searcher` 基于 hint 构造 backrun bundle，统一通过 JSON-RPC 发给 `server`。

3. 模拟层  
   `server` 内部的 replay simulator 根据目标区块的真实历史交易和 bundle 交易做重排、冲突检测与插入模拟。

4. 分发层  
   模拟成功后的 bundle 会进入 builder dispatcher，按信誉、reward 和 exploration / exploitation 策略做 block producer 路由。

5. 观测层  
   系统提供 block / validator / searcher / builder 多维指标接口，同时将 builder 相关行为单独写入 `builder_report.log`。

## 3. 核心技术实现

### 3.1 真实交易数据采集与回放

系统首先通过交易采集程序从 Ethereum 公共 RPC 节点拉取区块、交易和收据，并写入 CSV。数据集中保留了后续模拟所需的关键字段，包括：

- 区块号、区块哈希、时间戳、base fee
- 交易哈希、from、to、nonce
- gas limit、receipt gas used、effective gas price
- logs count、method id
- `raw_tx`

运行时，数据集加载模块会把 CSV 解析成 `Block` 和 `Transaction` 结构，按区块号和交易序排序，同时建立按区块号和交易哈希的索引，供模拟器和客户端快速查询。

这部分实现位于：

- [internal/dataset/dataset.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/dataset/dataset.go)

### 3.2 Replay Simulator：基于真实区块上下文的重排模拟

Replay simulator 是本项目的核心技术模块之一。与简单的“固定返回成功”式 mock 模拟不同，它会：

1. 根据 bundle 的目标区块，在数据集中找到对应的历史区块；
2. 展开 bundle 中的交易和嵌套 bundle；
3. 用 sender + nonce 检测 bundle 与历史交易之间的冲突；
4. 根据 priority fee 估算 bundle 交易的插入位置；
5. 将 bundle 交易与历史交易合并，重排为新的候选区块顺序；
6. 在 block gas limit 约束下，模拟尾部交易被挤出；
7. 输出模拟后的区块上下文，包括插入位置、被挤出的交易、bundle 内交易位置等信息。

该模块使系统具备了“历史区块回放 + 插入重排”的能力，可以作为后续研究 bundle 竞争、builder 路由和 rebate 设计的基础。

这部分实现位于：

### 3.3 Hint 提取与隐私控制

系统支持从成功模拟的 bundle 中提取 hint，并通过 SSE 广播给 searcher。hint 提取支持多种粒度：

- 交易哈希
- 合约地址
- 函数选择器
- calldata
- logs / special logs
- 降精度后的 gas 相关信息


### 3.4 模拟工作流与服务端调度

`server` 作为系统中心节点，负责：

1. 加载配置和 replay dataset；
2. 初始化 signer、queue、bundle store、simulator、metrics store、dispatcher；
3. 启动 simulation worker；
4. 对外暴露 JSON-RPC、SSE 和 metrics 接口；
5. 周期性推进当前回放区块；
6. 在每个 bundle 成功模拟后，提取 hint、更新 metrics、并分发给 builder。

Simulation worker 的流程是：

1. 从模拟队列中取出 bundle；
2. 调用 replay simulator 执行模拟；
3. 根据结果更新 metrics；
4. 若成功则生成 matching hash、广播 hint、存储结果；
5. 将 bundle 交给 builder dispatcher。

### 3.5 Builder / Block Producer 动态评分机制

系统没有采用“固定 score + 固定路由”的静态 builder 选择方式，而是维护了一套动态行为画像。每个 builder 都会记录：

- 分发尝试次数
- 分发成功 / 失败次数
- sandwich attack 次数
- well-behaved 事件次数
- valuable order flow 次数
- 累计 reward、平均 reward、最近 reward

在此基础上，系统会计算两个层次的量：

1. `Score`  
   作为信誉分，反映 builder 的长期行为质量。成功率高、价值高、well-behaved 多的 builder 会升分；sandwich 攻击多、连续失败多的 builder 会降分。

2. `Reward`  
   作为 bandit 学习信号，反映最近的收益质量。reward 默认由成功、失败、价值、well-behaved、sandwich 等行为自动组合计算，也允许通过 API 显式注入。

### 3.6 Exploration / Exploitation 分层分发

系统在 builder / block producer 路由时，不再只按固定权重做 exploitation，而是加入了 exploration 层，用于解决新加入 producer 的冷启动问题和订单流中心化问题。

#### Exploration 层

以下 producer 会被视为 exploration candidate：

- dispatch 样本数仍不足；
- 注册时间较近，仍处于 grace period。

在 exploration 命中时，系统不会直接按 `Score` 选路，而是基于以下量计算探索权重：

- `expectedReward`
- 样本不足带来的 uncertainty 加权
- 新 producer bonus
- UCB bonus

即：探索时更偏向“高潜力但不确定”的 producer。

#### Exploitation 层

当未命中 exploration，或者不存在 under-explored producer 时，系统进入 exploitation 层。此时分发依据不再只是静态权重，而是：

`expectedReward = reputation_score * reward_multiplier`

也就是把长期信誉分和近期 reward 学习结果结合起来，尽量把更多订单流分给当前更有预期收益的 producer。

## 4. 主要创新点

### 4.1 将历史区块重排模拟引入 Rebate 研究

通过 replay simulator，本项目不仅能判断 bundle 是否成功，还能观察它在目标区块里的插入位置、对历史交易的挤出影响以及 gas / priority fee 层面的竞争关系。这使其具备研究订单流价值与分发策略的基础。

### 4.2 将隐私提示机制和 backrun 行为放到同一实验框架中

通过 matching hash、hint 提取、SSE 广播和 searcher backrun 构造，系统形成了一个简化但完整的 MEV-Share 风格闭环。

### 4.3 将 Builder 评价从静态权重推进到行为学习

系统不是单纯维护一个人工配置的 score，而是同时引入：

- 长期信誉分 `Score`
- 显式学习信号 `Reward`
- exploration / exploitation 分层调度
- UCB 风格不确定性奖励

这让 builder 路由从“固定配置”走向了“在线学习 + 去中心化约束”的方向。

### 4.4 面向去中心化的订单流分层路由

系统明确将“防止订单流过度集中于既有 producer”作为目标，通过 exploration 层给新 producer 样本机会，再通过 exploitation 层回到收益导向分发。这一点比单纯追求短期收益更接近真实 rebate / orderflow market 设计中的长期目标。

## 5. 后续演进方向

后续可以沿以下方向继续深化：

1. 引入更细粒度的 reward 定义  
   把成功率、价值、延迟、退款兑现、冲突率等统一纳入 reward。

2. 从启发式 bandit 走向 contextual bandit  
   把订单流类型、交易复杂度、历史收益波动、producer 特征等作为 context，引入更细粒度的策略学习。

3. 将订单流做分层分桶  
   例如将“便宜订单流用于探索、贵订单流用于保守兑现”，降低所有订单流共同承担的风险敞口。

4. 接入更真实的执行语义  
   例如引入更完整的交易执行和状态覆盖，减少 replay 与真实链上执行之间的偏差。

5. 增强可视化与报告系统  
   把 block、searcher、builder、reward、exploration / exploitation 分布统一做成面板，便于实验评估和对外汇报。

