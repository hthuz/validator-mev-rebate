#!/usr/bin/env python3
"""Mitigation服务器启动脚本：所有参数从 config 读取，不使用 CLI 参数。"""

import sys
from pathlib import Path

# 添加项目根目录到Python路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.server import create_server
from mitigation.strategy import build_strategy_from_config


def main():
    """启动服务器"""
    server = create_server(strategy=build_strategy_from_config())
    server.start()


if __name__ == "__main__":
    main()
