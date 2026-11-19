"""从 sandwiches_nobundle 的 victim 交易构建 replay 数据源。

与 user/replay.py 的 BlockReplaySource 接口一致，sim 可以无缝切换。
"""
from collections import defaultdict
from typing import Callable, Dict, List, Optional

from type.models import SandwichTransaction
from utils.sandwich import get_victim_transactions
from log import user_logger as logger


VictimLoader = Callable[[int], List[SandwichTransaction]]


class VictimReplaySource:
    """按 epoch 一次性加载 victim 交易，按 slot 分组后供 sim 重放。"""

    def __init__(self, epoch: int, loader: Optional[VictimLoader] = None):
        self.epoch = epoch
        transactions = (loader or get_victim_transactions)(epoch)

        self._by_slot: Dict[int, List[SandwichTransaction]] = defaultdict(list)
        for tx in transactions:
            self._by_slot[int(tx.slot)].append(tx)

        # 同一 slot 内按区块位置排序，保持原始相对顺序
        for slot in self._by_slot:
            self._by_slot[slot].sort(
                key=lambda tx: (
                    tx.position if tx.position is not None else 0,
                    tx.signature,
                )
            )

        logger.info(
            f"victim 数据源加载完成: epoch {epoch}, "
            f"{len(transactions)} 笔交易, {len(self._by_slot)} 个 slot"
        )

    def transactions_for(self, slot: int) -> List[SandwichTransaction]:
        return self._by_slot.get(slot, [])

    def available_slots(self) -> List[int]:
        return sorted(self._by_slot)

    def all_transactions(self) -> List[SandwichTransaction]:
        return [
            tx
            for slot in sorted(self._by_slot)
            for tx in self._by_slot[slot]
        ]
