#


目前已实现实际交易的rebate功能，包括交易的接收、处理、分发和奖励。 
不过功能上还是简单的prototype实现，后续进一步完善和丰富功能。

可以实现的方面
- prediction： 现在只是根据当前交易，searcher来发送mev交易，后续可以根据历史交易来预测未来的交易，从而优化mev交易的发送。
- user & searcher 合作: 比如user可以指定mev交易的发送时间，searcher可以根据user的指定时间来发送mev交易， 从而促进user和searcher之间的合作，实现更高效的mev交易发送。
- searcher间合作：比如searcher A提供基本fund， searcher B提供交易策略，让擅长不同类型交易的searcher合作，实现更高效的mev交易发送。

## server 端日志

```
2026-06-26T12:06:35+08:00 INF Bundle dispatched successfully builder=builder-alpha bundleHash=0x83a43bbaa80131011b47564d4b74c55ee028e7922574d6425a99bcaec7c38727
2026-06-26T12:06:35+08:00 INF Bundle processed successfully bundleHash=0x83a43bbaa80131011b47564d4b74c55ee028e7922574d6425a99bcaec7c38727 gasUsed=45692 profit=12766319715240
2026-06-26T12:06:36+08:00 INF New block block=25398878
2026-06-26T12:06:38+08:00 INF New block block=25398879
2026-06-26T12:06:40+08:00 INF New block block=25398880
2026-06-26T12:06:40+08:00 INF Received SendBundle request block=25398880 bodyLen=1 maxBlock=25398881 version=v0.1
2026-06-26T12:06:40+08:00 INF Bundle added to queue bundleHash=0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859 priority=false queueSize=1 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF Bundle accepted bundleHash=0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859 hasBackrun=false
2026-06-26T12:06:40+08:00 INF Processing bundle bundleHash=0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859 retry=0 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF Replay bundle simulated bundleTxs=1 displacedTxs=1 mevGasPrice=0 stateBlock=25398880 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF Hint broadcasted via SSE matchingHash=0x2c3ca405f12c9816fa3a3013a7df34cdf78dea516ae8a0f07fbba289e37aac51 subscribers=1
2026-06-26T12:06:40+08:00 INF Dispatching bundle to builder builder=builder-alpha bundleHash=0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859 score=3 totalScore=4 url=http://localhost:18545
2026-06-26T12:06:40+08:00 INF MockBuilder received eth_sendMevBundle id=1 params=[{"body":[{"tx":"0x02f870018302544480840adfcca282565f94388c818ca8b9251b393131c08a736a67ccb192978730c346d343c34380c001a0ded131a60e5937bcdb3fe2c37dd88fb27adf80214d2f3c65bf2a9e7fc997edbfa016344874bbebc59e1ef527b02e4412bdf99e4b744524a364c5433197039114fe"}],"inclusion":{"block":"0x1838e60","maxBlock":"0x1838e61"},"metadata":{"bodyHashes":["0x68c3a862a75a5c86d818091249edb9dc7adc406a2598d00fa13df436c0a8bb10"],"bundleHash":"0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859","matchingHash":"0x2c3ca405f12c9816fa3a3013a7df34cdf78dea516ae8a0f07fbba289e37aac51","receivedAt":"0x655203c5c9e2d","signer":"0x39ead65c9a14604b3aa19945cf53c0d3fcdd4a7f"},"privacy":{"hints":87},"validity":{},"version":"v0.1"}]
2026-06-26T12:06:40+08:00 INF Bundle dispatched successfully builder=builder-alpha bundleHash=0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859
2026-06-26T12:06:40+08:00 INF Bundle processed successfully bundleHash=0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859 gasUsed=21000 profit=0
2026-06-26T12:06:40+08:00 INF Received SendBundle request block=25398880 bodyLen=2 maxBlock=25398881 version=v0.1
2026-06-26T12:06:40+08:00 INF Backrun matched bundleHash=0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43 matchingHash=0x2c3ca405f12c9816fa3a3013a7df34cdf78dea516ae8a0f07fbba289e37aac51
2026-06-26T12:06:40+08:00 INF Bundle added to queue bundleHash=0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43 priority=false queueSize=1 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF Bundle accepted bundleHash=0xb37f185ad16c4fd09bcad55540a06ccad0dad6c0b970c74686c0b0dae415510b hasBackrun=true
2026-06-26T12:06:40+08:00 INF Processing bundle bundleHash=0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43 retry=0 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF Replay bundle simulated bundleTxs=2 displacedTxs=2 mevGasPrice=108953094 stateBlock=25398880 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF Hint broadcasted via SSE matchingHash=0xb4d1653b92645dffe51ac6cd024a742b3f58c481423ae10d61da7e2e7af0ddff subscribers=1
2026-06-26T12:06:40+08:00 INF Dispatching bundle to builder builder=builder-alpha bundleHash=0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43 score=3 totalScore=4 url=http://localhost:18545
2026-06-26T12:06:40+08:00 INF MockBuilder received eth_sendMevBundle id=1 params=[{"body":[{"bundle":{"body":[{"tx":"0x02f870018302544480840adfcca282565f94388c818ca8b9251b393131c08a736a67ccb192978730c346d343c34380c001a0ded131a60e5937bcdb3fe2c37dd88fb27adf80214d2f3c65bf2a9e7fc997edbfa016344874bbebc59e1ef527b02e4412bdf99e4b744524a364c5433197039114fe"}],"inclusion":{"block":"0x1838e60","maxBlock":"0x1838e61"},"metadata":{"bodyHashes":["0x68c3a862a75a5c86d818091249edb9dc7adc406a2598d00fa13df436c0a8bb10"],"bundleHash":"0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859","matchingHash":"0x2c3ca405f12c9816fa3a3013a7df34cdf78dea516ae8a0f07fbba289e37aac51","receivedAt":"0x655203c5c9e2d","signer":"0x39ead65c9a14604b3aa19945cf53c0d3fcdd4a7f"},"privacy":{"hints":87},"validity":{},"version":"v0.1"}},{"tx":"0x02f894018303412f840cd56a888418cb986d8262709425e5fd18e791077c5c656d060d6314ab56cb9c33870619e4fad2abdda0c166535bc2fca7ddaa633fac2304ec0af8254802d91710699367dd6432778353c001a020459fa0a674f5e6f940c692e40c466e29839342242d15dcc17978e302a85af4a04e7251a810af4b4036b8193dbbe574eb6aec6a426af2cdc758e9901f322beba9"}],"inclusion":{"block":"0x1838e60","maxBlock":"0x1838e61"},"metadata":{"bodyHashes":["0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859","0xa3d5304124e89bc7a812eb947e21c8c1f779482bb0c6d3b1f331ccde973fda3c"],"bundleHash":"0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43","matchingHash":"0xb4d1653b92645dffe51ac6cd024a742b3f58c481423ae10d61da7e2e7af0ddff","prematched":true,"receivedAt":"0x655203c5cafec","signer":"0x39ead65c9a14604b3aa19945cf53c0d3fcdd4a7f"},"privacy":{"hints":84},"validity":{},"version":"v0.1"}]
2026-06-26T12:06:40+08:00 INF Bundle dispatched successfully builder=builder-alpha bundleHash=0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43
2026-06-26T12:06:40+08:00 INF Bundle processed successfully bundleHash=0x1de269411ec4803debe3c9ed7eaafcc643bf8c6f67ba5d2a7d53d7b0b3f2ef43 gasUsed=42512 profit=4631813944384
2026-06-26T12:06:42+08:00 INF New block block=25398881
2026-06-26T12:06:44+08:00 INF New block block=25398882
2026-06-26T12:06:45+08:00 INF Received SendBundle request block=25398882 bodyLen=1 maxBlock=25398883 version=v0.1
2026-06-26T12:06:45+08:00 INF Bundle added to queue bundleHash=0x76ab54b91cec7645f3fb3007e18044c951f7f2daacd1dc191616c58a08c7963f priority=false queueSize=1 targetBlock=25398882
2026-06-26T12:06:45+08:00 INF Bundle accepted bundleHash=0x76ab54b91cec7645f3fb3007e18044c951f7f2daacd1dc191616c58a08c7963f hasBackrun=false
2026-06-26T12:06:45+08:00 INF Processing bundle bundleHash=0x76ab54b91cec7645f3fb3007e18044c951f7f2daacd1dc191616c58a08c7963f retry=0 targetBlock=25398882
```

