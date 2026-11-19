from typing import Dict, List, Set
from collections import defaultdict
import threading
from type.models import BlockTransaction, TransactionRecord
from log import transaction_manager_logger as logger


class TransactionManager:
    """管理接收到的交易，以及每个 slot 的结束标识"""

    def __init__(self):
        # key 是 slot 号
        self._transactions: Dict[int, List[BlockTransaction]] = defaultdict(list)
        self._ended_slots: Set[int] = set()
        self._lock = threading.Lock()
        self._sent_records: List[TransactionRecord] = []

    def add_transaction(self, transaction: BlockTransaction) -> None:
        """添加一个交易到队列"""
        with self._lock:
            self._transactions[transaction.slot].append(transaction)
            # logger.info(f"接收交易: {transaction.signature}, slot: {transaction.slot}")

    def mark_slot_end(self, slot: int) -> None:
        """标记某个 slot 的用户交易已全部发送完毕（收到 end 标识）"""
        with self._lock:
            self._ended_slots.add(slot)
            logger.info(f"收到 end 标识，slot {slot} 交易发送结束")

    def is_slot_ended(self, slot: int) -> bool:
        """该 slot 是否已收到 end 标识"""
        with self._lock:
            return slot in self._ended_slots

    def clear_slot_end(self, slot: int) -> None:
        """清除某个 slot 的 end 标识"""
        with self._lock:
            self._ended_slots.discard(slot)

    def get_transactions_for_slot(self, slot: int) -> List[BlockTransaction]:
        """获取特定 slot 的所有交易"""
        with self._lock:
            return self._transactions.get(slot, []).copy()

    def remove_transactions_for_slot(self, slot: int) -> List[BlockTransaction]:
        """移除并返回特定 slot 的所有交易"""
        with self._lock:
            transactions = self._transactions.pop(slot, [])
            self._ended_slots.discard(slot)
            return transactions

    def remove_transactions(self, slot: int, signatures: Set[str]) -> None:
        """只移除指定签名的交易，其余保留（用于延迟策略）"""
        if not signatures:
            return
        with self._lock:
            remaining = [tx for tx in self._transactions.get(slot, []) if tx.signature not in signatures]
            if remaining:
                self._transactions[slot] = remaining
            else:
                self._transactions.pop(slot, None)

    def add_sent_record(self, record: TransactionRecord) -> None:
        """记录已发送的交易"""
        with self._lock:
            self._sent_records.append(record)
            # logger.info(f"记录发送: signature={record.signature}, validator={record.validator}, strategy={record.strategy}")

    def get_sent_records(self) -> List[TransactionRecord]:
        """获取所有发送记录"""
        with self._lock:
            return self._sent_records.copy()

    def get_pending_slots(self) -> Set[int]:
        """获取所有有待处理交易的 slot 号"""
        with self._lock:
            return set(self._transactions.keys())

    def clear(self) -> None:
        """清空所有数据"""
        with self._lock:
            self._transactions.clear()
            self._ended_slots.clear()
            self._sent_records.clear()
