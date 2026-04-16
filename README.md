# Validator-MEV-Rebate

MEV rebate with emphasis on validator performance



具体思路：可以适用于不同链(ethereum + solana等等)，都是分成两个部分，一部分是给block producer的风险评估，打分，另一部分是订单流分配机制，在通过返利来吸引普通用户的交易，再根据评分来将收集到的订单流分配给不同的block producer.

还可以拓展的一些点：

- 具体的利益分配
- 不仅仅根据过去的表现进行评估，还增加预测模型， 判断未来作恶可能性（结合神经网络模型等）
- 将滑点纳入订单流分配考虑
- 不同情况下的测试，比如恶意节点占比很高，交易流很少/很多
