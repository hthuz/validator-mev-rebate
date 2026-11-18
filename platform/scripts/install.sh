#!/bin/bash
# 快速安装和启动脚本

echo "=========================================="
echo "交易接收平台 - 安装和启动"
echo "=========================================="

# 检查Python
if ! command -v python3 &> /dev/null; then
    echo "❌ Python3 未安装"
    exit 1
fi

echo "✓ Python3 已安装"

# 安装依赖
echo ""
echo "正在安装依赖..."
pip install -q flask requests pandas python-dotenv

if [ $? -eq 0 ]; then
    echo "✓ 依赖安装成功"
else
    echo "❌ 依赖安装失败"
    exit 1
fi

# 检查配置
echo ""
echo "检查配置文件..."
if [ -f "../.env" ]; then
    echo "✓ .env 文件存在"
else
    echo "⚠️  .env 文件不存在，将使用默认配置"
fi

echo ""
echo "=========================================="
echo "准备就绪！"
echo "=========================================="
echo ""
echo "启动选项："
echo "1. 一键启动（服务器+客户端）："
echo "   python transaction_platform/run_all.py"
echo ""
echo "2. 分别启动："
echo "   服务器: python transaction_platform/run_server.py"
echo "   客户端: python transaction_platform/run_user.py"
echo ""
echo "3. 快速测试："
echo "   python transaction_platform/test_quick.py"
echo ""
