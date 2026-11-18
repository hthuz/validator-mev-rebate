from typing import Dict, List, Optional

import pandas as pd

from config import BLOCK_PATH, SLOT_LEADER_PATH, START_SLOT
from log import slot_manager_logger as logger
from utils import build_block_index

# 没有区块数据时，从 START_SLOT 起生成的 slot 数量
_TEST_SLOT_COUNT = 100


class SlotManager:
    """
    管理 slot 及其 leader。

    slot 就是 slot 号（与 block 是同一个概念，整数），跟 blockTime/timestamp 无关。
    server 记录的 slot 表示「user 需要发送这个 slot 的交易」；
    处理完该 slot 的交易后推进到下一个 slot。

    slot 从配置的 START_SLOT 开始，每次推进固定 +1。
    结束点取区块数据里最大的 slot 号（没有区块数据时用 START_SLOT + 99）。
    """

    def __init__(self):
        # key: slot 号, value: validator leader
        self._slot_leaders: Dict[int, str] = {}

        self._start_slot: int = START_SLOT
        self._end_slot: int = START_SLOT
        self._current_slot: int = START_SLOT

        self._load_slot_leaders()
        self._load_slots()

    def _load_slot_leaders(self) -> None:
        """从 CSV 加载 slot 号 -> leader 的映射"""
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

    def _load_slots(self) -> None:
        """确定 slot 结束点；无区块数据时补充测试用 leader"""
        block_index = build_block_index(BLOCK_PATH)

        if block_index:
            # 结束点 = 区块数据里最大的 slot 号
            self._end_slot = max(block_index)
            logger.info(
                f"Slot 范围: {self._start_slot} - {self._end_slot} "
                f"(从 START_SLOT 起每次 +1，共 {self._end_slot - self._start_slot + 1} 个)"
            )
        else:
            logger.warning(f"没有可用的区块数据 ({BLOCK_PATH})，使用测试数据")
            self._end_slot = self._start_slot + _TEST_SLOT_COUNT - 1
            for offset in range(_TEST_SLOT_COUNT):
                slot = self._start_slot + offset
                self._slot_leaders.setdefault(slot, f"validator_{offset % 5}")
            logger.info(f"测试 slot 范围: {self._start_slot} - {self._end_slot}")

    def get_leader(self, slot: int) -> Optional[str]:
        """获取指定 slot 的 leader"""
        return self._slot_leaders.get(slot)

    def set_current_slot(self, slot: int) -> None:
        """把当前 slot 设置为指定的 slot 号"""
        self._current_slot = slot
        logger.info(f"当前slot更新为: {slot}")

    def reset_to_first_slot(self) -> int:
        """回到配置的 START_SLOT，返回该值"""
        self._current_slot = self._start_slot
        logger.info(f"当前slot重置为 START_SLOT: {self._current_slot}")
        return self._current_slot

    def get_current_slot(self) -> int:
        """获取当前 slot"""
        return self._current_slot

    def has_next_slot(self) -> bool:
        """是否还有下一个 slot"""
        return self._current_slot < self._end_slot

    def advance_slot(self) -> int:
        """推进到下一个 slot（固定 +1）"""
        if not self.has_next_slot():
            logger.warning("已经是最后一个slot，无法继续推进")
            return self._current_slot
        self._current_slot += 1
        logger.info(f"推进到下一个slot: {self._current_slot}")
        return self._current_slot

    def get_available_slots(self) -> List[int]:
        """获取所有可用的 slot（从 START_SLOT 起每次 +1）"""
        return list(range(self._start_slot, self._end_slot + 1))
