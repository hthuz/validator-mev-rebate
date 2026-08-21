# MEV Rebate主要技术

## 1. 项目目标

一套围绕真实交易数据、隐私提示、重排模拟、builder 分发和收益归因展开的原型平台。

- 将真实以太坊交易接入 MEV rebate 研究闭环；
- 将区块重排模拟与 builder 路由机制结合；
- 将 builder 信誉、reward 学习和去中心化探索统一进同一套分发框架；
- 为后续 contextual bandit、分层订单流路由和 rebate 机制研究提供了可运行基础。

## 2. 整体架构

系统由五个核心层组成。当前代码同时支持 replay 实验和 mock 压测实验：replay 模式依赖真实交易 CSV，mock 模式直接生成可控的区块和 bundle 流量，用于长周期 builder score / bandit 策略验证。

1. 数据集层  
   从真实 Ethereum 公共节点采集交易，保存为 CSV，并在 replay 模式运行时加载为 replay dataset。mock 模式不依赖 CSV，而是由 `MockSimulator` 生成区块上下文和模拟结果。

2. 交互层  
   `user` 可以基于数据集构造真实 bundle，也可以在 mock 模式下持续生成测试 bundle；`searcher` 基于 hint 构造 backrun bundle，统一通过 JSON-RPC 发给 `server`。

3. 模拟层  
   `server` 根据配置选择 replay simulator 或 mock simulator。replay simulator 根据目标区块的真实历史交易和 bundle 交易做重排、冲突检测与插入模拟；mock simulator 用于高吞吐、可重复的策略实验。

4. 分发层  
   模拟成功后的 bundle 会进入 builder dispatcher，按信誉、reward 和 exploration / exploitation 策略做 block producer 路由。

5. 观测层  
   系统提供 block / validator / searcher / builder 多维指标接口，运行日志统一写入 `logs/rebate.log`，实验指标写入配置指定的 `logs/experiment*` 目录，并通过 JSONL 文件记录 bundle、dispatch、builder snapshot 和 block summary。

整体架构如下：

```text
┌───────────────────────────────────────────────────────────────────────────────────────────────┐
│                                       Clients / 外部交互层                                      │
│                                                                                               │
│  ┌────────────────────────────┐          ┌─────────────────────────────────────────────────┐  │
│  │ user                       │          │ searcher                                        │  │
│  │ - replay bundle            │          │ - subscribe /events hints                       │  │
│  │ - mock bundle generator    │          │ - build backrun bundle                          │  │
│  │ - subscribe /blocks        │          │ - submit backrun bundle                         │  │
│  └──────────────┬─────────────┘          └──────────────────────┬──────────────────────────┘  │
└─────────────────┼────────────────────────────────────────────────┼─────────────────────────────┘
                  │ JSON-RPC: eth_sendMevBundle / eth_callBundle  │
                  └────────────────────────────┬───────────────────┘
                                               ▼
┌───────────────────────────────────────────────────────────────────────────────────────────────┐
│                                            server                                             │
│                                                                                               │
│  ┌──────────────────────┐       ┌──────────────────────┐       ┌──────────────────────────┐   │
│  │ JSON-RPC API          ├──────▶│ SimulationQueue       ├──────▶│ SimulationWorker(s)       │   │
│  │ POST /                │       │ target block gating   │       │ workers = config value    │   │
│  └──────────┬───────────┘       └──────────┬───────────┘       └──────────┬───────────────┘   │
│             │                              │                              │                   │
│             │                              │                              ▼                   │
│             │                              │                 ┌──────────────────────────┐      │
│             │                              └────────────────▶│ ReplaySimulator          │      │
│             │                                                │ or MockSimulator         │      │
│             │                                                └──────────┬───────────────┘      │
│             │                                                           │ simulation result    │
│             │                                                           ▼                      │
│             │        ┌──────────────────────┐       ┌──────────────────────────┐              │
│             ├───────▶│ Builder HTTP API      │       │ BundleStore              │              │
│             │        │ /builders/scores      │       │ matching hash + result   │              │
│             │        │ /builders/register    │       └──────────┬───────────────┘              │
│             │        │ /builders/observe     │                  │                              │
│             │        └──────────┬───────────┘                  ▼                              │
│             │                   │                    ┌──────────────────────────┐              │
│             │                   │                    │ Hint Extractor           │              │
│             │                   │                    │ privacy-controlled hints │              │
│             │                   │                    └──────────┬───────────────┘              │
│             │                   │                               ▼                              │
│             │                   │                    ┌──────────────────────────┐              │
│             │                   │                    │ SSE Hub                  │              │
│             │                   │                    │ /events hints            │              │
│             │                   │                    │ /blocks new blocks       │              │
│             │                   │                    └──────────────────────────┘              │
│             │                   │                                                               │
│             │                   ▼                                                               │
│             │        ┌──────────────────────┐       ┌──────────────────────────┐              │
│             │        │ BuilderRegistry       │◀─────▶│ Builder Dispatcher        │              │
│             │        │ BaseScore/Score/Stats │       │ Score + Reward + Bandit  │              │
│             │        └──────────┬───────────┘       └──────────┬───────────────┘              │
│             │                   │                              │ eth_sendMevBundle             │
│             │                   │                              ▼                              │
│             │                   │                    ┌──────────────────────────┐              │
│             │                   │                    │ Builders / Producers      │              │
│             │                   │                    │ alpha / beta / ...        │              │
│             │                   │                    │ mock builder servers      │              │
│             │                   │                    └──────────────────────────┘              │
│             │                   │                                                               │
│             │                   ▼                                                               │
│             │        ┌──────────────────────────────────────────────────────┐                  │
│             └───────▶│ ObservabilityService                                  │                  │
│                      │ - MetricsStore: block/validator/searcher/global        │                  │
│                      │ - ExperimentRecorder: metadata + JSONL files           │                  │
│                      │ - MetricsHandler: HTTP /metrics/*                     │                  │
│                      └──────────────────────────────────────────────────────┘                  │
└───────────────────────────────────────────────────────────────────────────────────────────────┘
```

