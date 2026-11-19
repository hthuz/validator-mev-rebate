from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Any, Callable, Optional

from type.models import BlockTransaction, TransactionRecord
from log import strategy_logger as logger


@dataclass
class StrategyContext:
    """策略运行所需的外部依赖，由 server 注入。"""

    get_leader: Callable[[int], Optional[str]]
    risk_store: Any = None


class SendStrategy(ABC):
    """交易发送策略的基类"""

    # 类属性默认值，避免已有策略必须实现 __init__
    _context: Optional[StrategyContext] = None

    def set_context(self, context: StrategyContext) -> None:
        self._context = context

    @property
    def context(self) -> Optional[StrategyContext]:
        return self._context

    @abstractmethod
    def should_send(self, transaction: BlockTransaction, current_slot: int) -> bool:
        """判断是否应该发送交易"""
        pass

    @abstractmethod
    def get_strategy_name(self) -> str:
        """获取策略名称"""
        pass

    def send_to_validator(
        self,
        transaction: BlockTransaction,
        validator: str,
        current_slot: Optional[int] = None,
    ) -> TransactionRecord:
        """发送交易到验证者（模拟），并记录实际落地的 slot 与延迟。"""
        target_slot = transaction.slot if current_slot is None else current_slot
        delay_slots = max(0, target_slot - transaction.slot)

        risk_store = self._context.risk_store if self._context else None
        leader_risky = bool(risk_store and risk_store.is_risky(validator))

        return TransactionRecord(
            signature=transaction.signature,
            slot=transaction.slot,
            validator=validator,
            strategy=self.get_strategy_name(),
            target_slot=target_slot,
            delay_slots=delay_slots,
            leader_risky=leader_risky,
        )