## searcher 端日志


```
2026-06-26T12:06:34+08:00 INF connected to SSE stream url=http://localhost:8080/events
2026-06-26T12:06:34+08:00 INF SSE connected confirmed
2026-06-26T12:06:35+08:00 INF received hint hash=0x08e34fa5be319586ed29269497c6006e77496ee6d9af5bafe1c9cb48b65d836d logs=1 txs=1
2026-06-26T12:06:35+08:00 INF submitted backrun bundle matchingHash=0x08e34fa5be319586ed29269497c6006e77496ee6d9af5bafe1c9cb48b65d836d targetBlock=25398877
2026-06-26T12:06:35+08:00 INF tx hint index=0 txHash=0x8b28d95ae5ba747461cabca7919bf610a1a792a3f9f28e25699a9e69f254836f
2026-06-26T12:06:35+08:00 INF log hint address=0xdAC17F958D2ee523a2206206994597C13D831ec7 index=0 topics=0
2026-06-26T12:06:35+08:00 INF received hint hash=0x727582b94f4aa3358e0c1d03410e1d99aa50cbff71e9306381c40767bea77ebb logs=2 txs=2
2026-06-26T12:06:35+08:00 INF skipping backrun because chain depth limit reached matchingHash=0x727582b94f4aa3358e0c1d03410e1d99aa50cbff71e9306381c40767bea77ebb maxChainDepth=2 txDepth=2
2026-06-26T12:06:35+08:00 INF tx hint index=0 txHash=0x8b28d95ae5ba747461cabca7919bf610a1a792a3f9f28e25699a9e69f254836f
2026-06-26T12:06:35+08:00 INF tx hint index=1 txHash=0x6010950bf9c456978854727ec4a0f27c6fdd83d39b62c9ec6a643680742bed7d
2026-06-26T12:06:35+08:00 INF log hint address=0xdAC17F958D2ee523a2206206994597C13D831ec7 index=0 topics=0
2026-06-26T12:06:35+08:00 INF log hint address=0x4313C378Cc91eA583C91387B9216e2c03096b27f index=1 topics=0
2026-06-26T12:06:40+08:00 INF received hint hash=0x2c3ca405f12c9816fa3a3013a7df34cdf78dea516ae8a0f07fbba289e37aac51 logs=1 txs=1
2026-06-26T12:06:40+08:00 INF submitted backrun bundle matchingHash=0x2c3ca405f12c9816fa3a3013a7df34cdf78dea516ae8a0f07fbba289e37aac51 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF tx hint index=0 txHash=0x68c3a862a75a5c86d818091249edb9dc7adc406a2598d00fa13df436c0a8bb10
2026-06-26T12:06:40+08:00 INF log hint address=0x388C818CA8B9251b393131C08a736A67ccB19297 index=0 topics=0
2026-06-26T12:06:40+08:00 INF received hint hash=0xb4d1653b92645dffe51ac6cd024a742b3f58c481423ae10d61da7e2e7af0ddff logs=2 txs=2
2026-06-26T12:06:40+08:00 INF skipping backrun because chain depth limit reached 
```

