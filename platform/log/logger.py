import logging
import sys
from typing import Optional


def setup_logger(name: str, level: int = logging.INFO) -> logging.Logger:
    """设置日志记录器"""
    logger = logging.getLogger(name)
    logger.setLevel(level)

    # 避免重复添加handler
    if logger.handlers:
        return logger

    # 创建console handler
    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(level)

    # 创建formatter
    formatter = logging.Formatter(
        fmt='%(asctime)s [%(name)s] %(levelname)s: %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S'
    )
    handler.setFormatter(formatter)

    logger.addHandler(handler)
    return logger


# 全局logger实例
server_logger = setup_logger('Server')
slot_manager_logger = setup_logger('SlotManager')
transaction_manager_logger = setup_logger('TransactionManager')
strategy_logger = setup_logger('Strategy')
user_logger = setup_logger('User')
