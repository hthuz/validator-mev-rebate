from flask import Flask, request, jsonify
from typing import Dict, Any
import threading
import time
from .transaction_manager import TransactionManager
from .slot_manager import SlotManager
from .strategy import ImmediateStrategy, SendStrategy
from type.models import Transaction
from config import SERVER_HOST, SERVER_PORT, DEBUG_MODE
from log import server_logger as logger


class TransactionServer:
    """交易接收服务器"""

    def __init__(self, strategy: SendStrategy = None):
        self.app = Flask(__name__)
        self.transaction_manager = TransactionManager()
        self.slot_manager = SlotManager()
        self.strategy = strategy or ImmediateStrategy()
        self._running = False
        self._slot_thread = None

        # Debug模式：从配置读取
        self.debug_mode = DEBUG_MODE
        self._slot_advance_event = threading.Event()
        if not self.debug_mode:
            self._slot_advance_event.set()  # 非debug模式下自动推进

        # 注册路由
        self._register_routes()

    def _register_routes(self):
        """注册API路由"""

        @self.app.route('/health', methods=['GET'])
        def health():
            return jsonify({
                'status': 'ok',
                'current_slot': self.slot_manager.get_current_slot(),
                'strategy': self.strategy.get_strategy_name(),
                'debug_mode': self.debug_mode
            })

        @self.app.route('/sendTransaction', methods=['POST'])
        def send_transaction():
            """接收用户交易的端点"""
            try:
                data = request.get_json()

                if not data:
                    return jsonify({'error': 'No data provided'}), 400

                # 验证必需字段
                required_fields = ['tx_id', 'slot', 'signature']
                for field in required_fields:
                    if field not in data:
                        return jsonify({'error': f'Missing required field: {field}'}), 400

                # 创建交易对象
                transaction = Transaction(
                    tx_id=data['tx_id'],
                    slot=int(data['slot']),
                    signature=data['signature'],
                    data=data
                )

                # 添加到交易管理器
                self.transaction_manager.add_transaction(transaction)

                return jsonify({
                    'status': 'accepted',
                    'tx_id': transaction.tx_id,
                    'slot': transaction.slot
                }), 200

            except Exception as e:
                return jsonify({'error': str(e)}), 500

        @self.app.route('/transactions/pending', methods=['GET'])
        def get_pending():
            """获取待处理交易"""
            pending_slots = self.transaction_manager.get_pending_slots()
            return jsonify({
                'pending_slots': sorted(list(pending_slots)),
                'count': len(pending_slots)
            })

        @self.app.route('/transactions/sent', methods=['GET'])
        def get_sent():
            """获取已发送交易记录"""
            records = self.transaction_manager.get_sent_records()
            return jsonify({
                'records': [
                    {
                        'tx_id': r.tx_id,
                        'slot': r.slot,
                        'validator': r.validator,
                        'strategy': r.strategy,
                        'sent_at': r.sent_at
                    }
                    for r in records
                ],
                'count': len(records)
            })

        @self.app.route('/slot/current', methods=['GET'])
        def get_current_slot():
            """获取当前slot"""
            return jsonify({
                'current_slot': self.slot_manager.get_current_slot()
            })

        @self.app.route('/slot/advance', methods=['POST'])
        def advance_slot_manual():
            """手动推进slot（仅在debug模式下）"""
            if not self.debug_mode:
                return jsonify({'error': 'Not in debug mode'}), 400

            # 触发slot推进
            self._slot_advance_event.set()

            return jsonify({
                'status': 'ok',
                'message': 'Slot will advance'
            })

    def _process_slot(self):
        """处理当前slot的交易（根据策略）"""
        current_slot = self.slot_manager.get_current_slot()

        # 获取当前slot的leader
        leader = self.slot_manager.get_leader(current_slot)
        if not leader:
            logger.debug(f"Slot {current_slot} 没有对应的leader")
            return

        # 获取所有待处理的slot
        pending_slots = self.transaction_manager.get_pending_slots()

        # 处理所有应该发送的交易
        for slot in sorted(pending_slots):
            transactions = self.transaction_manager.get_transactions_for_slot(slot)

            for tx in transactions:
                # 根据策略判断是否发送
                if self.strategy.should_send(tx, current_slot):
                    # 发送交易并记录
                    record = self.strategy.send_to_validator(tx, leader)
                    self.transaction_manager.add_sent_record(record)

            # 如果该slot的交易都被处理了，移除它们
            if all(self.strategy.should_send(tx, current_slot) for tx in transactions):
                self.transaction_manager.remove_transactions_for_slot(slot)

    def _keyboard_listener(self):
        """键盘监听线程（debug模式下按Enter推进slot）"""
        import sys
        logger.info("键盘监听已启动，按Enter键推进slot")

        while self._running:
            try:
                input()  # 等待Enter键
                self._slot_advance_event.set()
            except:
                break

    def _slot_advancement_loop(self):
        """Slot推进循环"""
        logger.info("Slot推进线程启动")
        if self.debug_mode:
            logger.info("🐛 DEBUG模式已启用 - 需要手动确认才能推进slot")
            logger.info("   使用 POST /slot/advance 或按Enter键推进")

        # 获取可用的slots
        available_slots = self.slot_manager.get_available_slots()
        if not available_slots:
            logger.warning("没有可用的slots")
            return

        # 从第一个可用slot开始
        start_slot = min(available_slots)
        self.slot_manager.set_current_slot(start_slot - 1)

        while self._running:
            if self.debug_mode:
                # Debug模式：等待手动确认
                logger.info("⏸️  等待手动确认推进到下一个slot...")
                self._slot_advance_event.wait()
                self._slot_advance_event.clear()

            # 推进到下一个slot
            current_slot = self.slot_manager.advance_slot()

            logger.info(f"========== Slot {current_slot} ==========")

            # 处理当前slot的交易
            self._process_slot()

            if not self.debug_mode:
                # 非Debug模式：自动等待400ms
                time.sleep(0.4)

    def set_strategy(self, strategy: SendStrategy):
        """动态切换发送策略"""
        self.strategy = strategy
        logger.info(f"策略切换为: {strategy.get_strategy_name()}")

    def start(self, start_slot_thread: bool = True):
        """启动服务器"""
        self._running = True

        if start_slot_thread:
            # 启动slot推进线程
            self._slot_thread = threading.Thread(target=self._slot_advancement_loop, daemon=True)
            self._slot_thread.start()

            # Debug模式下启动键盘监听线程
            if self.debug_mode:
                self._keyboard_thread = threading.Thread(target=self._keyboard_listener, daemon=True)
                self._keyboard_thread.start()

        logger.info(f"启动服务器在 {SERVER_HOST}:{SERVER_PORT}")
        logger.info(f"使用策略: {self.strategy.get_strategy_name()}")
        if self.debug_mode:
            logger.info("🐛 DEBUG模式: 按Enter键推进slot")

        # 启动Flask服务器
        self.app.run(host=SERVER_HOST, port=SERVER_PORT, threaded=True)

    def stop(self):
        """停止服务器"""
        self._running = False
        if self._slot_thread:
            self._slot_thread.join(timeout=2)


def create_server(strategy: SendStrategy = None) -> TransactionServer:
    """工厂函数：创建服务器实例"""
    return TransactionServer(strategy=strategy)


if __name__ == "__main__":
    server = create_server()
    server.start()
