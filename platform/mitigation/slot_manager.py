from typing import Dict, List, Optional

import pandas as pd

from config import (
    SLOT_LEADER_PATH,
    MITIGATION_START_SLOT,
    MITIGATION_END_SLOT,
)
from log import slot_manager_logger as logger


class SlotManager:
    """
    管理 slot 及其 leader。

    slot 就是 slot 号（与 block 是同一个概念，整数），跟 blockTime/timestamp 无关。
    server 记录的 slot 表示「user 需要发送这个 slot 的交易」；
    处理完该 slot 的交易后推进到下一个 slot。

    slot 范围由 config 的 MITIGATION_START_SLOT / MITIGATION_END_SLOT 决定，
    两端都是 inclusive，每次推进固定 +1。
    """

    def __init__(self):
        # key: slot 号, value: validator leader
        self._slot_leaders: Dict[int, str] = {}

        self._start_slot: int = MITIGATION_START_SLOT
        self._end_slot: int = MITIGATION_END_SLOT
        self._current_slot: int = MITIGATION_START_SLOT

        if self._start_slot > self._end_slot:
            raise ValueError(
                f"MITIGATION_START_SLOT ({self._start_slot}) 不能大于 "
                f"MITIGATION_END_SLOT ({self._end_slot})"
            )

        self._load_slot_leaders()
        logger.info(
            f"Slot 范围: {self._start_slot} - {self._end_slot} (inclusive)，"
            f"共 {self.end_slot - self.start_slot + 1} 个"
        )

    def _load_slot_leaders(self) -> None:
        try:
            df = pd.read_csv(SLOT_LEADER_PATH)
            logger.info(f"加载slot leaders文件: {SLOT_LEADER_PATH}")
            logger.debug(f"列名: {df.columns.tolist()}")

            if 'slot' in df.columns and 'leader' in df.columns:
                for _, row in df.iterrows():
                    self._slot_leaders[int(row['slot'])] = str(row['leader'])
            elif len(df.columns) >= 2:
                for _, row in df.iterrows():
                    self._slot_leaders[int(row.iloc[0])] = str(row.iloc[1])

            logger.info(f"加载了 {len(self._slot_leaders)} 条slot leader记录")
        except Exception as e:
            logger.warning(f"无法加载slot leaders文件: {e}")
            self._slot_leaders = {}

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
        logger.info(f"当前slot更新为: {slot}")

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
