from typing import Dict, List, Set
from collections import Counter, defaultdict
import threading
from type.models import BlockTransaction, TransactionRecord
from log import transaction_manager_logger as logger


class TransactionManager:
    """管理接收到的交易、slot 结束标识，以及发送统计。

    未延迟的交易只累加计数，不逐笔留存；延迟交易保留完整记录，
    用于回答「哪些交易被延迟、从哪个 slot 挪到哪个 slot、延迟了多少」。
    """

    def __init__(self):
        # key 是 slot 号
        self._transactions: Dict[int, List[BlockTransaction]] = defaultdict(list)
        self._ended_slots: Set[int] = set()
        self._lock = threading.Lock()

        self._sent_count: int = 0
        self._sent_delay_sum: int = 0
        self._sent_delayed: List[TransactionRecord] = []

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
        """累计发送计数；只有延迟交易才保留逐笔详情。"""
        with self._lock:
            self._sent_count += 1
            delay = max(0, record.delay_slots)
            self._sent_delay_sum += delay
            if delay > 0:
                self._sent_delayed.append(record)

    def get_delayed_records(self) -> List[TransactionRecord]:
        """获取延迟交易的逐笔详情（拷贝）"""
        with self._lock:
            return self._sent_delayed.copy()

    @staticmethod
    def _delay_stats(count: int, total: int, minimum: int, maximum: int) -> dict:
        if count == 0:
            return {"count": 0, "sum": 0, "avg": 0.0, "min": 0, "max": 0}
        return {
            "count": count,
            "sum": total,
            "avg": round(total / count, 6),
            "min": minimum,
            "max": maximum,
        }

    def get_report(self) -> dict:
        """发送概况：总量、延迟详情、延迟分布与 slot 迁移概况。"""
        with self._lock:
            delayed = self._sent_delayed.copy()
            total = self._sent_count
            delay_sum = self._sent_delay_sum

        delays = [r.delay_slots for r in delayed]
        delayed_count = len(delayed)
        immediate_count = total - delayed_count

        # 全部交易里未延迟的记 0，所以最小值在有 immediate 时恒为 0
        all_min = 0 if immediate_count > 0 else (min(delays) if delays else 0)
        all_max = max(delays) if delays else 0

        original_counts = Counter(r.slot for r in delayed)
        target_counts = Counter(r.target_slot for r in delayed)
        move_counts = Counter((r.slot, r.target_slot) for r in delayed)

        return {
            "total_processed": total,
            "immediate_count": immediate_count,
            "delayed_count": delayed_count,
            "delay_stats": {
                "all": self._delay_stats(total, delay_sum, all_min, all_max),
                "delayed": self._delay_stats(
                    delayed_count, delay_sum,
                    min(delays) if delays else 0,
                    max(delays) if delays else 0,
                ),
            },
            "slot_overview": {
                "original": {str(slot): count for slot, count in sorted(original_counts.items())},
                "target": {str(slot): count for slot, count in sorted(target_counts.items())},
                "moves": [
                    {"original_slot": original, "target_slot": target, "count": count}
                    for (original, target), count in sorted(move_counts.items())
                ],
            },
            "delayed_records": [
                {
                    "signature": r.signature,
                    "slot": r.slot,
                    "target_slot": r.target_slot,
                    "delay_slots": r.delay_slots,
                    "validator": r.validator,
                    "leader_risky": r.leader_risky,
                    "strategy": r.strategy,
                    "sent_at": r.sent_at,
                }
                for r in delayed
            ],
        }

    def get_pending_slots(self) -> Set[int]:
        """获取所有有待处理交易的 slot 号"""
        with self._lock:
            return set(self._transactions.keys())

    def clear(self) -> None:
        """清空所有数据"""
        with self._lock:
            self._transactions.clear()
            self._ended_slots.clear()
            self._sent_count = 0
            self._sent_delay_sum = 0
            self._sent_delayed.clear()
