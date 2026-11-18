"""
User module for simulating transaction sending by replaying historical blocks
"""
import os
import json
import time
import requests
from typing import List, Dict, Any, Optional
from pathlib import Path
from ...logger import user_logger as logger


class TransactionUser:
    """模拟用户：从历史区块数据中重放交易"""

    def __init__(self, server_url: str, block_data_path: str):
        self.server_url = server_url.rstrip('/')
        self.block_data_path = block_data_path
        self._blocks_data: Dict[int, List[Dict[str, Any]]] = {}
        self._load_block_data()

    def _load_block_data(self) -> None:
        """从BLOCK_PATH加载区块数据"""
        logger.info(f"加载区块数据: {self.block_data_path}")

        if not os.path.exists(self.block_data_path):
            logger.warning(f"区块数据路径不存在: {self.block_data_path}")
            return

        # 遍历目录中的所有JSON文件
        block_files = list(Path(self.block_data_path).glob("*.json"))
        logger.info(f"找到 {len(block_files)} 个区块文件")

        for block_file in block_files:
            try:
                with open(block_file, 'r') as f:
                    block_data = json.load(f)

                # 提取slot和交易信息
                # 根据实际数据格式调整
                slot = block_data.get('slot') or block_data.get('blockHeight')
                transactions = block_data.get('transactions', [])

                if slot is not None:
                    self._blocks_data[int(slot)] = transactions
                    logger.debug(f"加载slot {slot}: {len(transactions)} 笔交易")

            except Exception as e:
                logger.warning(f"无法加载 {block_file}: {e}")

        if self._blocks_data:
            min_slot = min(self._blocks_data.keys())
            max_slot = max(self._blocks_data.keys())
            total_txs = sum(len(txs) for txs in self._blocks_data.values())
            logger.info(f"加载完成: slot范围 {min_slot}-{max_slot}, 共 {total_txs} 笔交易")
        else:
            logger.warning("没有加载到任何区块数据，使用模拟数据")
            # 生成一些测试数据
            self._generate_test_data()

    def _generate_test_data(self) -> None:
        """生成测试数据（当没有真实数据时）"""
        logger.info("生成测试数据")
        for slot in range(101, 110):
            transactions = []
            for i in range(5):  # 每个slot 5笔交易
                transactions.append({
                    'signature': f'sig_{slot}_{i}',
                    'slot': slot,
                    'fee': 5000,
                    'status': 'success'
                })
            self._blocks_data[slot] = transactions
        logger.info(f"生成了 {len(self._blocks_data)} 个slot的测试数据")

    def send_transaction(self, tx_data: Dict[str, Any]) -> bool:
        """发送单个交易到服务器"""
        try:
            url = f"{self.server_url}/sendTransaction"

            # 构造交易数据
            payload = {
                'tx_id': tx_data.get('signature', f"tx_{time.time()}"),
                'slot': tx_data['slot'],
                'signature': tx_data.get('signature', ''),
                'fee': tx_data.get('fee', 0),
                'status': tx_data.get('status', 'unknown'),
                'raw_data': tx_data
            }

            response = requests.post(url, json=payload, timeout=5)

            if response.status_code == 200:
                logger.info(f"✓ 发送交易成功: {payload['tx_id']}, slot: {payload['slot']}")
                return True
            else:
                logger.error(f"✗ 发送交易失败: {response.status_code}, {response.text}")
                return False

        except Exception as e:
            logger.error(f"✗ 发送交易异常: {e}")
            return False

    def replay_slot(self, current_server_slot: int, send_block_slot: int) -> int:
        """
        当服务器处于current_server_slot时，发送send_block_slot的所有交易

        Args:
            current_server_slot: 服务器当前的slot号
            send_block_slot: 要发送的区块的slot号

        Returns:
            成功发送的交易数量
        """
        transactions = self._blocks_data.get(send_block_slot, [])

        if not transactions:
            return 0

        logger.info(f"服务器slot {current_server_slot}: 发送区块 {send_block_slot} 的 {len(transactions)} 笔交易")

        success_count = 0
        for tx in transactions:
            if self.send_transaction(tx):
                success_count += 1
            time.sleep(0.01)  # 短暂延迟，避免过快

        return success_count

    def get_available_blocks(self) -> List[int]:
        """获取所有可用的区块slot"""
        return sorted(self._blocks_data.keys())

    def run_simulation(self, start_slot: int = None, delay: float = 0.4):
        """
        运行完整的模拟：
        - 当服务器在slot N时，发送区块 N+1 的交易
        """
        available_blocks = self.get_available_blocks()

        if not available_blocks:
            logger.warning("没有可用的区块数据")
            return

        if start_slot is None:
            start_slot = min(available_blocks) - 1

        logger.info(f"开始模拟，起始slot: {start_slot}")
        logger.info(f"可用区块范围: {min(available_blocks)} - {max(available_blocks)}")

        current_slot = start_slot

        try:
            while True:
                # 当前slot模拟
                send_block = current_slot + 1

                if send_block in self._blocks_data:
                    self.replay_slot(current_slot, send_block)

                # 推进到下一个slot
                current_slot += 1
                time.sleep(delay)

                # 如果超出范围，结束
                if current_slot > max(available_blocks):
                    logger.info(f"模拟完成，已到达最后的slot {max(available_blocks)}")
                    break

        except KeyboardInterrupt:
            logger.info("模拟被用户中断")


def create_user(server_url: str = "http://localhost:8080", block_data_path: str = None) -> TransactionUser:
    """工厂函数：创建用户实例"""
    if block_data_path is None:
        from ..config import BLOCK_PATH
        block_data_path = BLOCK_PATH

    return TransactionUser(server_url, block_data_path)


if __name__ == "__main__":
    # 测试用例
    user = create_user()
    logger.info(f"可用区块: {user.get_available_blocks()}")
