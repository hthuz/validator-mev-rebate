from .base_strategy import SendStrategy
from type.models import Transaction


class DelayedStrategy(SendStrategy):
    """延迟发送策略：等待N个slot后再发送"""

    def __init__(self, delay_slots: int = 1):
        self.delay_slots = delay_slots

    def should_send(self, transaction: Transaction, current_slot: int) -> bool:
        """当当前slot >= 交易slot + 延迟slot数时，发送交易"""
        return current_slot >= (transaction.slot + self.delay_slots)

    def get_strategy_name(self) -> str:
        return f"DelayedStrategy(delay={self.delay_slots})"
