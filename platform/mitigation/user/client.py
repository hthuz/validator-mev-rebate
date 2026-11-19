"""
User module for simulating transaction sending by replaying historical blocks.

新的 slot 机制：
  - user 连接 server 后监听 new_slot 事件（事件名需与 server.py 保持一致）
  - 收到广播里的 slot 号后，从 BLOCKS_DATA 取该 slot 的交易
  - 逐笔发送交易，最后发送 end 标识表示本 slot 交易全部发完
  - 然后等待 server 的下一个 slot 广播

交易统一用公共类型 BlockTransaction，从 block JSON 按需加载。
"""
import requests
import socketio
from typing import List, Dict, Any

from config import SERVER_PORT, BLOCK_PATH
from log import user_logger as logger
from type.models import BlockTransaction
from utils import build_block_index, load_block_transactions

NEW_SLOT_EVENT = 'new_slot'
SIMULATION_DONE_EVENT = 'simulation_done'


class TransactionUser:
    """模拟用户：根据 server 广播的 slot 号重放该 slot 的区块交易"""

    def __init__(self, server_url: str, block_data_path: str):
        self.server_url = server_url.rstrip('/')
        self.block_data_path = block_data_path

        # BLOCKS_DATA: slot 号 -> 该 slot 的 BlockTransaction 列表（按需加载并缓存）
        self.BLOCKS_DATA: Dict[int, List[BlockTransaction]] = {}
        # slot 号 -> 对应的区块文件路径
        self._block_index = build_block_index(block_data_path)

        self._sio = socketio.Client(logger=False, engineio_logger=False)
        self._running = False
        self._register_socket_events()

        if self._block_index:
            slots = sorted(self._block_index)
            logger.info(
                f"加载区块索引完成: slot 范围 {slots[0]}-{slots[-1]}, "
                f"共 {len(slots)} 个 slot"
            )
        else:
            logger.warning(f"区块数据路径无可用文件: {block_data_path}，将使用测试数据")

    # ------------------------------------------------------------------
    # Socket.IO 事件
    # ------------------------------------------------------------------
    def _register_socket_events(self) -> None:
        @self._sio.on(NEW_SLOT_EVENT)
        def on_new_slot(data: Dict[str, Any]):
            slot = int(data['slot'])
            logger.info(f"收到新 slot 广播: {slot}")
            self._send_slot_transactions(slot)

        @self._sio.on(SIMULATION_DONE_EVENT)
        def on_simulation_done(data: Dict[str, Any]):
            logger.info(f"服务器通知模拟结束: {data}")
            self._running = False
            self._sio.disconnect()

    # ------------------------------------------------------------------
    # 区块数据（按需加载）
    # ------------------------------------------------------------------
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

    def _get_transactions_for_slot(self, slot: int) -> List[BlockTransaction]:
        """从 BLOCKS_DATA 获取指定 slot 的交易（首次访问时加载）"""
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

    def get_available_blocks(self) -> List[int]:
        """获取所有可用的 slot 号"""
        return sorted(self._block_index.keys())

    # ------------------------------------------------------------------
    # 发送交易
    # ------------------------------------------------------------------
    def send_transaction(self, tx: BlockTransaction, slot: int) -> bool:
        """发送单笔交易到服务器（发送 BlockTransaction 的完整字段）"""
        try:
            payload = tx.to_dict()
            payload['slot'] = slot

            response = requests.post(
                f"{self.server_url}/sendTransaction", json=payload, timeout=30
            )

            if response.status_code == 200:
                logger.info(f"✓ 发送交易成功: {tx.signature}, slot: {slot}")
                return True
            else:
                logger.error(f"✗ 发送交易失败: {response.status_code}, {response.text}")
                return False

        except Exception as e:
            logger.error(f"✗ 发送交易异常: {e}")
            return False

    def send_end(self, slot: int) -> bool:
        """发送 end 标识，告诉 server 该 slot 的交易已全部发完"""
        try:
            response = requests.post(
                f"{self.server_url}/sendTransaction",
                json={'slot': slot, 'end': True},
                timeout=30,
            )
            if response.status_code == 200:
                logger.info(f"✓ slot {slot} 交易发送结束，已发送 end 标识")
                return True
            else:
                logger.error(f"✗ 发送 end 标识失败: {response.status_code}, {response.text}")
                return False
        except Exception as e:
            logger.error(f"✗ 发送 end 标识异常: {e}")
            return False

    def _send_slot_transactions(self, slot: int) -> None:
        """把某个 slot 的全部交易发给 server，然后发 end 标识"""
        transactions = self._get_transactions_for_slot(slot)
        logger.info(f"slot {slot}: 发送 {len(transactions)} 笔交易")

        for tx in transactions:
            self.send_transaction(tx, slot)

        self.send_end(slot)

    # ------------------------------------------------------------------
    # 运行
    # ------------------------------------------------------------------
    def run(self) -> None:
        """连接服务器，之后完全由服务器广播驱动"""
        self._running = True
        logger.info(f"连接服务器: {self.server_url}")
        self._sio.connect(self.server_url)
        self._sio.wait()

    def stop(self) -> None:
        """停止用户客户端"""
        self._running = False
        if self._sio.connected:
            self._sio.disconnect()


def create_user(
    server_url: str = None, block_data_path: str = None
) -> TransactionUser:
    """工厂函数：创建用户实例"""
    if server_url is None:
        server_url = f"http://localhost:{SERVER_PORT}"
    if block_data_path is None:
        block_data_path = BLOCK_PATH

    return TransactionUser(server_url, block_data_path)


if __name__ == "__main__":
    user = create_user()
    logger.info(f"可用区块: {user.get_available_blocks()[:5]} ...")
    user.run()
