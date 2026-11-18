# Claude 项目规范

本文档定义了本项目的开发规范和工作流程。

## 项目结构规范

### 文件组织

```
项目根目录/
├── config/          # 统一配置（所有模块共享）
├── log/             # 统一日志
├── type/            # 公共数据类型
├── utils/           # 公共工具函数
├── labels/          # 标签管理
├── docx/            # 📄 项目文档统一存放处
├── scripts/         # 📜 项目级脚本统一存放处
├── tests/           # 🧪 项目级测试统一存放处
├── app/             # 应用模块
│   ├── docx/        # 模块文档
│   ├── scripts/     # 模块脚本
│   └── tests/       # 模块测试
├── mitigation/      # 缓解实验模块
│   ├── docx/
│   ├── scripts/
│   └── tests/
└── sandwich/        # 检测方法模块
    ├── docx/
    ├── scripts/
    └── tests/
```

### 规则

1. **文档位置**
   - 项目级文档 → `docx/`
   - 模块级文档 → `<module>/docx/`
   - 所有 `.md`, `.txt`, `.pdf` 等文档文件必须放在 docx/ 下

2. **脚本位置**
   - 项目级脚本 → `scripts/`
   - 模块级脚本 → `<module>/scripts/`
   - 所有 `.sh`, `.py` 等可执行脚本必须放在 scripts/ 下

3. **测试位置**
   - 项目级测试 → `tests/`
   - 模块级测试 → `<module>/tests/`
   - 所有测试文件必须放在 tests/ 下

## 代码修改规范

### ⚠️ 重要原则

1. **直接修改现有代码**
   - 修改功能时，直接在已有代码上进行修改
   - 不要创建功能类似的新模块或新文件
   - 不要保留旧代码作为备份（git会记录历史）

2. **避免重复**
   - 发现功能相似的代码，合并而不是复制
   - 使用已有的工具函数，不要重新实现
   - 公共功能提取到 utils/ 或 type/

3. **示例**
   ```
   ❌ 错误做法：
   - 创建 server_new.py 替代 server.py
   - 创建 strategy_v2.py 替代原有策略
   - 保留 old_utils.py 备份
   
   ✅ 正确做法：
   - 直接修改 server.py
   - 在原有策略文件中更新逻辑
   - 删除不需要的旧文件
   ```

## Git 使用规范

### ⚠️ 不要操作 Git

- **不要**执行 `git add`, `git commit`, `git push` 等命令
- **不要**创建分支、合并分支
- **不要**修改 git 配置
- 用户会自行管理 git 操作

### 可以查看状态

```bash
# ✅ 允许查看状态
git status
git diff
git log

# ❌ 不允许修改
git add
git commit
git push
git branch
git checkout
```

## 配置管理

### 统一配置源

- **所有配置从 `config/` 导入**
- 不要在模块中创建独立的配置文件
- 环境变量统一在 `.env` 中定义

```python
# ✅ 正确
from config import DEBUG_MODE, SERVER_PORT

# ❌ 错误
from .config import DEBUG_MODE  # 模块不应有自己的config
```

## 导入规范

### 公共层导入（顶层导入）

```python
from config import DEBUG_MODE, SERVER_PORT
from log import server_logger
from type.models import Transaction, SlotLeader
from utils import tosol, format_sol
```

### 模块内导入（相对导入）

```python
from .strategy import ImmediateStrategy
from .transaction_manager import TransactionManager
```

## 开发工作流

### 1. 理解现有代码
- 先阅读相关文件
- 理解现有架构和逻辑
- 确定修改点

### 2. 直接修改
- 在原文件上修改
- 不创建新的重复文件
- 保持代码风格一致

### 3. 放置文件
- 文档 → docx/
- 脚本 → scripts/
- 测试 → tests/

### 4. 验证
- 运行相关测试
- 确保导入正常
- 检查功能正确

## 测试规范

### 测试文件命名

```
tests/
├── test_imports.py      # 导入测试
├── test_<module>.py     # 模块功能测试
└── test_integration.py  # 集成测试
```

### 运行测试

```bash
# 运行单个测试
python tests/test_imports.py

# 运行所有测试
pytest

# 运行模块测试
python mitigation/tests/test_strategy.py
```

## 依赖管理

### 使用 uv

```bash
# 同步依赖
uv sync

# 添加依赖
uv add package-name

# 运行命令
uv run python script.py
```

### 不要混用包管理器

- ✅ 使用 uv
- ❌ 不要使用 pip install（会破坏uv管理）
- ❌ 不要创建 requirements.txt（使用 pyproject.toml）

## 文档规范

### 必需文档

每个模块应有：
- `README.md` - 模块说明、使用方法
- `docx/` - 详细文档、设计文档

项目级文档（在根目录 docx/）：
- `STRUCTURE.md` - 项目结构说明
- `REFACTOR_SUMMARY.md` - 重构总结
- 其他项目级文档

### 文档内容

- 清晰的模块说明
- 快速开始指南
- API 文档（如适用）
- 示例代码
- 常见问题

## 注意事项

### ✅ 应该做

- 先查看现有代码再动手
- 修改前理解代码逻辑
- 保持代码风格一致
- 添加必要的注释和文档
- 运行测试验证修改

### ❌ 不应该做

- 创建功能重复的新文件
- 不理解代码就修改
- 操作 git
- 混用包管理器
- 在错误的目录放置文件

## 快速参考

| 文件类型 | 项目级位置 | 模块级位置 |
|---------|-----------|-----------|
| 文档 | `docx/` | `<module>/docx/` |
| 脚本 | `scripts/` | `<module>/scripts/` |
| 测试 | `tests/` | `<module>/tests/` |
| 配置 | `config/` | ❌ 不要创建 |
| 日志 | `log/` | ❌ 不要创建 |

---

遵循这些规范可以保持项目整洁、一致、易维护。
