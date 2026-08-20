# Mock 10000 区块 Rebate 实验报告

## 1. 实验目的

本实验使用 mock simulator 长时间运行，验证以下行为：

1. score 在 0–100 百分制下是否稳定运行；
2. 正常 builder 是否缓慢获得更高分；
3. 恶意 builder 是否下降更快；
4. score 是否存在上限，不会无限增长；
5. 新 builder 是否能在实验中途加入并获得冷启动探索流量；
6. exploration / exploitation 是否长期共存。

## 2. 实验配置

配置文件：

`config/experiment_mock_5builders.yaml`

关键配置：

- simulator：`mock`
- 目标区块数：约 10000
- block summary 实际记录：14964
- bundle 目标区块范围：约 10174 个
- 区块间隔：10ms
- user：每个新区块随机发送 `0..5` 个 bundle
- exploration mode：`alternating`
- exploration rate：`0.20`（交替模式下不作为随机概率）
- 最少探索次数：1000
- score 范围：`0.5–100`
- 正向 score 目标最多为 `BaseScore + 15`
- 正向更新步长：目标差值的 `1%`
- 负向更新步长：目标差值的 `5%`
- 竞争集中度最大惩罚：`15%`
- 高于平均分发份额的集中度惩罚率：`0.35`
- 相对平均 reward 惩罚率：`0.10`

初始 builder：

| Builder | Base score | 角色 |
|---|---:|---|
| `alpha` | 85 | 高质量、稳定成功 |
| `beta` | 67 | 正常、高成功率 |
| `gamma` | 50 | 正常、少量失败 |
| `delta` | 27 | 低质量、较多失败 |
| `epsilon` | 13 | 恶意、sandwich 行为 |
| `theta` | 85 | 初始高分，后半程频繁作恶 |

运行约 3000 个区块后，通过新增的运行时接口注册：

```http
POST /builders/register
```

注册：

- `builder-zeta`
- 初始 score：60
- 角色：中途加入的正常 builder

此外，`theta` 在实验开始时就以 85 分加入，前半程按正常 builder 运行；从约 6000 区块开始，持续注入低成功率和高频 sandwich 观测，用于验证高信誉 builder 的后续惩罚。

之后每隔约 1000 个区块注入一轮行为观测：

- `alpha/beta/gamma/zeta`：高成功率、well-behaved、正向价值；
- `delta`：较低成功率，但没有 sandwich；
- `epsilon`：较低成功率，并持续注入 sandwich 行为。

## 3. 总体结果

| 指标 | 结果 |
|---|---:|
| 区块 summary | 14964 |
| bundle events | 25618 |
| dispatch events | 25618 |
| builder snapshot events | 25674 |
| exploration dispatch | 12809 |
| exploitation dispatch | 12809 |
| bundle 仿真成功率 | 100% |
| bundle-level MEV profit | 0.295325812 ETH |
| bundle-level refundable value | 0.029532581 ETH |

实验目标区块范围已达到约 10000 个。由于 server 在 user 停止后仍继续推进了一段时间，block summary 多记录了部分 shutdown 延迟期间的区块；bundle 目标区块范围为 `1000799..1010972`，共约 10174 个。

## 4. Builder 分发结果

| Builder | Dispatch | 占比 | Exploration | Exploitation |
|---|---:|---:|---:|---:|
| `alpha` | 6050 | 23.6% | 2957 | 3093 |
| `beta` | 5054 | 19.7% | 2505 | 2549 |
| `gamma` | 4170 | 16.3% | 2114 | 2056 |
| `delta` | 2542 | 9.9% | 1287 | 1255 |
| `epsilon` | 460 | 1.8% | 267 | 193 |
| `theta` | 3518 | 13.7% | 1679 | 1839 |
| `zeta` | 3824 | 14.9% | 2000 | 1824 |

`zeta` 在运行中才注册，但仍然获得了 3824 次 dispatch，其中 2000 次来自 exploration，说明运行时加入的 builder 能够进入冷启动流程。

恶意 `epsilon` 获得了 460 次 dispatch，占比 1.8%；高分恶意 `theta` 在作恶后仍保留了一部分历史流量，但 score 下降后其后续权重明显降低。alpha 的分发占比为 23.6%，仍未形成绝对集中。

各 builder 的总分发 bundle 数和被选中的不同目标区块数如下。本轮同一目标区块可能包含多个 bundle，因此两项数值不同；统计脚本会对目标区块自动去重。

| Builder | 总分发 bundle | 被选中区块数 |
|---|---:|---:|
| `alpha` | 6050 | 4455 |
| `beta` | 5054 | 3894 |
| `gamma` | 4170 | 3381 |
| `delta` | 2542 | 2232 |
| `epsilon` | 460 | 432 |
| `theta` | 3518 | 2775 |
| `zeta` | 3824 | 2947 |

## 5. Score 变化

最终 score：

| Builder | 初始 score | 最终 score | 变化 |
|---|---:|---:|---:|
| `alpha` | 85 | 97.16 | +12.16 |
| `beta` | 67 | 80.61 | +13.61 |
| `gamma` | 50 | 64.55 | +14.55 |
| `delta` | 27 | 39.63 | +12.63 |
| `epsilon` | 13 | 1.70 | -11.30 |
| `theta` | 85 | 19.12 | -65.88 |
| `zeta` | 60 | 74.78 | +14.78 |

