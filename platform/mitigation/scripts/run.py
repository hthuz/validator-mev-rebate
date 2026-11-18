#!/usr/bin/env python3
"""Mitigation服务器启动脚本（Python版本）"""

import sys
from pathlib import Path

# 添加项目根目录到Python路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.server import create_server
from mitigation.strategy import ImmediateStrategy, DelayedStrategy

def main():
    """启动服务器"""
    import argparse

    parser = argparse.ArgumentParser(description='启动Mitigation交易服务器')
    parser.add_argument(
        '--strategy',
        choices=['immediate', 'delayed'],
        default='immediate',
        help='选择发送策略 (默认: immediate)'
    )
    parser.add_argument(
        '--delay',
        type=int,
        default=1,
        help='DelayedStrategy的延迟slot数 (默认: 1)'
    )

    args = parser.parse_args()

    # 根据参数选择策略
    if args.strategy == 'immediate':
        strategy = ImmediateStrategy()
    else:
        strategy = DelayedStrategy(delay_slots=args.delay)

    # 创建并启动服务器
    server = create_server(strategy=strategy)
    server.start()

if __name__ == "__main__":
    main()
