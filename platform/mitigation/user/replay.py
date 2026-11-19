"""区块重放数据源：纯业务，不依赖网络。

server + client 模式和 sim 模式共用这一份区块加载逻辑。
"""
from typing import Dict, List

from type.models import BlockTransaction
from utils import build_block_index, load_block_transactions
from log import user_logger as logger


class BlockReplaySource:
    """按 slot 提供历史区块交易，按需加载并缓存。"""

    def __init__(self, block_data_path: str):
        self.block_data_path = block_data_path

        # slot 号 -> 该 slot 的 BlockTransaction 列表（按需加载并缓存）
        self.BLOCKS_DATA: Dict[int, List[BlockTransaction]] = {}
        # slot 号 -> 对应的区块文件路径
        self._block_index = build_block_index(block_data_path)

        if self._block_index:
            slots = sorted(self._block_index)
            logger.info(
                f"加载区块索引完成: slot 范围 {slots[0]}-{slots[-1]}, "
                f"共 {len(slots)} 个 slot"
            )
        else:
            logger.warning(
                f"区块数据路径无可用文件: {block_data_path}，将使用测试数据"
            )

    def _generate_test_transactions(self, slot: int) -> List[BlockTransaction]:
        """没有真实区块数据时的测试交易"""
        return [
            BlockTransaction(
                slot=slot,
                timestamp=0,
                signatures=[f'sig_{slot}_{i}'],
                message={},
                fee=5000,
                preBalances=[],
                postBalances=[],
                instructions=[],
                logMessages=[],
            )
            for i in range(5)
        ]

    def transactions_for(self, slot: int) -> List[BlockTransaction]:
        """获取指定 slot 的交易（首次访问时加载）"""
        if slot in self.BLOCKS_DATA:
            return self.BLOCKS_DATA[slot]

        transactions: List[BlockTransaction] = []
        block_file = self._block_index.get(slot)
        if block_file is not None:
            try:
                transactions = load_block_transactions(block_file, slot=slot)
            except Exception as e:
                logger.warning(f"无法加载 {block_file}: {e}")

        if not transactions and not self._block_index:
            transactions = self._generate_test_transactions(slot)

        self.BLOCKS_DATA[slot] = transactions
        logger.debug(f"slot {slot}: 从 BLOCKS_DATA 加载 {len(transactions)} 笔交易")
        return transactions

    def available_slots(self) -> List[int]:
        """获取所有可用的 slot 号"""
        return sorted(self._block_index.keys())
