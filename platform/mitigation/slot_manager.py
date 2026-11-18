from typing import Dict, List, Optional

import pandas as pd

from config import BLOCK_PATH, SLOT_LEADER_PATH
from log import slot_manager_logger as logger
from utils import build_block_time_index, extract_slot_from_block_path

# 没有区块数据时的测试用起始时间（Unix 秒），slot start time 按秒递增
_TEST_SLOT_START_TIME = 1749916800
_TEST_SLOT_COUNT = 100


class SlotManager:
    """
    管理 slot 及其 leader。

    ⚠️ slot 统一用「slot start time」表示：
    即区块的 blockTime（Unix 秒时间戳），而不是 slot 序号。
    同一秒内产生的多个区块归为同一个模拟 slot。
    """

    def __init__(self):
        # 真实 slot 序号 -> leader（来自 CSV，仅用于建立时间片 leader 映射）
        self._raw_leaders: Dict[int, str] = {}
        # key: slot start time (blockTime), value: validator leader
        self._slot_leaders: Dict[int, str] = {}
        # 有序的 slot start time 列表
        self._slots: List[int] = []
        self._current_index: int = -1
        self._current_slot: int = 0

        self._load_slot_leaders()
        self._load_slots()

    def _load_slot_leaders(self) -> None:
        """从 CSV 加载真实 slot 序号 -> leader 的映射"""
        try:
            df = pd.read_csv(SLOT_LEADER_PATH)
            logger.info(f"加载slot leaders文件: {SLOT_LEADER_PATH}")
            logger.debug(f"列名: {df.columns.tolist()}")

            if 'slot' in df.columns and 'leader' in df.columns:
                for _, row in df.iterrows():
                    self._raw_leaders[int(row['slot'])] = str(row['leader'])
            elif len(df.columns) >= 2:
                for _, row in df.iterrows():
                    self._raw_leaders[int(row.iloc[0])] = str(row.iloc[1])

            logger.info(f"加载了 {len(self._raw_leaders)} 条slot leader记录")
        except Exception as e:
            logger.warning(f"无法加载slot leaders文件: {e}")
            self._raw_leaders = {}

    def _load_slots(self) -> None:
        """从区块数据建立 slot start time 序列，并绑定对应的 leader"""
        block_index = build_block_time_index(BLOCK_PATH)

        if not block_index:
            logger.warning(f"没有可用的区块数据 ({BLOCK_PATH})，使用测试数据")
            self._build_test_slots()
            return

        for start_time in sorted(block_index):
            block_files = sorted(
                block_index[start_time],
                key=lambda p: extract_slot_from_block_path(p) or 0,
            )

            # 同一秒内的多个真实 slot 共用一个 start time，
            # 取最早那个 slot 的 leader 作为该时间片的 leader。
            leader = None
            for block_file in block_files:
                real_slot = extract_slot_from_block_path(block_file)
                if real_slot is not None and real_slot in self._raw_leaders:
                    leader = self._raw_leaders[real_slot]
                    break

            self._slots.append(start_time)
            self._slot_leaders[start_time] = leader or "unknown"

        logger.info(f"加载了 {len(self._slots)} 个 slot start time")
        logger.info(f"Slot start time 范围: {min(self._slots)} - {max(self._slots)}")

    def _build_test_slots(self) -> None:
        """无真实区块数据时的测试 slot start time 序列"""
        count = len(self._raw_leaders) or _TEST_SLOT_COUNT
        for i in range(count):
            start_time = _TEST_SLOT_START_TIME + i
            real_slot = sorted(self._raw_leaders)[i] if self._raw_leaders else None
            self._slots.append(start_time)
            self._slot_leaders[start_time] = (
                self._raw_leaders[real_slot] if real_slot is not None else f"validator_{i % 5}"
            )
        logger.info(f"使用测试数据: {len(self._slots)} 个 slot start time")

    def get_leader(self, slot: int) -> Optional[str]:
        """获取指定 slot start time 对应的 leader"""
        return self._slot_leaders.get(slot)

    def set_current_slot(self, slot: int) -> None:
        """把当前 slot 设置为指定的 slot start time"""
        if slot not in self._slot_leaders:
            logger.warning(f"slot start time {slot} 不在已知slot列表中")
        self._current_slot = slot
        if slot in self._slots:
            self._current_index = self._slots.index(slot)
        logger.info(f"当前slot (start time) 更新为: {slot}")

    def reset_to_first_slot(self) -> int:
        """回到第一个 slot start time，返回该值"""
        if not self._slots:
            logger.warning("没有可用的slot")
            return self._current_slot
        self._current_index = 0
        self._current_slot = self._slots[0]
        logger.info(f"当前slot (start time) 重置为: {self._current_slot}")
        return self._current_slot

    def get_current_slot(self) -> int:
        """获取当前 slot start time"""
        return self._current_slot

    def has_next_slot(self) -> bool:
        """是否还有下一个 slot"""
        return 0 <= self._current_index < len(self._slots) - 1

    def advance_slot(self) -> int:
        """推进到下一个 slot start time"""
        if not self.has_next_slot():
            logger.warning("已经是最后一个slot，无法继续推进")
            return self._current_slot
        self._current_index += 1
        self._current_slot = self._slots[self._current_index]
        logger.info(f"推进到下一个slot (start time): {self._current_slot}")
        return self._current_slot

    def get_available_slots(self) -> List[int]:
        """获取所有可用的 slot start time（升序）"""
        return list(self._slots)