## 3. 核心技术实现

### 3.1 真实交易数据采集与回放

在 replay 模式下，系统首先通过交易采集程序从 Ethereum 公共 RPC 节点拉取区块、交易和收据，并写入 CSV。数据集中保留了后续模拟所需的关键字段，包括：

- 区块号、区块哈希、时间戳、base fee
- 交易哈希、from、to、nonce
- gas limit、receipt gas used、effective gas price
- logs count、method id
- `raw_tx`

运行时，数据集加载模块会把 CSV 解析成 `Block` 和 `Transaction` 结构，按区块号和交易序排序，同时建立按区块号和交易哈希的索引，供模拟器和客户端快速查询。

这部分实现位于：

- [internal/dataset/dataset.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/dataset/dataset.go)
- [internal/sim/replay_sim.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/sim/replay_sim.go)
- [internal/client/replay_bundle.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/client/replay_bundle.go)

### 3.2 Replay Simulator：基于真实区块上下文的重排模拟

Replay simulator 是本项目的核心技术模块之一。与用于长周期压测的 mock 模拟不同，它会：

1. 根据 bundle 的目标区块，在数据集中找到对应的历史区块；
2. 展开 bundle 中的交易和嵌套 bundle；
3. 用 sender + nonce 检测 bundle 与历史交易之间的冲突；
4. 根据 priority fee 估算 bundle 交易的插入位置；
5. 将 bundle 交易与历史交易合并，重排为新的候选区块顺序；
6. 在 block gas limit 约束下，模拟尾部交易被挤出；
7. 输出模拟后的区块上下文，包括插入位置、被挤出的交易、bundle 内交易位置等信息。

该模块使系统具备了“历史区块回放 + 插入重排”的能力，可以作为后续研究 bundle 竞争、builder 路由和 rebate 设计的基础。

这部分实现位于：

- [internal/sim/replay_sim.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/sim/replay_sim.go)
- [internal/sim/mock_sim.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/sim/mock_sim.go)

### 3.3 Hint 提取与隐私控制

系统支持从成功模拟的 bundle 中提取 hint，并通过 SSE 广播给 searcher。hint 提取支持多种粒度：

- 交易哈希
- 合约地址
- 函数选择器
- calldata
- logs / special logs
- 降精度后的 gas 相关信息

