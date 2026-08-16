# Rebate 五 Builder、100 区块实验报告

## 1. 实验目的

本实验使用当前 100 个区块、33303 笔交易的数据集，验证 rebate 的 builder 分发和动态信誉机制是否能够：

1. 在多个 builder 冷启动时触发 exploration；
2. 根据质量和行为信号形成不同的动态 score；
3. 对恶意行为降低 builder 的后续分发权重；
4. 在 score 更新后从 exploration 逐步转向 exploitation。

## 2. 实验配置

实验配置文件：

`config/experiment_5builders.yaml`

关键配置：

- 数据集：`data/ethereum_transactions.csv`
- 数据规模：100 个区块，33303 笔交易
- 区块回放间隔：1 秒
- 用户 bundle 发送间隔：1 秒
- searcher 最大链深：2
- exploration rate：`0.20`
- 最少探索次数：每个新 builder `5` 次
- 新 builder grace period：600 秒

Builder 初始配置：

| Builder | 初始 score | 实验角色 |
|---|---:|---|
| `builder-alpha` | 3.0 | 高质量：高成功率、高价值、well-behaved |
| `builder-beta` | 2.0 | 中高质量：少量失败、较高价值 |
| `builder-gamma` | 1.5 | 中低质量：失败、少量 sandwich、少量正向行为 |
| `builder-delta` | 0.8 | 低质量：较多失败、sandwich 行为 |
| `builder-epsilon` | 0.4 | 恶意 builder：低成功率、大量 sandwich 行为 |

实验开始后，先运行约 25 秒，让所有 builder 在冷启动状态下竞争；随后通过 `/builders/observe` 注入行为观测：

- `alpha`：20/20 成功，20 次 well-behaved，5 ETH value；
- `beta`：18/20 成功，10 次 well-behaved，0.5 ETH value；
- `gamma`：12/20 成功，1 次 sandwich，2 次 well-behaved；
- `delta`：4/10 成功，2 次 sandwich；
- `epsilon`：1/10 成功，8 次 sandwich。

## 3. 总体结果

| 指标 | 结果 |
|---|---:|
| 区块数 | 100 |
| bundle events | 196 |
| dispatch events | 196 |
| bundle 仿真成功率 | 100% |
| exploration dispatch | 33 |
| exploitation dispatch | 163 |
| MEV profit | 0.002217401 ETH |
| refundable value | 0.000221740 ETH |
| 有 MEV profit 的区块 | 88 / 100 |
| 平均被挤出交易数 | 1.49 |
| 最大被挤出交易数 | 2 |

实验原始数据：

- `logs/experiment_5builders/block_summary.jsonl`
- `logs/experiment_5builders/bundle_events.jsonl`
- `logs/experiment_5builders/builder_dispatches.jsonl`
- `logs/experiment_5builders/builder_snapshots.jsonl`

## 4. Builder 分发结果

| Builder | Dispatch | 占比 | Exploration | Exploitation | 成功率 |
|---|---:|---:|---:|---:|---:|
| `builder-alpha` | 91 | 46.4% | 9 | 82 | 100% |
| `builder-beta` | 71 | 36.2% | 17 | 54 | 100% |
| `builder-gamma` | 18 | 9.2% | 2 | 16 | 100% |
| `builder-delta` | 13 | 6.6% | 3 | 10 | 100% |
| `builder-epsilon` | 3 | 1.5% | 2 | 1 | 100% |

冷启动阶段共发生 82 次 dispatch，5 个 builder 都获得过流量：

| Builder | 冷启动阶段 dispatch |
|---|---:|
| `alpha` | 31 |
| `beta` | 28 |
| `gamma` | 10 |
| `delta` | 10 |
| `epsilon` | 3 |

这说明 exploration 能够让低初始分的 builder 获得测试机会；其中 `epsilon` 在受到恶意行为惩罚前仍获得了少量冷启动流量。

## 5. Score 学习结果

| Builder | Base score | 最终 effective score | 成功/尝试 | Sandwich | Well-behaved | Valuable flow |
|---|---:|---:|---:|---:|---:|---:|
| `alpha` | 3.0 | 7.8007 | 111/111 | 0 | 20 | 91 |
| `beta` | 2.0 | 4.5905 | 89/91 | 0 | 10 | 70 |
| `gamma` | 1.5 | 1.7094 | 30/38 | 1 | 2 | 19 |
| `delta` | 0.8 | 0.5948 | 17/23 | 2 | 0 | 13 |
| `epsilon` | 0.4 | 0.0500 | 4/13 | 8 | 0 | 3 |

