# Rebate Experiments


## 1. 实验目的

本轮实验主要验证以下几点：

1. 实验指标是否能够稳定落盘到结构化文件；
2. exploration / exploitation 分发是否都能在实际运行中出现；
3. builder score、dispatch、block-level profit 等关键指标;

## 2. 实验配置

- 区块间隔：`2s`
- 用户发送间隔：`5s`
- searcher 最大链深：`2`
- 实验方式： `105` 条 block summary
- 数据集特征：共 `4048` 条交易，覆盖 `105` 个区块
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

## 4. 数据规模

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

## 6. 图表

### 6.1 Block Profit And Refund

区块级 profit 与 refundable value 走势：

![Block Profit And Refund](../logs/experiment/plots/block_profit_refund.png){ width=95% }

### 6.2 Block Success Rate

区块级 bundle success rate 与 bundle volume：

![Block Success Rate](../logs/experiment/plots/block_success_rate.png){ width=95% }

### 6.3 Dispatch Layer By Block

每个区块的 exploration / exploitation 分布：

![Dispatch Layer By Block](../logs/experiment/plots/dispatch_layer_by_block.png){ width=95% }

### 6.4 Builder Score Trends

builder 有效分数变化趋势：

![Builder Score Trends](../logs/experiment/plots/builder_score_trends.png){ width=95% }

### 6.5 Builder Dispatch Mix

builder 分发量和成功率对比：

![Builder Dispatch Mix](../logs/experiment/plots/builder_dispatch_mix.png){ width=95% }

### 6.6 Bundle Success Profit Trend

bundle 成功率滚动窗口与单 bundle profit 走势：

![Bundle Success Profit Trend](../logs/experiment/plots/bundle_success_profit_trend.png){ width=95% }


## 7. 结论

1. 当前能够稳定输出 block、bundle、builder dispatch、builder snapshot 四类结构化数据；
2. 当前样本已经能够观察到 exploration / exploitation 共存、builder score 正向更新，以及重排造成的交易挤出效应。