## user端日志

2026-06-26T12:06:35+08:00 INF received resp RPC response={"bundleHash":"0xc2cccb8397443345832e900c8c41879bad2942f08af66ea1cc8771eb0d8b6f76"}
2026-06-26T12:06:40+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398880 targetBlock=25398880
2026-06-26T12:06:40+08:00 INF received resp RPC response={"bundleHash":"0x39351e9debeed7ec205cbbf6ee42ee28747744726305693dec167136a6cd7859"}
2026-06-26T12:06:45+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398882 targetBlock=25398882
2026-06-26T12:06:45+08:00 INF received resp RPC response={"bundleHash":"0x76ab54b91cec7645f3fb3007e18044c951f7f2daacd1dc191616c58a08c7963f"}
2026-06-26T12:06:50+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398885 targetBlock=25398885
2026-06-26T12:06:50+08:00 INF received resp RPC response={"bundleHash":"0xef1e5ac84b29e985e92e73437fc205bfa2901232b5af89e455112273d060e6d0"}
2026-06-26T12:06:55+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398887 targetBlock=25398887
2026-06-26T12:06:55+08:00 INF received resp RPC response={"bundleHash":"0x7d9bef3789786e7d84e225775e8eff067e5756678117f07ee58eb7d9b1a7c403"}
2026-06-26T12:07:00+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398890 targetBlock=25398890
2026-06-26T12:07:00+08:00 INF received resp RPC response={"bundleHash":"0xbd3223f5e949614f93a5fe826e443e1c708cb0d53fb094a85c4d3788afa97546"}
2026-06-26T12:07:05+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398890 targetBlock=25398890
2026-06-26T12:07:05+08:00 INF received resp RPC response={"bundleHash":"0xf8530a78704ac31f7fa0800f693460eb743b0ad8070524bb2ef9b885bfd567e1"}
2026-06-26T12:07:10+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398890 targetBlock=25398890
2026-06-26T12:07:10+08:00 INF received resp RPC response={"bundleHash":"0x6a9a4c4da03fbee623e2b90646a94810fc93c6feae99a9b7f338fd5cb29fe681"}
2026-06-26T12:07:15+08:00 INF sending replay bundle bodyLen=1 currentBlock=25398890 targetBlock=25398890
2026-06-26T12:07:15+08:00 INF received resp RPC response={"bundleHash":"0x3911ec9de87b0d2f95f54783d8662b4fae324c44d23fd49a55cbe3dc1dbe7a65"}