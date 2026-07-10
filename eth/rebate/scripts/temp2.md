# Builder Score 记录

之前Score机制没有完善，现在将score机制，生成高价值区块，无三明治攻击，失败率更低的builder，能获得更高的权重。

## Builder 部分相关日志

```md
{"level":"info","log_type":"builder_report","event":"builder_score_updated","builder":"builder-alpha","base_score":3,"effective_score":5.938624506276985,"attempts":17,"successes":17,"dispatch_failures":0,"sandwich_attacks":0,"well_behaved_events":5,"valuable_order_flow":10,"time":"2026-07-10T10:21:01+08:00","message":"builder score updated"}
{"level":"info","log_type":"builder_report","event":"builder_dispatch_selected","bundle_hash":"0x1ca466c54f1b913e4a656cdeb7d7d016788496dab7a300a22722d42c1994e64a","builder":"builder-alpha","url":"http://localhost:18545","base_score":3,"effective_score":5.938624506276985,"total_score":6.1552913041834545,"time":"2026-07-10T10:21:02+08:00","message":"builder dispatch selected"}
{"level":"info","log_type":"builder_report","event":"mock_builder_received_bundle","builder_addr":":18545","id":1,"params":[{"version":"v0.1","inclusion":{"block":"0x1838e64","maxBlock":"0x1838e65"},"body":[{"tx":"0x02f87301820460842faf08008445976db6830138809404d6e5de405fff9e0605d7ccbbcabbd6227462b3864d88c18f876080c001a056b2c34b75eb35ed0a8288662b236679c4ba67f4b7f7e12f400e44c9531a3b1ca06813362d90bcfd65fd94d1014cea6f26a27cd852b3f730e42e272278d7822434"}],"validity":{},"privacy":{"hints":87},"metadata":{"bundleHash":"0x1ca466c54f1b913e4a656cdeb7d7d016788496dab7a300a22722d42c1994e64a","bodyHashes":["0xeb4b64bf628e771f365fed8fcd93549cf576f069c8a4581cceb03c3bb17b3bb8"],"signer":"0x142175ff8a4da78bb4762fbb9d988abe541ae395","receivedAt":"0x6563864606eaf","matchingHash":"0x5a4e167629cb3dcd1fc57ae2ffd524e43e780cd535d36c1a64afb3be16e04db7"}}],"time":"2026-07-10T10:21:02+08:00","message":"mock builder received eth_sendMevBundle"}
{"level":"info","log_type":"builder_report","event":"builder_dispatch_result","bundle_hash":"0x1ca466c54f1b913e4a656cdeb7d7d016788496dab7a300a22722d42c1994e64a","builder":"builder-alpha","success":true,"time":"2026-07-10T10:21:02+08:00","message":"builder dispatch succeeded"}
{"level":"info","log_type":"builder_report","event":"builder_score_updated","builder":"builder-alpha","base_score":3,"effective_score":5.938629258090078,"attempts":18,"successes":18,"dispatch_failures":0,"sandwich_attacks":0,"well_behaved_events":5,"valuable_order_flow":11,"time":"2026-07-10T10:21:02+08:00","message":"builder score updated"}
{"level":"info","log_type":"builder_report","event":"builder_dispatch_selected","bundle_hash":"0x535f7f0ec4a4cb53dbe17b8e44b249421e9d7e0fca863295ebf10efce088e74b","builder":"builder-alpha","url":"http://localhost:18545","base_score":3,"effective_score":5.938629258090078,"total_score":6.155296055996548,"time":"2026-07-10T10:21:03+08:00","message":"builder dispatch selected"}

```

可以看见目前可能根据sandwich attack， well behave事件等，进行score的实时更新。

后续调研回进一步完善，主要是考虑exploration + exploitation的方法，用便宜的订单流去做探索,用贵的订单流去做保守的收益兑现,而不是让所有订单流都承担同样的风险敞口， 进行分层路由，避免只给一小撮builder。