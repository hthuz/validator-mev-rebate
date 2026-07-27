# Rebate Experiments

本文档汇总 2026-07-27 这轮 `rebate` 本地实验的运行结果，便于后续技术报告、实验对比和图表引用。

本次更新后的实验目标是累计至少 `100` 个区块样本。由于当前 replay 数据集只包含 `15` 个唯一区块，因此本轮实验采用多轮 replay 累积的方式，最终得到 `105` 条 block-level 样本。

## 1. 实验目的

本轮实验主要验证以下几点：

1. 实验指标是否能够稳定落盘到结构化文件；
2. exploration / exploitation 分发是否都能在实际运行中出现；
3. builder score、dispatch、block-level profit 等关键指标是否能被后续绘图脚本直接消费；
4. 在当前仅有 15 个 replay 区块的数据集条件下，是否仍然能通过多轮 replay 累积出 100+ 区块样本；
5. Python 绘图链路是否可直接复用到后续实验批次。

## 2. 实验配置

- 运行脚本：`scripts/run_demo.sh`
- 模拟模式：`replay`
- 数据集：`data/ethereum_transactions.csv`
- 区块间隔：`2s`
- 用户发送间隔：`5s`
- searcher 最大链深：`2`
- 实验方式：`7` 轮 replay 累积，最终得到 `105` 条 block summary
- 数据集特征：共 `4048` 条交易，覆盖 `15` 个唯一区块
- builder：
  - `builder-alpha`，基础分 `3.0`
  - `builder-beta`，基础分 `1.0`
- exploration 配置：
  - `enabled=true`
  - `rate=0.20`
  - `min_explore_dispatches=5`
  - `new_producer_age_seconds=600`
  - `uncertainty_weight=1.25`
  - `fresh_producer_bonus=0.75`

## 3. 输出产物