成功模拟后，系统使用 signer 生成 matching hash，把 bundle 与模拟结果写入 `BundleStore`，再按 bundle privacy 配置提取 hint。`searcher` 通过 `/events` SSE 订阅 hint；mock user 可以通过 `/blocks` SSE 订阅新区块事件，以便在新区块到来时生成新 bundle。

这部分实现位于：

- [internal/hints/hints.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/hints/hints.go)
- [internal/sse/hub.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/sse/hub.go)
- [internal/sse/block_hub.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/sse/block_hub.go)

### 3.4 模拟工作流与服务端调度

`server` 作为系统中心节点，负责：

1. 加载 YAML 配置，并根据 `simulator.mode` 初始化 replay 或 mock simulator；
2. 初始化 signer、queue、bundle store、observability service、builder registry 和 dispatcher；
3. 按 `simulator.workers` 启动多个 simulation worker；
4. 如果配置了 `mock_builders`，在本进程内启动 mock builder HTTP 节点；
5. 对外暴露 JSON-RPC、SSE、builder 管理和 metrics 接口；
6. 周期性推进当前区块，并把新区块广播到 `/blocks`；
7. 在每个 bundle 成功模拟后，提取 hint、更新 metrics、记录实验事件，并分发给 builder；
8. mock 模式下，如果配置 `simulator.stop_block_number`，区块推进到该高度后 server 自动 graceful shutdown。

Simulation worker 的流程是：

1. 从模拟队列中取出 bundle；
2. 调用当前配置的 simulator 执行模拟；
3. 通过 `ObservabilityService` 更新 block / validator / searcher metrics；
4. 通过 `ObservabilityService` 将 bundle simulation event 写入实验 JSONL；
5. 若成功则生成 matching hash、广播 hint、存储结果；
6. 将 bundle 交给 builder dispatcher；
7. dispatcher 选择 target producer、发送 bundle、记录 dispatch event；
8. dispatcher 根据发送结果写入 builder observation，并触发动态 score 更新。

运行时流程如下：

```text
┌────────────────┐
│ user/searcher  │
│ submit bundle  │
└───────┬────────┘
        │ POST / JSON-RPC: eth_sendMevBundle
        ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ JSON-RPC API                                                                │
│ - validate request                                                          │
│ - parse SendMevBundleArgs                                                   │
│ - enqueue by inclusion.block                                                │
└───────┬──────────────────────────────────────────────────────────────────────┘
        ▼
┌──────────────────────┐       block update       ┌──────────────────────────┐
│ SimulationQueue       │◀────────────────────────│ blockUpdater              │
│ wait until target     │                         │ AdvanceBlock()            │
│ block is processable  │                         │ Broadcast /blocks         │
└───────┬──────────────┘                         └───────────┬──────────────┘
        │ pop bundle                                          │ if mock and
        ▼                                                     │ stop_block_number reached
┌──────────────────────┐                                      ▼
│ SimulationWorker      │                              ┌──────────────────┐
│ worker goroutine      │                              │ graceful shutdown │
└───────┬──────────────┘                              └──────────────────┘
        │ SimulateBundle()
        ▼
┌──────────────────────────────┐
│ ReplaySimulator / MockSimulator│
│ - replay: reorder real block   │
│ - mock: generate mock result   │
└───────┬──────────────────────┘
        │ simulation result
        ▼
┌──────────────────────────────┐
│ ObservabilityService          │
│ update MetricsStore           │
│ block/validator/searcher      │
└───────┬──────────────────────┘
        │ RecordBundleSimulation
        ▼
┌──────────────────────────────┐
│ ExperimentRecorder inside     │
│ observability module          │
│ bundle_events.jsonl           │
└───────┬──────────────────────┘
        │
        ├────────────────────────── simulation failed ───────────────────────┐
        │                                                                    │
        ▼ simulation success                                                 │
┌──────────────────────┐       ┌──────────────────────┐                     │
│ BundleStore           │──────▶│ Hint Extractor        │                     │
│ matching hash/result  │       │ privacy hints         │                     │
└──────────────────────┘       └──────────┬───────────┘                     │
                                          ▼                                 │
                               ┌──────────────────────┐                    │
                               │ SSE /events           │                    │
                               │ searcher receives hint│                    │
                               └──────────────────────┘                    │
                                                                            │
        ┌───────────────────────────────────────────────────────────────────┘
        ▼
┌──────────────────────┐
│ Builder Dispatcher    │
│ select target producer│
└───────┬──────────────┘
        │ read Score/Reward/Stats
        ▼
┌──────────────────────┐
│ BuilderRegistry       │
│ candidate snapshot    │
└───────┬──────────────┘
        │ target selected
        ▼
┌──────────────────────┐       eth_sendMevBundle      ┌──────────────────────┐
│ Builder Dispatcher    ├─────────────────────────────▶│ Target Builder        │
│ record dispatch event │◀─────────────────────────────┤ success / error       │
└───────┬──────────────┘                               └──────────────────────┘
        │ Observe dispatch result
        ▼
┌──────────────────────┐
│ BuilderRegistry       │
│ update reward stats   │
│ update effective score│
└───────┬──────────────┘
        │ RecordBuilderSnapshot
        ▼
┌──────────────────────┐
│ ObservabilityService  │
│ builder_dispatches    │
│ builder_snapshots     │
│ block_summary         │
└──────────────────────┘
```

