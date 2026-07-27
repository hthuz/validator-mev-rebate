# Rebate Experiment Metrics

实验数据默认落在 `logs/experiment/`，用于后续实验复盘和汇报绘图。

## 输出文件

- `bundle_events.jsonl`
  - 每个 bundle 一条记录
  - 包含目标区块、searcher、是否 backrun、仿真成功与否、profit/refund/gas、插入位置、被挤出交易数
- `builder_dispatches.jsonl`
  - 每次 builder 分发一条记录
  - 包含 exploration / exploitation 层、builder 分数、expected reward、bandit score、分发是否成功
- `builder_snapshots.jsonl`
  - 每次 builder 学习更新后的快照
  - 包含 attempts/successes/failures、sandwich/well-behaved、reward、effective score
- `block_summary.jsonl`
  - 每个区块结束时一条汇总
  - 包含 bundle 数、成功率、总 profit、总 refundable、gas 使用、builder 分布

## 绘图脚本

依赖：`matplotlib`

```bash
python3 -m pip install matplotlib
python3 scripts/plot_experiment_metrics.py \
  --input-dir logs/experiment \
  --output-dir logs/experiment/plots
```

默认会生成：

- `block_profit_refund.png`
- `block_success_rate.png`
- `dispatch_layer_by_block.png`
- `builder_score_trends.png`
- `builder_dispatch_mix.png`
- `bundle_success_profit_trend.png`
- `summary.json`
