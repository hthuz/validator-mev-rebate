#!/bin/bash
# Mitigation Server启动脚本

# 获取脚本所在目录的父目录（mitigation目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MITIGATION_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$MITIGATION_DIR")"

echo "项目根目录: $PROJECT_ROOT"
echo "Mitigation目录: $MITIGATION_DIR"

# 切换到项目根目录
cd "$PROJECT_ROOT"

# 设置Python路径
export PYTHONPATH="$PROJECT_ROOT:$PYTHONPATH"

# 检查并安装uv
if ! command -v uv &> /dev/null; then
    echo "uv未安装，请先安装uv: curl -LsSf https://astral.sh/uv/install.sh | sh"
    exit 1
fi

# 同步依赖
echo "同步依赖..."
uv sync

# 启动服务器
echo "启动Mitigation服务器..."
uv run python -m mitigation.server
