import pandas as pd
from typing import Dict, Optional
from type.models import SlotLeader
from config import SLOT_LEADER_PATH
from log import slot_manager_logger as logger


class SlotManager:
    """管理slot和leader信息"""

    def __init__(self):
        self._slot_leaders: Dict[int, str] = {}
        self._current_slot: int = 0
        self._load_slot_leaders()

    def _load_slot_leaders(self) -> None:
        """从CSV文件加载slot leader信息"""
        try:
            df = pd.read_csv(SLOT_LEADER_PATH)
            logger.info(f"加载slot leaders文件: {SLOT_LEADER_PATH}")
            logger.debug(f"列名: {df.columns.tolist()}")

            # 假设CSV格式为: slot, leader
            # 根据实际文件格式调整列名
            if 'slot' in df.columns and 'leader' in df.columns:
                for _, row in df.iterrows():
                    self._slot_leaders[int(row['slot'])] = str(row['leader'])
            elif len(df.columns) >= 2:
                # 如果没有列名，使用前两列
                for _, row in df.iterrows():
                    self._slot_leaders[int(row.iloc[0])] = str(row.iloc[1])

            logger.info(f"加载了 {len(self._slot_leaders)} 个slot leaders")
            if self._slot_leaders:
                min_slot = min(self._slot_leaders.keys())
                max_slot = max(self._slot_leaders.keys())
                logger.info(f"Slot范围: {min_slot} - {max_slot}")
        except Exception as e:
            logger.warning(f"无法加载slot leaders文件: {e}")
            # 测试用：生成一些默认的slot leaders
            for i in range(100, 200):
                self._slot_leaders[i] = f"validator_{i % 5}"
            logger.info(f"使用测试数据: {len(self._slot_leaders)} 个slot leaders")

    def get_leader(self, slot: int) -> Optional[str]:
        """获取指定slot的leader"""
        return self._slot_leaders.get(slot)

    def set_current_slot(self, slot: int) -> None:
        """设置当前slot"""
        self._current_slot = slot
        logger.info(f"当前slot更新为: {slot}")

    def get_current_slot(self) -> int:
        """获取当前slot"""
        return self._current_slot

    def advance_slot(self) -> int:
        """推进到下一个slot（每次+1）"""
        self._current_slot += 1
        return self._current_slot

    def get_available_slots(self) -> list[int]:
        """获取所有可用的slot"""
        return sorted(self._slot_leaders.keys())