`epsilon` 的 score 从 `0.4` 降到系统最小有效分 `0.05`。在恶意观测注入后，剩余实验阶段没有新的 `epsilon` dispatch；这与机制设计一致：恶意行为通过 sandwich penalty、失败率和 score 下限共同降低后续被选中的概率。

相对地，`alpha` 的高成功率、正向行为和价值产出使 score 提升到 `7.8007`，最终获得最多流量。`beta` 也保持较高 score，但因为初始信誉和行为信号弱于 `alpha`，分发量低于 `alpha`。

## 6. Rebate 与 MEV mitigation 观察

本轮实验能验证的 mitigation 链路是：

1. 冷启动阶段不直接排除低分 builder，而是通过 exploration 给它们有限试用机会；
2. builder 的成功率、价值产出、well-behaved 和 sandwich 信号进入动态信誉计算；
3. 恶意行为不会立即阻断系统，但会使 builder score 快速降低；
4. score 降低后，builder 从正常竞争流量中退出，只保留理论上的最低探索权重；
5. 高质量 builder 获得更多后续订单流，形成 exploitation。

在本实验中，`epsilon` 的分发占比只有 `1.5%`，而 `alpha + beta` 合计获得 `82.7%` 的分发。恶意 builder 没有成为主要订单流接收方，说明当前信誉惩罚对降低恶意 builder 的暴露面有效。

同时，100 个区块中有 88 个区块产生 MEV profit，累计 profit 为 `0.002217401 ETH`，说明回放数据足以覆盖不同收益强度的 bundle 场景。

## 7. 图表

![Block Profit And Refund](../logs/experiment_5builders/plots/block_profit_refund.png){ width=95% }

![Block Success Rate](../logs/experiment_5builders/plots/block_success_rate.png){ width=95% }

![Dispatch Layer By Block](../logs/experiment_5builders/plots/dispatch_layer_by_block.png){ width=95% }

![Builder Score Trends](../logs/experiment_5builders/plots/builder_score_trends.png){ width=95% }

![Builder Dispatch Mix](../logs/experiment_5builders/plots/builder_dispatch_mix.png){ width=95% }

![Bundle Success Profit Trend](../logs/experiment_5builders/plots/bundle_success_profit_trend.png){ width=95% }

## 8. 限制与后续实验

1. 5 个 builder 当前都是 mock endpoint，实际发送层全部返回成功；因此表中的 dispatch 成功率不能代表真实网络可用性。
2. 恶意行为通过 `/builders/observe` 注入，而不是由 mock builder 自动执行 sandwich；本实验验证的是信誉反馈和流量抑制机制，不是攻击检测器本身。
3. `epsilon` 被惩罚后仍保留最低 score，但本轮随机样本中没有再次被选中；需要更长回放或更高探索率才能测量最低探索权重的长期暴露。
4. 当前 reward 和 score 是在线累计模型，人工注入的观测与真实 dispatch observation 共用统计量，后续应分别记录“真实观测”和“外部报告”来源。
5. 要做更强的因果评估，应增加 baseline：关闭 rebate 的均匀分发、固定高分 builder 分发，并比较恶意流量占比、成功率、MEV profit 和 refund。

## 9. 复现实验

```bash
cd eth/rebate
go build -o ./server ./cmd/server
go build -o ./searcher ./cmd/searcher
go build -o ./user ./cmd/user

./server -config config/experiment_5builders.yaml
./searcher -server http://localhost:8080 \
  -events http://localhost:8080/events \
  -dataset data/ethereum_transactions.csv \
  -max-chain-depth 2
./user -server http://localhost:8080 \
  -dataset data/ethereum_transactions.csv \
  -interval 1s
```

运行过程中按本报告第 2 节的观测参数调用 `/builders/observe`，直到 `logs/experiment_5builders/block_summary.jsonl` 达到 100 行，然后生成图表：

```bash
MPLBACKEND=Agg python3 scripts/plot_experiment_metrics.py \
  --input-dir logs/experiment_5builders \
  --output-dir logs/experiment_5builders/plots
```
