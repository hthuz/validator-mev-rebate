# MEV mitigation prototype  测试报告

github link: https://github.com/hthuz/validator-mev-rebate

目前主要是基于mev rebate,通过将更多订单流分配给表现更好的builder的思路，从而激励更少的mev作恶行为。目前rebate与交易dispatch分发逻辑跑通，后续是更复杂的一些分发策略，builder权重设置策略。

## rebate server端日志

```bash
2026-05-08T14:31:20+08:00 INF Bundle added to queue bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059 priority=false queueSize=1 targetBlock=1000000
2026-05-08T14:31:20+08:00 INF Bundle accepted bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059 hasBackrun=false
2026-05-08T14:31:20+08:00 INF Processing bundle bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059 retry=0 targetBlock=1000000
2026-05-08T14:31:20+08:00 DBG Bundle simulated bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059 gasUsed=21000 stateBlock=1000008
2026-05-08T14:31:20+08:00 INF Hint broadcasted via SSE matchingHash=0x0420f9731c3b09f7d9f8e240da67e72953f0905cdede8e0c7ba1449b9505cbc6 subscribers=1
2026-05-08T14:31:20+08:00 INF Dispatching bundle to builder builder=builder-alpha bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059 score=3 totalScore=4 url=http://localhost:18545
2026-05-08T14:31:20+08:00 INF MockBuilder received eth_sendMevBundle id=1 params=[{"body":[{"tx":"0xf86f80843b9aca00825208947a250d5630b4cf539739df2c5dacb4c659f2488d880de0b6b3a764000084a9059cbb26a01aad4c838c758d90b67d824dd015446a4487777e8041a185904690c7e60eced6a020dbe8c5444a8c74139f1e74b695a67c9c56dd9357fcf36bf7b354f72889ff3f"}],"inclusion":{"block":"0xf4240","maxBlock":"0xf424a"},"metadata":{"bodyHashes":["0x9d9b4539155e7a632ad32566dfc7d8d67490f55b536ebca653e1158b7d8de0c4"],"bundleHash":"0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059","matchingHash":"0x0420f9731c3b09f7d9f8e240da67e72953f0905cdede8e0c7ba1449b9505cbc6","receivedAt":"0x651488b849602","signer":"0x090490da4ef03408f2b38da1b0aa7cdb27f6141b"},"privacy":{"hints":84},"validity":{},"version":"v0.1"}]
2026-05-08T14:31:20+08:00 INF Bundle dispatched successfully builder=builder-alpha bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059
2026-05-08T14:31:20+08:00 INF Bundle processed successfully bundleHash=0x1fa736370f95f085eb8f7b009f2509d047026e58694b520f498aa59d84caa059 gasUsed=21000 profit=2100000
2026-05-08T14:31:22+08:00 INF New block block=1000009
^C2026-05-08T14:31:22+08:00 INF Shutting down...
2026-05-08T14:31:22+08:00 INF === Builder Dispatch Summary ===
2026-05-08T14:31:22+08:00 INF Builder dispatch stats builder=builder-alpha failed=0 success=35 total=35
2026-05-08T14:31:22+08:00 INF Builder dispatch stats builder=builder-beta failed=0 success=12 total=12
2026-05-08T14:31:22+08:00 INF ================================
```

可以看出其能正常发送交易给builder,将user 交易SSE推流给searcher,并且根据builder的score，将交易dispatch给不同的builder

## user端日志

```bash
2026-05-08T14:29:48+08:00 INF sending req
2026-05-08T14:29:48+08:00 INF received resp RPC response={"bundleHash":"0xb7c6cbc7a03192691ec923e135a6ec7cd3a4ea451799499cad1cc17c8ed8984c"}
2026-05-08T14:29:50+08:00 INF sending req
2026-05-08T14:29:50+08:00 INF received resp RPC response={"bundleHash":"0x65fa4ecdad17d10ed667d2dfaf257756586a2ba07081216f385051dc678a0e38"}
2026-05-08T14:29:52+08:00 INF sending req
```

能验证其发送的交易请求被正确接收，不过对于确认是否上链仍需要user自己确认，rebate不提供这个机制保证 => 后续可以尝试解决这个问题



## searcher端日志

```bash
2026-05-08T14:30:04+08:00 INF received hint hash=0x7ca43ac190c5899d3ca5a7f20eb45bdf92ca575c0077450e7b600a7b1c9c28dc logs=1 txs=1
2026-05-08T14:30:04+08:00 INF tx hint index=0 txHash=0x1ab83e5c543182a28ec542890b954dd7331fe7e20f21b7b858b51b1a25f55af2
2026-05-08T14:30:04+08:00 INF log hint address=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D index=0 topics=3
2026-05-08T14:30:06+08:00 INF received hint hash=0x43b6ffd834357ab1eddf06ede1f21a7a07865e3bfd4bbe32d04772a574350082 logs=1 txs=1
2026-05-08T14:30:06+08:00 INF tx hint index=0 txHash=0xe593b1d9bd71559a2dbf0a51f400ef56bf06b52d15f10177bbdf0b555e2acd0f
2026-05-08T14:30:06+08:00 INF log hint address=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D index=0 topics=3
2026-05-08T14:30:08+08:00 INF received hint hash=0x31536b30145ca3762cd483839c8787ec289c192b2e64afa50706bbd11373ba59 logs=1 txs=1
2026-05-08T14:30:08+08:00 INF tx hint index=0 txHash=0x17e52630aa45d3ae4ab999b33e87a4701b5473cf4cfddca6e01ac72809a56f78
2026-05-08T14:30:08+08:00 INF log hint address=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D index=0 topics=3
```

能看出searcher能订阅到隐私暴露允许的交易，支持其在这基础上发送bundle. 不过目前和已有的mev-share有点类似，在这个searcher订阅和发送交易上，可以再尝试增加一些功能与创新。





