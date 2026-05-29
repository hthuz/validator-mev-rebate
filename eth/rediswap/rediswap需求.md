# RediSwap Demo 需求文档（MVP）

## 1. 项目目标

实现一个简化版 RediSwap 模拟器，用于演示其核心机制：

1. User 提交 swap 请求（带 limit）
2. Arbitrager 提交对外部市场价格的 belief
3. 系统根据 belief 对每个 user tx 计算潜在 MEV
4. 系统为每个 tx 进行 second-price auction
5. Winner 获得该 tx 的 MEV 权限
6. 系统生成 bundle（front → user → back）
7. Winner 支付 second price
8. Payment refund 给 user 或 LP
9. 输出最终 bundle 和 refund 结果

目标：

**完整跑通论文 Example1 的结果**

不要求：

* DSIC proof
* Sybil-proof
* TEE
* 链上执行
* 多区块
* 隐私
* Permit2
* 真正 mempool

---

# 2. 系统参与者

系统包含三类参与者：

---

## 2.1 User

表示普通交易用户。

输入：

```text
direction:
    X→Y
    或
    Y→X

max_input:
    最大可支付

min_output:
    最少接受
```

例如：

```text
TX1:

X→Y

input=8

output>=25
```

表示：

```text
用户最多支付 8X
至少收到 25Y
```

---

### User 行为

User：

```text
创建 swap tx
↓

提交到 pending queue
↓

等待 bundle generation
↓

收到 execution + refund
```

---

## 2.2 Arbitrager

表示套利者。

唯一输入：

```text
belief_price=q
```

即：

套利者认为：

```text
1 X

=

q Y
```

例如：

```text
arb1:

belief=4
```

意味着：

```text
1X≈4Y
```

---

Arbitrager 提供：

```text
price belief
↓

参与所有 tx auction
↓

若胜出：

获得 MEV execution 权
支付 auction payment
```

---

## 2.3 LP

LP：

仅接受：

```text
refund
```

不主动参与。

---

# 3. Pool 模型

采用：

```text
Constant Product:

x*y=k
```

例如：

初始：

```text
x=4

y=100

k=400
```

对应论文 Example。

---

Pool 需要支持：

状态：

```text
reserve_x

reserve_y

spot_price
```

以及：

```text
execute swap

compute price
```

---

# 4. 核心流程

系统按 block 模拟：

输入：

```text
initial pool

pending tx

arbitrager beliefs
```

输出：

```text
bundle

payments

refunds
```

---

执行：

---

## Step1：

收集：

```text
pending tx

=

[
TX1
TX2
TX3
...
]
```

以及：

```text
beliefs

=

[
q1
q2
...
]
```

---

## Step2：

对每个 tx：

计算：

对于：

```text
X→Y:
```

计算：

[
\Phi=
\delta_X q
----------

\delta_Y
]

对于：

```text
Y→X:
```

计算：

[
\Phi
====

*

\delta_X q
+
\delta_Y
]

然后：

[
V=max(0,\Phi)
]

作为：

```text
bid
```



---

得到：

```text
TX1:

arb1:
7

arb2:
0
```

等等。

---

## Step3：

Auction

对每个 tx：

赢家：

```text
winner

=

highest bid
```

支付：

```text
payment

=

second highest bid
```

即：

second-price auction



---

输出：

例如：

```text
TX2:

winner=arb1

payment=18
```

---

## Step4：

Bundle Generation

对于：

每个获胜 tx：

生成：

```text
front-run

↓

user tx

↓

back-run
```

形成：

```text
initial state

↓

front

↓

user

↓

back

↓

return initial
```

论文核心思想：

所有 sandwich：

```text
start from initial

end at initial
```

避免：

```text
tx1
影响
tx2
```

即：

independent sub-bundles



---

Demo 不要求真实 AMM path。

仅记录：

```text
bundle:

[
front
user
back
]
```

即可。

---

## Step5：

Rebalancing Auction

除了 user tx：

还需对：

```text
initial pool state
```

做一次 auction。

对应：

LP LVR。

赢家：

获得：

```text
rebalancing arbitrage
```

支付：

```text
second price
```

refund：

```text
LP
```



---

## Step6：

Refund

规则：

对于：

```text
user tx auction
```

payment：

```text
→ user
```

对于：

```text
rebalancing auction
```

payment：

```text
→ LP
```

---

输出：

例如：

```text
TX2 refund:

18

↓

user(TX2)
```

以及：

```text
36

↓

LP
```

---

# 5. 输入输出规范

建议：

---

输入：

```json
{
  "pool": {
      "x":4,
      "y":100
  },

  "transactions":[
      {
          "direction":"X->Y",
          "input":8,
          "output":25
      }
  ],

  "arbitragers":[
      {
          "id":"arb1",
          "belief":4
      }
  ]
}
```

---

输出：

```json
{
    "bundle":[...],

    "auction":[
        {

        "tx":"TX1",

        "winner":"arb1",

        "payment":0
        }
    ],

    "refund":[
        {

        "receiver":"user",

        "amount":18
        }
    ]
}
```

---

# 6. Example1 验收标准（必须通过）

输入：

论文：

```text
Pool:

(4,100)

TX1:

(8,25)

TX2:

(30,12)

TX3:

(20,10)

arb1=4

arb2=1
```



---

期望输出：

```text
TX1:

winner=arb1

payment=0


TX2:

winner=arb1

payment=18

refund→TX2 user


TX3:

winner=arb2

payment=0


Rebalancing:

winner=arb2

payment=36

refund→LP
```



---

# 7. 非目标（明确不要做）

以下不属于 Demo：

```text
Sybil attack

TEE

privacy

MEV-Boost

PBS

builder

proposer

multi block

dynamic fee

multi pool

zk

proof
```

全部跳过。

---

# 8. 最终交付要求

Code agent 最终需提供：

```text
1.
可配置输入：

pool
tx
belief

2.
自动运行：

auction
bundle
refund

3.
输出：

winner
payment
refund
bundle

4.
Example1 能完全复现论文结果
```

这就是一个**最小可运行 RediSwap 模拟器**，复杂度大概在几百行以内，但已经保留论文 80% 的核心思想：**MEV internalization + auction + redistribution**。
