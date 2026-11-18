# Mitigation - 交易发送策略模拟平台

## 概述

Mitigation模块是一个模拟交易发送策略的平台，用于研究和测试不同的交易发送时机对sandwich攻击缓解的效果。

## 功能特性

- **交易接收与管理**: 通过HTTP API接收用户交易
- **Slot模拟**: 模拟Solana的slot推进机制
- **多策略支持**: 支持多种交易发送策略
  - ImmediateStrategy: 立即发送策略
  - DelayedStrategy: 延迟发送策略
- **Debug模式**: 支持手动控制slot推进，便于调试

## 架构

```
mitigation/
├── server.py              # Flask服务器主程序
├── slot_manager.py        # Slot管理器
├── transaction_manager.py # 交易管理器
├── config.py              # 配置文件
├── strategy/              # 策略模块
│   ├── base_strategy.py   # 策略基类
│   ├── immediate_strategy.py
│   └── delayed_strategy.py
└── user/                  # 用户客户端（待实现）
    └── client.py
```

## 快速开始

### 1. 安装依赖

项目使用uv管理依赖：

```bash
# 安装uv (如果还没安装)
curl -LsSf https://astral.sh/uv/install.sh | sh

# 同步依赖
uv sync
```

### 2. 配置环境变量

确保项目根目录的 `.env` 文件包含以下配置：

```env
SLOT_LEADER_PATH=/path/to/slot_leaders.csv
DEBUG_MODE=true
```

### 3. 启动服务器

**方式1: 使用启动脚本**
```bash
bash mitigation/scripts/start.sh
```

**方式2: 使用Python脚本**
```bash
uv run python mitigation/scripts/run.py --strategy immediate
# 或使用延迟策略
uv run python mitigation/scripts/run.py --strategy delayed --delay 2
```

**方式3: 直接运行**
```bash
uv run python -m mitigation.server
```

## API接口

### 1. 健康检查
```
GET /health
```

### 2. 发送交易
```
POST /sendTransaction
Content-Type: application/json

{
  "tx_id": "tx_123",
  "slot": 100,
  "signature": "sig_abc",
  "data": {}
}
```

### 3. 查看待处理交易
```
GET /transactions/pending
```

### 4. 查看已发送交易
```
GET /transactions/sent
```

### 5. 获取当前Slot
```
GET /slot/current
```

### 6. 手动推进Slot (仅Debug模式)
```
POST /slot/advance
```

## Debug模式

当 `DEBUG_MODE=true` 时：
- Slot不会自动推进
- 可以通过API手动推进: `POST /slot/advance`
- 可以在终端按Enter键推进

## 开发计划

- [ ] 完善用户客户端模拟
- [ ] 添加更多发送策略
- [ ] 实现交易效果分析
- [ ] 添加可视化界面
- [ ] 完善测试用例

## 测试

```bash
# 运行测试（待实现）
python -m pytest mitigation/tests/
```
