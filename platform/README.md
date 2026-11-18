# Validator MEV Platform

一个用于Solana验证者MEV的综合平台，包含sandwich攻击检测和缓解策略研究。

## 项目结构

```
.
├── config/              # 统一配置管理
│   ├── __init__.py
│   └── settings.py
├── log/                 # 统一日志管理
│   ├── __init__.py
│   └── logger.py
├── type/                # 公共数据类型
│   ├── models.py
│   ├── staker.py
│   └── validator.py
├── utils/               # 公共工具函数
│   ├── obj.py
│   ├── sandwich.py
│   ├── stake_reader.py
│   └── utils.py
├── labels/              # 标签管理
│   └── labels.py
├── app/                 # 主应用
│   ├── backend/
│   ├── frontend/
│   ├── docx/
│   ├── tests/
│   └── scripts/
├── mitigation/          # Sandwich缓解实验
│   ├── strategy/        # 发送策略
│   ├── user/            # 用户客户端
│   ├── docx/
│   ├── tests/
│   └── scripts/
└── sandwich/            # Sandwich检测方法
    ├── docx/
    ├── tests/
    └── scripts/
```

## 架构设计

### 公共层

- **config**: 所有配置统一管理，从环境变量读取
- **log**: 统一的日志系统，支持多个日志记录器
- **type**: 公共数据类型定义（Transaction, SlotLeader等）
- **utils**: 公共工具函数
- **labels**: 标签管理

### 应用模块

1. **app**: 主应用（风险评估和可视化）
2. **mitigation**: Sandwich攻击缓解策略模拟平台
3. **sandwich**: Sandwich攻击检测方法（待实现）

## 快速开始

### 1. 安装uv

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

### 2. 同步依赖

```bash
uv sync
```

### 3. 配置环境变量

复制 `.env.example` 到 `.env` 并配置：

```env
SLOT_LEADER_PATH=/path/to/slot_leaders.csv
DEBUG_MODE=true
SERVER_HOST=0.0.0.0
SERVER_PORT=5000
```

### 4. 运行模块

**Mitigation模块**
```bash
bash mitigation/scripts/start.sh
# 或
uv run python -m mitigation.server
```

**App模块**
```bash
bash app/scripts/start.sh
```

## 开发

### 运行测试

```bash
uv run pytest
```

### 代码格式化

```bash
uv run black .
uv run ruff check .
```

## 模块说明

### Mitigation

交易发送策略模拟平台，用于研究不同发送时机对sandwich攻击的缓解效果。

详见: [mitigation/README.md](mitigation/README.md)

### App

主应用，包含验证者风险评估和可视化界面。

详见: [app/README.md](app/README.md)

### Sandwich

Sandwich攻击检测方法（待实现）

## 贡献

请阅读 CONTRIBUTING.md 了解贡献指南。

## 许可证

MIT License