实验配置中的 alpha 初始分为 85。当前轮 alpha 最终为 97.16，分发占比为 23.6%；评分更新仍根据各 builder 的分发份额和相对平均 reward 计算竞争因子，集中度惩罚最多为 15%。因此，表现稳定且 well-behaved 的 alpha 仍保持较高分，但不会无限制增长或完全独占订单流。

当前 dispatch 结束时的 snapshot score 为 alpha `97.16`、beta `80.61`、gamma `64.55`、delta `39.63`、epsilon `1.70`、theta `19.12`、zeta `74.78`。

## 6. 结果分析

### 6.1 正常 builder

正常 builder 的 score 是渐进变化的：

- `alpha`：85 → 97.16，在保持高质量的同时通过交替分层避免无限增长；
- `beta`：67 → 80.61；
- `gamma`：50 → 64.55；
- `zeta`：60 → 74.78。

单次观测只按目标差值的 1% 更新，因此不会因为一次成功或一次高价值事件立即跳到高分。与此同时，目标分最多为 `BaseScore + 15`，且全局上限为 100，避免正常 builder 无限增长。

### 6.2 恶意 builder

`epsilon` 持续收到低成功率和 sandwich 观测：

- sandwich events：40；
- dispatch attempts：860；
- dispatch successes：700；
- 最终 score：`1.30`。

负向更新步长为 5%，高于正常上升的 1%，因此恶意 builder 的下降速度相对更快，但仍然是平滑下降，而不是单次归零。

### 6.3 新 builder

`zeta` 在运行中途加入，初始 score 为 60，注册后立刻被标记为 exploration candidate。最终获得 3824 次分发，其中 2000 次为 exploration，并逐步上升到约 74.78 分，说明新 builder 不会因为加入时间晚而完全没有机会。

### 6.4 高分 builder 后续作恶

`theta` 初始 score 为 85，前半程没有恶意行为；从约 6000 区块开始注入高频恶意观测后，score 快速下降至 19.12：

- 作恶后最终 snapshot：`19.12`；
- sandwich events：40；
- dispatch attempts：4068；
- dispatch successes：3915。

这验证了 score 惩罚不依赖低初始信誉：高分 builder 只要持续作恶，同样会被快速降权。

### 6.5 长期分发策略

本轮 user 通过 `/blocks` SSE 消费每个新区块事件，并为每个事件随机生成 `0..5` 个 bundle；服务端使用 32 个 mock simulation workers。最终有效 bundle 为 25618 个，目标区块范围约 10174 个，平均约 `2.52` 个 bundle/区块，符合 `0..5` 均匀随机分布的期望均值。

按目标区块统计，0 个 bundle 的区块为 1739 个，1、2、3、4、5 个 bundle 的区块分别为 1618、1616、1730、1777、1694 个；只有包含至少一个 bundle 的 8435 个区块会出现在 bundle 日志中。

交替模式在每次存在探索候选时切换分层：

- exploration：12809 / 25618，50%；
- exploitation：12809 / 25618，50%；
- 相邻 dispatch 分层发生切换：25617 / 25617。

因此，交替模式不再由 `rate` 随机决定层级，而是严格保持 1:1 的 exploration/exploitation；`rate` 参数仅保留用于 probabilistic 模式。

## 7. 图表与原始数据

实验目录：

`logs/experiment_mock_5builders/`

原始数据：

- `block_summary.jsonl`
- `bundle_events.jsonl`
- `builder_dispatches.jsonl`
- `builder_snapshots.jsonl`
- `metadata.json`
- `plots/summary.json`：包含每个 builder 的 `total_built_blocks` 和 `total_distributed_bundles`

图表：

![Block Profit And Refund](../logs/experiment_mock_5builders/plots/block_profit_refund.png){ width=95% }

![Block Success Rate](../logs/experiment_mock_5builders/plots/block_success_rate.png){ width=95% }

![Dispatch Layer By Block](../logs/experiment_mock_5builders/plots/dispatch_layer_by_block.png){ width=95% }

![Builder Score Trends](../logs/experiment_mock_5builders/plots/builder_score_trends.png){ width=95% }

![Builder Dispatch Mix](../logs/experiment_mock_5builders/plots/builder_dispatch_mix.png){ width=95% }

![Builder Block Counts](../logs/experiment_mock_5builders/plots/builder_block_counts.png){ width=95% }

![Bundle Success Profit Trend](../logs/experiment_mock_5builders/plots/bundle_success_profit_trend.png){ width=95% }

## 8. 说明与限制

1. mock simulator 的历史交易数量按区块随机生成，普通区块为 100–500 笔，少量区块为 1000–5000 笔；当前实验日志没有单独持久化每个区块的历史交易数量，只在模拟 block context 中使用。
2. mock builder endpoint 全部返回成功，因此 dispatch 成功率主要验证分发链路，不代表真实网络可用性。
3. 本轮的 sandwich 行为通过 `/builders/observe` 注入，验证的是 score 惩罚和流量抑制，不是实际攻击检测器。
4. 10ms 区块推进速度高于单个 mock simulation 的处理延迟，部分 bundle 的 block summary 已经在 bundle 处理前完成，因此区块级 profit summary 可能低估；本报告使用 bundle-level profit 作为 mock 收益指标。
5. 真实交易 replay 模式仍保留，百分制 score 和新的平滑更新逻辑对 replay 同样生效。
6. exploration/exploitation 参数和 score 模型参数已写入 `metadata.json`，并嵌入绘图脚本生成的 `plots/summary.json`。
