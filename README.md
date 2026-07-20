# Validator-MEV-Rebate

MEV rebate with emphasis on validator performance



具体思路：可以适用于不同链(ethereum + solana等等)，都是分成两个部分，一部分是给block producer的风险评估，打分，另一部分是订单流分配机制，在通过返利来吸引普通用户的交易，再根据评分来将收集到的订单流分配给不同的block producer.

还可以拓展的一些点：

- 具体的利益分配
- 不仅仅根据过去的表现进行评估，还增加预测模型， 判断未来作恶可能性（结合神经网络模型等）
- 将滑点纳入订单流分配考虑
- 不同情况下的测试，比如恶意节点占比很高，交易流很少/很多


https://mev-share.flashbots.net/

  启动方式：                                                                                                                      
  # 使用默认路径 config/config.yaml                                                                                               
  go run ./cmd/server/main.go                                                                                                     
                                                                                                                                  
  # 指定配置文件                                                                                                                  
  go run ./cmd/server/main.go -config /path/to/my-config.yaml                                                                     
                                                                                                                                  
  环境变量也可以覆盖配置，例如 REBATE_SERVER_PORT=9090。


方向五:MEV 收益的区块内再分配(smoothing),而不是点对点分成
现在 MEV-Share 的分润模式是"谁被 backrun,谁分钱",这其实制造了一个新的博弈:searcher 仍然有动机去精确定位、瞄准某个具体用户交易。builder 由于能看到整个区块的全貌,其实有能力做一件 searcher 做不到的事——把这个区块里所有由订单流本身创造出来的 MEV,按贡献比例(而不是按"谁被打中")分给这个区块里的所有订单流提供方。这样一来,攻击某个特定用户不再比"雨露均沾"更有利可图,某种程度上削弱了精准狙击单个用户的经济动机。这个方向目前还比较偏研究/构想阶段,没有看到大规模落地的实现,是相对有创新空间的部分。