- 实验原始数据目录：[logs/experiment](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment)
- 图表输出目录：[logs/experiment/plots](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots)
- 指标说明文档：[docs/experiment_metrics.md](file:///Users/bytedance/validator-mev-rebate/eth/rebate/docs/experiment_metrics.md)

## 4. 数据规模

本轮实验最终落盘的数据量如下：

- `block_summary.jsonl`：105 条
- `bundle_events.jsonl`：83 条
- `builder_dispatches.jsonl`：82 条
- `builder_snapshots.jsonl`：82 条

汇总指标：

- `total_blocks = 105`
- `total_bundle_events = 83`
- `total_dispatch_events = 82`
- `bundle_success_rate = 0.9879518072289156`
- `total_mev_profit_eth = 0.001586`
- `unique_replay_block_numbers = 15`

## 5. 核心结果

### 5.1 分发层结果

- exploration 分发：19 次
- exploitation 分发：63 次
- 两种分发层都在实际运行中被触发，说明新增的实验记录已经能覆盖 bandit 策略的关键行为。

builder 分发统计：

- `builder-alpha`：80 次分发，成功率 100%
- `builder-beta`：2 次分发，成功率 100%

进一步看分层分发：

- `builder-alpha`：
  - exploration：18 次
  - exploitation：62 次
- `builder-beta`：
  - exploration：1 次
  - exploitation：1 次

这说明在当前样本下，绝大多数流量仍然集中在 `builder-alpha`，而 `builder-beta` 虽然仍能获得少量流量，但份额明显偏低。这也提示后续如果要更系统地观察冷启动 builder 的探索效果，仍然需要更丰富的 builder 行为注入或更长时程实验。

### 5.2 Builder 学习结果

本轮实验结束时的有效分数：

- `builder-alpha`：从 `4.5` 增长到 `4.500267798894659`
- `builder-beta`：从 `1.5000030967608016` 增长到 `1.500003175366126`

本轮没有引入 sandwich 攻击或 failure 注入，因此两者都表现为小幅正向增长，符合“成功分发 + 正收益”下的学习预期。与此同时，由于 `builder-alpha` 接收了绝大部分订单流，它的 score 增幅也明显高于 `builder-beta`。

### 5.3 区块级结果

- 有利润区块数：38 / 105
- 最大区块利润：`461666262756044 wei`
- 平均区块空间占用：`0.0009550146031746032`

说明在多轮 replay 之后，利润事件分布依然较稀疏，但已经足以稳定覆盖多个 profit 峰值区块，便于后续做区块利润与分发策略的对比分析。

### 5.4 重排影响结果

- 平均被挤出交易数：`32.19`
- 单个 bundle 最大被挤出交易数：`125`

这说明 replay simulator 已经能记录 bundle 插入后对原始区块排序造成的更明显扰动，适合后续做“重排强度”和“收益效果”的关联分析。

## 6. 图表索引

- [block_profit_refund.png](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/block_profit_refund.png)
  - 区块级 profit 与 refundable value 走势
- [block_success_rate.png](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/block_success_rate.png)
  - 区块级 bundle success rate 与 bundle volume
- [dispatch_layer_by_block.png](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/dispatch_layer_by_block.png)
  - 每个区块的 exploration / exploitation 分布
- [builder_score_trends.png](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/builder_score_trends.png)
  - builder 有效分数变化趋势
- [builder_dispatch_mix.png](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/builder_dispatch_mix.png)
  - builder 分发量和成功率对比
- [bundle_success_profit_trend.png](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/bundle_success_profit_trend.png)
  - bundle 成功率滚动窗口与单 bundle profit 走势
- [summary.json](file:///Users/bytedance/validator-mev-rebate/eth/rebate/logs/experiment/plots/summary.json)
  - 图表脚本输出的汇总统计

## 7. 结论

本轮实验可以得出四个直接结论：

1. 新增的实验记录链路已经跑通，能够稳定输出 block、bundle、builder dispatch、builder snapshot 四类结构化数据；
2. 绘图脚本已经可以直接消费这些 JSONL 文件并输出可用于汇报的图表；
3. 即使当前 replay 数据集只有 15 个唯一区块，也可以通过多轮 replay 累积得到 100+ block-level 样本，用于实验汇报；
4. 当前样本已经能够观察到 exploration / exploitation 共存、builder score 正向更新，以及重排造成的交易挤出效应。

## 8. 一键实验脚本

已新增一键实验脚本：[scripts/run_experiment_report.sh](file:///Users/bytedance/validator-mev-rebate/eth/rebate/scripts/run_experiment_report.sh)

用途：

- 清理旧实验数据；
- 启动 `server / searcher / user`；
- 支持按指定时长采集实验数据；
- 支持按 `TARGET_BLOCKS` 自动多轮 replay，直到累计足够的 block summary；
- 停止服务；
- 自动生成图表和 `summary.json`。

直接运行：

```bash
cd eth/rebate
./scripts/run_experiment_report.sh
```

如果想延长实验时长，例如跑 60 秒：

```bash
cd eth/rebate
RUN_SECONDS=60 ./scripts/run_experiment_report.sh
```

如果想按区块样本量跑，例如累计到 100 个区块样本：

```bash
cd eth/rebate
TARGET_BLOCKS=100 ./scripts/run_experiment_report.sh
```

脚本内容如下：

```bash
#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/logs"
EXPERIMENT_DIR="${LOG_DIR}/experiment"
PLOTS_DIR="${EXPERIMENT_DIR}/plots"

RUN_SECONDS="${RUN_SECONDS:-45}"
CLEAN_EXPERIMENT_DIR="${CLEAN_EXPERIMENT_DIR:-1}"
TARGET_BLOCKS="${TARGET_BLOCKS:-0}"
REPLAY_BLOCKS_PER_ROUND="${REPLAY_BLOCKS_PER_ROUND:-15}"

print_usage() {
  cat <<EOF
Usage:
  $(basename "$0")

Environment overrides:
  RUN_SECONDS             default: ${RUN_SECONDS}
  CLEAN_EXPERIMENT_DIR    default: ${CLEAN_EXPERIMENT_DIR}
  TARGET_BLOCKS           default: ${TARGET_BLOCKS}
  REPLAY_BLOCKS_PER_ROUND default: ${REPLAY_BLOCKS_PER_ROUND}

Example:
  RUN_SECONDS=60 ./scripts/$(basename "$0")
  TARGET_BLOCKS=100 ./scripts/$(basename "$0")
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  print_usage
  exit 0
fi

cd "${ROOT_DIR}"

if [[ "${CLEAN_EXPERIMENT_DIR}" == "1" ]]; then
  rm -rf "${EXPERIMENT_DIR}"
fi
mkdir -p "${EXPERIMENT_DIR}"

cleanup() {
  ./scripts/run_demo.sh stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "starting experiment..."
echo "  run_seconds=${RUN_SECONDS}"
echo "  target_blocks=${TARGET_BLOCKS}"
echo "  experiment_dir=${EXPERIMENT_DIR}"
echo

if [[ "${TARGET_BLOCKS}" -gt 0 ]]; then
  echo "collecting data until block_summary >= ${TARGET_BLOCKS} ..."
  round=0
  while true; do
    count=0
    if [[ -f "${EXPERIMENT_DIR}/block_summary.jsonl" ]]; then
      count="$(wc -l < "${EXPERIMENT_DIR}/block_summary.jsonl" | tr -d ' ')"
    fi
    if [[ "${count}" -ge "${TARGET_BLOCKS}" ]]; then
      break
    fi

    round=$((round + 1))
    target_count=$((count + REPLAY_BLOCKS_PER_ROUND))
    echo
    echo "starting replay round ${round} ..."
    echo "  current_block_count=${count}"
    echo "  round_target_count=${target_count}"
    ./scripts/run_demo.sh start

    while true; do
      count="$(wc -l < "${EXPERIMENT_DIR}/block_summary.jsonl" | tr -d ' ')"
      echo "  round=${round} block_count=${count}"
      if [[ "${count}" -ge "${target_count}" ]]; then
        break
      fi
      sleep 5
    done

    ./scripts/run_demo.sh stop
    echo "completed replay round ${round}"
  done
else
  ./scripts/run_demo.sh start

  echo
  echo "collecting data for ${RUN_SECONDS}s ..."
  sleep "${RUN_SECONDS}"

  echo
  echo "stopping services ..."
  ./scripts/run_demo.sh stop
fi

echo
echo "generating plots ..."
MPLBACKEND=Agg python3 scripts/plot_experiment_metrics.py \
  --input-dir "${EXPERIMENT_DIR}" \
  --output-dir "${PLOTS_DIR}"

echo
echo "experiment completed"
echo "raw data : ${EXPERIMENT_DIR}"
echo "plots    : ${PLOTS_DIR}"
echo "summary  : ${PLOTS_DIR}/summary.json"
```

## 9. 手动复现实验

```bash
cd eth/rebate
TARGET_BLOCKS=100 ./scripts/run_experiment_report.sh
```
