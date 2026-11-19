from .base_strategy import SendStrategy
from type.models import BlockTransaction


class ImmediateStrategy(SendStrategy):
    """立即发送策略：当服务器slot达到交易slot时立即发送"""

    def should_send(self, transaction: BlockTransaction, current_slot: int) -> bool:
        """当当前slot >= 交易的slot时，发送交易"""
        return current_slot >= transaction.slot

    def get_strategy_name(self) -> str:
        return "ImmediateStrategy"
