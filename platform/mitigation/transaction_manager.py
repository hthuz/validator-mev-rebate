from typing import Dict, List, Set
from collections import defaultdict
import threading
from type.models import Transaction, TransactionRecord
from log import transaction_manager_logger as logger


class TransactionManager:
    """管理接收到的交易"""

    def __init__(self):
        self._transactions: Dict[int, List[Transaction]] = defaultdict(list)
        self._lock = threading.Lock()
        self._sent_records: List[TransactionRecord] = []

    def add_transaction(self, transaction: Transaction) -> None:
        """添加一个交易到队列"""
        with self._lock:
            self._transactions[transaction.slot].append(transaction)
            logger.info(f"接收交易: {transaction.tx_id}, slot: {transaction.slot}")

    def get_transactions_for_slot(self, slot: int) -> List[Transaction]:
        """获取特定slot的所有交易"""
        with self._lock:
            return self._transactions.get(slot, []).copy()

    def remove_transactions_for_slot(self, slot: int) -> List[Transaction]:
        """移除并返回特定slot的所有交易"""
        with self._lock:
            transactions = self._transactions.pop(slot, [])
            return transactions

    def add_sent_record(self, record: TransactionRecord) -> None:
        """记录已发送的交易"""
        with self._lock:
            self._sent_records.append(record)
            logger.info(f"记录发送: tx_id={record.tx_id}, validator={record.validator}, strategy={record.strategy}")

    def get_sent_records(self) -> List[TransactionRecord]:
        """获取所有发送记录"""
        with self._lock:
            return self._sent_records.copy()

    def get_pending_slots(self) -> Set[int]:
        """获取所有有待处理交易的slot"""
        with self._lock:
            return set(self._transactions.keys())

    def clear(self) -> None:
        """清空所有数据"""
        with self._lock:
            self._transactions.clear()
            self._sent_records.clear()