观测模块实现位于：

- [internal/observability/service.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/observability/service.go)
- [internal/observability/store.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/observability/store.go)
- [internal/observability/recorder.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/observability/recorder.go)
- [internal/observability/handler.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/observability/handler.go)
- [internal/observability/types.go](file:///Users/bytedance/validator-mev-rebate/eth/rebate/internal/observability/types.go)

服务端对外接口与当前实现对应如下：

- `POST /`：JSON-RPC 入口，支持 `eth_sendMevBundle`、`eth_callBundle`、`eth_cancelBundle`、`eth_blockNumber`
- `GET /events`：hint SSE stream
- `GET /blocks`：新区块 SSE stream
- `GET /builders/scores`：查询 builder 动态分数与画像
- `POST /builders/register`：注册新 builder
- `POST /builders/observe`：注入 builder 行为观测
- `GET /metrics/block/{blockNumber}`：查询单区块指标
- `GET /metrics/validator/{address}`、`GET /metrics/validators`：查询 validator 维度指标
- `GET /metrics/searcher/{address}`、`GET /metrics/searchers`：查询 searcher 维度指标
- `GET /metrics/global`、`GET /metrics/recent`：查询全局与近期区块指标

### 3.5 Builder / Block Producer 动态评分机制

系统没有采用“固定 score + 固定路由”的静态 builder 选择方式，而是维护了一套动态行为画像。配置文件中的 `score` 只作为 `BaseScore`，表示 builder 初始信誉；系统实际用于调度的是运行时动态更新的 `Score`，也称 `effective_score`。二者关系是：

- `BaseScore`：人工配置或注册时给定的初始信誉分，不会被普通观测直接改写；
- `Score`：根据行为观测、收益反馈和竞争约束计算得到的动态信誉分，直接影响后续分发权重；
- `Reward`：bandit 层使用的收益学习信号，记录在 `AverageReward` / `LastReward` 中，用于修正 `expectedReward`。

每个 builder 会持续累计以下行为画像：

- 分发尝试次数
- 分发成功 / 失败次数
- sandwich attack 次数
- well-behaved 事件次数
- valuable order flow 次数
- 连续失败次数
- 累计 reward、平均 reward、最近 reward

#### Score 更新输入

每次成功模拟后的 bundle 会进入 dispatcher。dispatcher 向目标 builder 发送 bundle 后，会自动写入一次观测：

- `DispatchAttempts = 1`
- 若发送成功，则 `DispatchSuccesses = 1`
- 若模拟结果产生正的 `profit + refundable_value`，则作为 `ValueCreatedWei`

实验脚本或外部系统也可以通过 `/builders/observe` 注入批量观测，例如成功率、sandwich attack、well-behaved 事件、显式 reward 等。这些观测会统一进入同一套 score 更新逻辑。

#### Reward 计算

如果观测中显式给出 `Reward`，系统直接使用该值，并截断到：

```text
Reward ∈ [-2.0, 2.0]
```

如果没有显式给出 reward，则由行为指标自动计算：

```text
success_rate = dispatch_successes / dispatch_attempts
failure_rate = (dispatch_attempts - dispatch_successes) / dispatch_attempts
value_component = clamp(log10(1 + value_created_eth) * 0.20, 0, 0.75)
well_behaved_component = clamp(well_behaved_events * 0.05, 0, 0.50)
sandwich_penalty = sandwich_attacks * 1.10

reward =
    1.00 * success_rate
  + 0.60 * value_component
  + 0.15 * well_behaved_component
  - 0.70 * failure_rate
  - sandwich_penalty

reward = clamp(reward, -2.0, 2.0)
```

其中价值项采用对数函数，避免单次高额订单流把 reward 拉得过高：

```text
value_reward = log10(1 + value_created_eth) * 0.20
```

每次观测后，系统更新：

```text
RewardSamples += 1
TotalReward += reward
AverageReward = TotalReward / RewardSamples
LastReward = reward
```

#### Effective Score 计算

动态信誉分不是直接跳到目标值，而是先计算一个 `targetScore`，再按不同步长平滑更新。

基础指标：

```text
success_rate = dispatch_successes / dispatch_attempts
reliability_factor = clamp(0.5 + success_rate, 0.35, 1.50)
failure_factor = clamp(1.0 - 0.12 * consecutive_failures, 0.40, 1.0)
sandwich_factor = clamp(1.0 - 0.20 * sandwich_attacks, 0.10, 1.0)
well_behaved_factor = clamp(1.0 + 0.05 * well_behaved_events, 1.0, 1.5)
value_factor = 1.0 + clamp(value_reward(total_value_created_wei), 0, 0.75)
```

目标分：

```text
targetScore =
    BaseScore
  * reliability_factor
  * failure_factor
  * sandwich_factor
  * well_behaved_factor
  * value_factor

targetScore = clamp(targetScore, 0.5, 100.0)
targetScore = min(targetScore, BaseScore + 15.0)
targetScore = targetScore * competition_factor
```

其中：

- 正常成功行为通过 `reliability_factor` 缓慢抬升信誉；
- 连续失败通过 `failure_factor` 处罚，单次全失败会累积 `ConsecutiveFailures`；
- sandwich attack 通过 `sandwich_factor` 快速扣分，每个事件按 `0.20` 线性惩罚，最低压到 `0.10`；
- well-behaved 事件最高提供 `1.5x` 正向因子；
- 价值产出最高提供 `0.75` 的额外因子；
- 单个 builder 的正向涨分最多不能超过 `BaseScore + 15`，避免短期高收益导致信誉失控；
- 全局有效分数范围为 `[0.5, 100]`。

#### Competition Factor

为避免订单流长期集中到单一 builder，score 还会乘以竞争惩罚：

```text
expected_share = 1 / builder_count
actual_share = builder_dispatch_attempts / total_dispatch_attempts

concentration_penalty =
    max(actual_share - expected_share, 0) * 0.35

relative_reward_penalty =
    if max_average_reward > 0 and builder_average_reward < max_average_reward:
        (1 - clamp(builder_average_reward / max_average_reward, 0, 1)) * 0.10
    else:
        0

competition_factor =
    1 - clamp(concentration_penalty + relative_reward_penalty, 0, 0.15)
```

也就是说，如果某个 builder 获得的流量份额明显高于均分预期，或者相对其他 builder 的平均 reward 偏低，它的动态 score 会被额外下调，最大下调幅度为 `15%`。

#### 平滑更新步长

最终 `Score` 不会直接替换为 `targetScore`，而是按方向使用不同更新速率：

```text
if targetScore >= currentScore:
    rate = 0.01
else:
    rate = 0.05

newScore = currentScore + (targetScore - currentScore) * rate
newScore = clamp(newScore, 0.5, 100.0)
```

这体现了系统的非对称更新原则：

- 好行为缓慢加分，单次正反馈只推进约 `1%`；
- 坏行为快速降分，负反馈按约 `5%` 步长靠近目标低分；
- 因此恶意行为会比正常收益更快反映到调度权重中。

### 3.6 Exploration / Exploitation 分层分发

系统在 builder / block producer 路由时，不再只按固定权重做 exploitation，而是加入了 exploration 层，用于解决新加入 producer 的冷启动问题和订单流中心化问题。

Dispatcher 决策路径如下：

```text
┌──────────────────────────────┐
│ Dispatch(bundle, simResult)  │
└───────────────┬──────────────┘
                ▼
┌──────────────────────────────┐
│ candidates = registry.All()  │
└───────────────┬──────────────┘
                ▼
        ┌──────────────────────────────┐
        │ privacy.builders is nonempty │
        └───────────────┬──────────────┘
                        │
          yes ┌─────────▼─────────┐ no
        ┌────▶│ filter candidates │──────────────┐
        │     └─────────┬─────────┘              │
        │               ▼                        ▼
        │     ┌───────────────────┐      ┌───────────────────┐
        │     │ filtered is empty │      │ use all builders   │
        │     └─────────┬─────────┘      └─────────┬─────────┘
        │               │ yes                      │
        │               └──────────────┬───────────┘
        │                              ▼
        │                    ┌───────────────────┐
        └────────────────────│ final candidates  │
                             └─────────┬─────────┘
                                       ▼
                              ┌────────────────┐
                              │ len == 1 ?     │
                              └───────┬────────┘
                                      │ yes
                                      ▼
                              ┌────────────────┐
                              │ direct target  │
                              └───────┬────────┘
                                      │
                         no           │
          ┌───────────────────────────┘
          ▼
┌──────────────────────────────────────┐
│ explorationCandidates =              │
│   attempts < minExploreDispatches    │
│   OR age < newProducerGracePeriod    │
└───────────────┬──────────────────────┘
                ▼
        ┌────────────────────────────────┐
        │ exploration enabled AND        │
        │ explorationCandidates nonempty │
        └───────────────┬────────────────┘
                        │ no
                        ▼
              ┌─────────────────────┐
              │ Exploitation layer   │
              │ weight=expectedReward│
              └──────────┬──────────┘
                         │
                         │ yes
                         ▼
              ┌─────────────────────┐
              │ exploration_mode     │
              └──────┬────────┬─────┘
                     │        │
          alternating│        │probabilistic
                     ▼        ▼
       ┌────────────────┐   ┌──────────────────────────┐
       │ nextExplore ?   │   │ random < explorationRate │
       └───────┬────────┘   └────────────┬─────────────┘
               │ yes                     │ yes
               ▼                         ▼
       ┌──────────────────────────────────────────────┐
       │ Exploration layer                             │
       │ weight=banditScore                            │
       │ banditScore = explorationWeight + ucbBonus   │
       └───────────────────┬──────────────────────────┘
                           │
       no / random miss    │
              ┌────────────┘
              ▼
       ┌──────────────────────────────────────────────┐
       │ weightedPick                                 │
       │ pick=random(0,totalWeight)                   │
       │ select first cumulativeWeight > pick         │
       └───────────────────┬──────────────────────────┘
                           ▼
       ┌──────────────────────────────────────────────┐
       │ target builder selected                       │
       │ send eth_sendMevBundle                        │
       │ record dispatch + observe result              │
       │ update reward stats + effective score         │
       └──────────────────────────────────────────────┘
```

一次 dispatch 的目标 producer 选择流程如下：

1. 从 registry 取出所有 builder；
2. 如果 bundle 的 privacy 字段指定了 builder 白名单，则先按白名单过滤；
3. 如果过滤后为空，则回退到全部已注册 builder；
4. 若只有一个候选 builder，则直接选择该 builder；
5. 若有多个候选 builder，则先计算 exploration candidates；
6. 根据 exploration 配置选择进入 exploration 层或 exploitation 层；
7. 在选定层内使用加权随机抽样选择最终 target producer。

当前 `experiment_mock_5builders.yaml` 中的核心参数为：

```yaml
dispatcher:
  strategy:
    exploration_enabled: true
    exploration_mode: alternating
    exploration_rate: 0.20
    min_explore_dispatches: 1000
    new_producer_grace_period: 600s
    uncertainty_weight: 1.25
    fresh_producer_bonus: 0.75
```

其中 `exploration_mode` 支持两种模式：

- `alternating`：在存在 exploration candidate 时，exploration / exploitation 交替执行；
- `probabilistic`：每次按 `exploration_rate` 概率进入 exploration，否则进入 exploitation。

#### Exploration 层

以下 producer 会被视为 exploration candidate：

- `DispatchAttempts < min_explore_dispatches`；
- 或者注册时间距离当前时间小于 `new_producer_grace_period`。

在当前实验配置中，任一 builder 只要累计 dispatch 样本数少于 `1000`，或者注册后仍处在 `600s` grace period 内，就会进入 exploration candidate 集合。

Exploration 层不是直接选择 UCB 分数最大的 builder，而是先计算每个候选 builder 的 bandit score，再按 bandit score 做加权随机抽样。这样可以避免单个候选因短期高分完全垄断 exploration 流量。

基础预期收益：

```text
reward_mean = clamp(AverageReward, -2.0, 2.0)
reward_multiplier = max(1 + reward_mean, 0.10)
expectedReward = max(Score, 0.5) * reward_multiplier
```

探索基础权重：

```text
explorationWeight = max(expectedReward, 0.5)
```

如果样本数不足，则加入缺样本加权：

```text
missing = min_explore_dispatches - DispatchAttempts
sample_bonus_multiplier =
    1 + uncertainty_weight * (missing / min_explore_dispatches)

explorationWeight *= sample_bonus_multiplier
```

如果 builder 仍处在新 producer grace period，则加入冷启动 bonus：

```text
freshness = 1 - age / new_producer_grace_period
fresh_bonus_multiplier = 1 + fresh_producer_bonus * freshness

explorationWeight *= fresh_bonus_multiplier
```

随后加入与 dispatch 次数相关的不确定性权重：

```text
explorationWeight *= 1 + uncertainty_weight / sqrt(DispatchAttempts + 1)
```

最后计算 UCB-style bonus：

```text
totalPulls = sum(all_builder.DispatchAttempts) + 1
ucbBonus =
    uncertainty_weight
  * sqrt( ln(totalPulls + 1) / (RewardSamples + 1) )
```

最终 bandit score：

```text
banditScore = explorationWeight + ucbBonus
```

这里的 `banditScore` 不是单独持久化的状态字段，也没有额外的“bandit score 更新表”。它是在每次 dispatch 选择 target producer 时，基于当时最新的 `Score`、`AverageReward`、`DispatchAttempts`、`RewardSamples` 和注册时间实时计算出来的。真正被持久累计的是 builder 的行为统计和 reward 统计；这些统计变化后，下一次计算出的 bandit score 会自然变化。

因此，探索阶段实际偏向三类 producer：

- 信誉和收益已经表现较好的 producer；
- 样本数不足、仍有较高不确定性的 producer；
- 新注册且还处于 grace period 的 producer。

但由于最终仍是加权随机抽样，低分 producer 只要仍在候选集中，也保留一定探索概率。

#### Exploitation 层

系统进入 exploitation 层的情况包括：

- `exploration_enabled = false`；
- 当前不存在 exploration candidate；
- `exploration_mode = alternating` 且本轮轮到 exploitation；
- `exploration_mode = probabilistic` 且随机数未命中 `exploration_rate`。

Exploitation 层使用 `expectedReward` 作为权重：

```text
expectedReward = max(Score, 0.5) * max(1 + clamp(AverageReward, -2.0, 2.0), 0.10)
```

其中：

- `Score` 表示长期信誉和风险控制结果；
- `AverageReward` 表示历史收益反馈；
- `reward_multiplier` 最低为 `0.10`，避免某个 builder 因短期负 reward 被完全打成零概率；
- 最终权重也会再通过 `minEffectiveScore = 0.5` 做下限保护。

#### Target Producer 加权抽样

无论 exploration 还是 exploitation，最终 target producer 都不是简单取最大值，而是加权随机选择。

对候选集合中的每个 builder 计算：

```text
weight_i = max(scoreFn(builder_i), 0.5)
totalWeight = sum(weight_i)
```

其中：

- exploration 层：`scoreFn = banditScore`
- exploitation 层：`scoreFn = expectedReward`

随后生成：

```text
pick = random(0, 1) * totalWeight
```

按候选列表顺序累加权重，选中第一个满足：

```text
cumulativeWeight > pick
```

的 builder 作为 target producer。

因此，某个 builder 被选中的近似概率为：

```text
P(builder_i) = weight_i / totalWeight
```

这种设计让高信誉、高 reward、低风险的 builder 获得更高流量份额，但不会让其他候选 builder 的概率直接归零，配合 exploration 层和 competition factor，可以持续保留发现新 producer、惩罚作恶 producer、抑制流量过度集中的能力。

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
