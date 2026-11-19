from abc import ABC, abstractmethod
from typing import List
from type.models import BlockTransaction, TransactionRecord
from log import strategy_logger as logger


class SendStrategy(ABC):
    """交易发送策略的基类"""

    @abstractmethod
    def should_send(self, transaction: BlockTransaction, current_slot: int) -> bool:
        """判断是否应该发送交易"""
        pass

    @abstractmethod
    def get_strategy_name(self) -> str:
        """获取策略名称"""
        pass

    def send_to_validator(self, transaction: BlockTransaction, validator: str) -> TransactionRecord:
        """发送交易到验证者（模拟）"""
        # logger.info(f"[{self.get_strategy_name()}] 发送交易 {transaction.signature} 到 validator {validator}")

        # 这里只是模拟发送，实际环境中会调用真实的发送逻辑
        record = TransactionRecord(
            signature=transaction.signature,
            slot=transaction.slot,
            validator=validator,
            strategy=self.get_strategy_name()
        )

        return record
