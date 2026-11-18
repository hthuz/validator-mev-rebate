# Platform 项目结构

本文档描述了整个平台的文件组织结构。

## 根目录结构

```
platform/
├── config/              # 统一配置管理
├── log/                 # 统一日志管理
├── type/                # 公共数据类型
├── utils/               # 公共工具函数
├── labels/              # 标签管理
├── app/                 # 主应用模块
├── mitigation/          # 缓解策略模块
├── sandwich/            # 检测方法模块
├── scripts/             # 全局脚本
├── tests/               # 全局测试
├── pyproject.toml       # 项目配置(uv)
├── .env                 # 环境变量
└── README.md            # 项目说明
```

## 公共层

### config/
- `settings.py`: 所有配置项的定义，从.env读取
- `__init__.py`: 导出所有配置

### log/
- `logger.py`: 日志系统实现
- `__init__.py`: 导出所有logger实例

### type/
- `models.py`: 核心数据模型 (Transaction, SlotLeader, TransactionRecord)
- `validator.py`: 验证者相关类型
- `staker.py`: 质押者相关类型
- `__init__.py`: 统一导出

### utils/
- `utils.py`: 通用工具函数
- `sandwich.py`: Sandwich相关工具
- `obj.py`: 对象处理工具
- `stake_reader.py`: 质押数据读取
- `__init__.py`: 统一导出

### labels/
- `labels.py`: 标签管理和处理

## 应用模块

### app/
主应用，包含前后端。

```
app/
├── backend/           # 后端服务
├── frontend/          # 前端界面
├── docx/             # 文档
├── tests/            # 测试
├── scripts/          # 脚本
└── README.md         # 模块说明
```

### mitigation/
交易发送策略模拟平台。

```
mitigation/
├── strategy/          # 策略实现
│   ├── base_strategy.py
│   ├── immediate_strategy.py
│   └── delayed_strategy.py
├── user/             # 用户客户端
├── server.py         # Flask服务器
├── slot_manager.py   # Slot管理
├── transaction_manager.py  # 交易管理
├── docx/             # 文档
├── tests/            # 测试
├── scripts/          # 脚本
└── README.md         # 模块说明
```

### sandwich/
Sandwich攻击检测方法（待实现）。

```
sandwich/
├── docx/             # 文档
├── tests/            # 测试
├── scripts/          # 脚本
└── README.md         # 模块说明
```

## 设计原则

1. **统一配置**: 所有配置从 `config/` 导入，不在各模块中重复定义
2. **统一日志**: 所有日志从 `log/` 导入，保持一致的日志格式
3. **公共类型**: 共享的数据类型定义在 `type/` 中
4. **模块独立**: 每个应用模块（app, mitigation, sandwich）相对独立
5. **标准结构**: 每个应用都有 docx, tests, scripts 子目录

## 导入规范

```python
# 从公共层导入
from config import DEBUG_MODE, SERVER_PORT
from log import server_logger
from type.models import Transaction
from utils import tosol, format_sol

# 从应用模块导入
from mitigation.strategy import ImmediateStrategy
```

## 依赖管理

项目使用 `uv` 进行依赖管理：

```bash
# 同步依赖
uv sync

# 添加新依赖
uv add package-name

# 运行命令
uv run python -m mitigation.server
```
