from typing import Dict, List, Optional

from config import (
    MITIGATION_START_SLOT,
    MITIGATION_END_SLOT,
)
from log import slot_manager_logger as logger
from utils.ch import query_clickhouse


class SlotManager:
    """
    管理 slot 及其 leader。

    slot 就是 slot 号（与 block 是同一个概念，整数），跟 blockTime/timestamp 无关。
    server 记录的 slot 表示「user 需要发送这个 slot 的交易」；
    处理完该 slot 的交易后推进到下一个 slot。

    slot 范围由 config 的 MITIGATION_START_SLOT / MITIGATION_END_SLOT 决定，
    两端都是 inclusive，每次推进固定 +1。leader 从 ClickHouse 的 slot_leaders 表加载，
    不再读本地文件。查询失败时降级为空字典（此时 process 会跳过无 leader 的 slot）。
    """

    def __init__(self, leaders: Optional[Dict[int, str]] = None):
        self._slot_leaders: Dict[int, str] = {}

        self._start_slot: int = MITIGATION_START_SLOT
        self._end_slot: int = MITIGATION_END_SLOT
        self._current_slot: int = MITIGATION_START_SLOT

        if self._start_slot > self._end_slot:
            raise ValueError(
                f"MITIGATION_START_SLOT ({self._start_slot}) 不能大于 "
                f"MITIGATION_END_SLOT ({self._end_slot})"
            )

        self._slot_leaders = (
            dict(leaders) if leaders is not None else self._load_slot_leaders()
        )
        logger.info(
            f"Slot 范围: {self._start_slot} - {self._end_slot} (inclusive)，"
            f"共 {self.end_slot - self.start_slot + 1} 个，"
            f"已加载 {len(self._slot_leaders)} 条 leader"
        )

    def _load_slot_leaders(self) -> Dict[int, str]:
        try:
            rows = query_clickhouse(
                "SELECT slot, leader FROM slot_leaders "
                f"WHERE slot BETWEEN {self._start_slot} AND {self._end_slot}"
            )
            leaders = {int(row['slot']): str(row['leader']) for row in rows}
            logger.info(f"从 ClickHouse 加载了 {len(leaders)} 条 slot leader 记录")
        except Exception as e:
            logger.warning(f"无法从 ClickHouse 加载 slot leaders: {e}")
            leaders = {}
        return leaders

    @property
    def start_slot(self) -> int:
        return self._start_slot

    @property
    def end_slot(self) -> int:
        return self._end_slot

    def get_leader(self, slot: int) -> Optional[str]:
        return self._slot_leaders.get(slot)

    def set_current_slot(self, slot: int) -> None:
        self._current_slot = slot
        # logger.info(f"Current slot: {slot}")

    def reset_to_first_slot(self) -> int:
        self._current_slot = self._start_slot
        logger.info(f"当前slot重置为 MITIGATION_START_SLOT: {self._current_slot}")
        return self._current_slot

    def get_current_slot(self) -> int:
        return self._current_slot

    def has_next_slot(self) -> bool:
        return self._current_slot < self._end_slot

    def advance_slot(self) -> int:
        if not self.has_next_slot():
            logger.warning("已经是最后一个slot，无法继续推进")
            return self._current_slot
        self._current_slot += 1
        logger.info(f"推进到下一个slot: {self._current_slot}")
        return self._current_slot

    def get_available_slots(self) -> List[int]:
        return list(range(self._start_slot, self._end_slot + 1))
