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
- 实际推进区块：约 10807
- 区块间隔：10ms
- user bundle 间隔：100ms
- exploration rate：`0.20`
- 最少探索次数：5
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
| 区块 summary | 10643 |
| bundle events | 857 |
| dispatch events | 857 |
| builder snapshot events | 913 |
| exploration dispatch | 178 |
| exploitation dispatch | 679 |
| bundle 仿真成功率 | 100% |
| bundle-level MEV profit | 0.009856711 ETH |
| bundle-level refundable value | 0.000985671 ETH |

10000 区块目标已经达到，实际多出的区块来自停止时的轮询和进程退出延迟。

## 4. Builder 分发结果

| Builder | Dispatch | 占比 | Exploration | Exploitation |
|---|---:|---:|---:|---:|
| `alpha` | 207 | 24.2% | 42 | 165 |
| `beta` | 155 | 18.1% | 37 | 118 |
| `gamma` | 140 | 16.3% | 27 | 113 |
| `delta` | 69 | 8.1% | 15 | 54 |
| `epsilon` | 10 | 1.2% | 2 | 8 |
| `theta` | 131 | 15.3% | 25 | 106 |
| `zeta` | 145 | 16.9% | 30 | 115 |

`zeta` 在运行中才注册，但仍然获得了 145 次 dispatch，其中 30 次来自 exploration，说明运行时加入的 builder 能够进入冷启动流程。

恶意 `epsilon` 只获得了 10 次 dispatch，占比 1.2%；高分恶意 `theta` 在作恶后仍保留了一部分历史流量，但 score 下降后其后续权重明显降低。alpha 的分发占比为 24.2%，未形成对其他 builder 的绝对集中，说明竞争惩罚对流量集中产生了温和的抑制作用。

各 builder 的总分发 bundle 数和被选中的不同目标区块数如下。由于本轮每个目标区块只产生一次 dispatch，因此两项数值相同；统计脚本会对重复目标区块自动去重。

| Builder | 总分发 bundle | 被选中区块数 |
|---|---:|---:|
| `alpha` | 207 | 207 |
| `beta` | 155 | 155 |
| `gamma` | 140 | 140 |
| `delta` | 69 | 69 |
| `epsilon` | 10 | 10 |
| `theta` | 131 | 131 |
| `zeta` | 145 | 145 |

## 5. Score 变化

最终 score：

| Builder | 初始 score | 最终 score | 变化 |
|---|---:|---:|---:|
| `alpha` | 85 | 98.84 | +13.84 |
| `beta` | 67 | 81.37 | +14.37 |
| `gamma` | 50 | 64.53 | +14.53 |
| `delta` | 27 | 36.59 | +9.59 |
| `epsilon` | 13 | 1.30 | -11.70 |
| `theta` | 85 | 16.53 | -68.47 |
| `zeta` | 60 | 74.48 | +14.48 |

实验配置中的 alpha 初始分为 85。新一轮实验中 alpha 最终为 98.84，而不是固定为 100；评分更新根据各 builder 的分发份额和相对平均 reward 计算竞争因子，集中度惩罚最多为 15%。因此，表现稳定且 well-behaved 的 alpha 仍保持较高分，但其流量份额和评分会受到其他竞争 builder 的温和制衡。

阶段性 score：

| 阶段 | beta | gamma | delta | epsilon | theta | zeta |
|---|---:|---:|---:|---:|---:|---:|
| 初始/注册后 | 95.35 | 79.82 | 62.86 | 16.22 | 93.91 | 60.66 |
| 阶段 2 | 97.16 | 80.81 | 63.91 | 1.57 | 96.71 | 69.42 |
| 阶段 3 | 98.11 | 81.21 | 64.40 | 1.40 | 98.15 | 72.96 |
| 作恶开始后 | 98.59 | 81.40 | 64.58 | 1.36 | 94.75 | 74.22 |
| 阶段 5 | 98.73 | 81.42 | 64.61 | 1.33 | 19.99 | 74.53 |
| 阶段 6 | 98.77 | 81.42 | 64.58 | 1.30 | 17.87 | 74.62 |
| 阶段 7 | 98.80 | 81.40 | 64.55 | 1.30 | 17.32 | 74.58 |
| 最终阶段 | 98.81 | 81.37 | 64.52 | 1.30 | 16.84 | 74.51 |

最终 dispatch 结束时的 snapshot score 为 alpha `98.84`、beta `81.37`、gamma `64.53`、delta `36.59`、epsilon `1.30`、theta `16.53`、zeta `74.48`。阶段采样和最终 snapshot 的时间点略有差异。

## 6. 结果分析

### 6.1 正常 builder

正常 builder 的 score 是渐进变化的：

- `alpha`：85 → 98.84，在保持高质量的同时受到竞争因子约束；
- `beta`：67 → 81.37；
- `gamma`：50 → 64.53；
- `zeta`：60 → 74.48。

单次观测只按目标差值的 1% 更新，因此不会因为一次成功或一次高价值事件立即跳到高分。与此同时，目标分最多为 `BaseScore + 15`，且全局上限为 100，避免正常 builder 无限增长。

### 6.2 恶意 builder

`epsilon` 持续收到低成功率和 sandwich 观测：

- sandwich events：40；
- dispatch attempts：410；
- dispatch successes：250；
- 最终 score：`1.30`。

负向更新步长为 5%，高于正常上升的 1%，因此恶意 builder 的下降速度相对更快，但仍然是平滑下降，而不是单次归零。

### 6.3 新 builder

`zeta` 在运行中途加入，初始 score 为 60，注册后立刻被标记为 exploration candidate。最终获得 145 次分发，其中 30 次为 exploration，并逐步上升到约 74.48 分，说明新 builder 不会因为加入时间晚而完全没有机会。

### 6.4 高分 builder 后续作恶

`theta` 初始 score 为 85，前半程没有恶意行为，阶段采样中先升到约 98.15；从约 6000 区块开始注入高频恶意观测后，score 快速下降：

- 作恶前：约 `98.15`；
- 作恶后阶段：`19.99`、`17.87`、`17.32`、`16.84`；
- 最终 snapshot：`16.53`；
- sandwich events：40；
- dispatch attempts：681；
- dispatch successes：528。

这验证了 score 惩罚不依赖低初始信誉：高分 builder 只要持续作恶，同样会被快速降权。

### 6.5 长期分发策略

整个实验中：

- exploration：178 / 857，约 20.8%；
- exploitation：679 / 857，约 79.2%。

实际分布接近配置中的 20% exploration rate，同时新 builder 的冷启动流量也被覆盖。

## 7. 图表与原始数据

实验目录：

`logs/experiment_mock_5builders/`

原始数据：

- `block_summary.jsonl`
- `bundle_events.jsonl`
- `builder_dispatches.jsonl`
- `builder_snapshots.jsonl`
- `score_snapshots.jsonl`
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
