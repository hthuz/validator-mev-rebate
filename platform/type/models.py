from dataclasses import dataclass, field
from typing import Dict, Any, Optional
from datetime import datetime


@dataclass
class Transaction:
    """表示一个用户交易"""
    tx_id: str
    slot: int
    signature: str
    data: Dict[str, Any]
    received_at: float = field(default_factory=lambda: datetime.now().timestamp())

    def __hash__(self):
        return hash(self.tx_id)

    def __eq__(self, other):
        if isinstance(other, Transaction):
            return self.tx_id == other.tx_id
        return False


@dataclass
class SlotLeader:
    """Slot领导者信息"""
    slot: int
    leader: str


@dataclass
class TransactionRecord:
    """交易发送记录"""
    tx_id: str
    slot: int
    validator: str
    strategy: str
    sent_at: float = field(default_factory=lambda: datetime.now().timestamp())
