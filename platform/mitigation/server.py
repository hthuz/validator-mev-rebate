from flask import Flask, request, jsonify
from flask_socketio import SocketIO
from typing import Dict, Any
from .transaction_manager import TransactionManager
from .slot_manager import SlotManager
from .strategy import ImmediateStrategy, SendStrategy
from type.models import Transaction
from config import SERVER_HOST, SERVER_PORT, DEBUG_MODE
from log import server_logger as logger

# 事件名
NEW_SLOT_EVENT = 'new_slot'
SIMULATION_DONE_EVENT = 'simulation_done'


class TransactionServer:
    """
    交易接收服务器。

    slot 机制（事件驱动，无定时器）：
      1. 进入新 slot -> 广播 new_slot 给 user，告诉它 slot start time
      2. user 从 BLOCKS_DATA 取该 slot 的交易并逐笔发送，最后发 end 标识
      3. server 按当前策略处理交易（immediate 策略收到就立刻发给验证者）
      4. 收到 end 标识 -> 处理收尾交易 -> 更新到下一个 slot -> 回到 1
    """

    def __init__(self, strategy: SendStrategy = None):
        self.app = Flask(__name__)
        self.socketio = SocketIO(self.app, cors_allowed_origins="*", async_mode="threading")
        self.transaction_manager = TransactionManager()
        self.slot_manager = SlotManager()
        self.strategy = strategy or ImmediateStrategy()
        self._running = False

        # Debug 模式只影响日志详细程度，slot 推进完全由 end 标识驱动
        self.debug_mode = DEBUG_MODE

        # 一开始就指向第一个 slot start time，保证 connect 时广播的是有效值
        self.slot_manager.reset_to_first_slot()

        self._register_routes()
        self._register_socket_events()

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
            """接收用户发送的交易，或 slot 的 end 标识"""
            try:
                data = request.get_json()
                if not data:
                    return jsonify({'error': 'No data provided'}), 400

                if 'slot' not in data:
                    return jsonify({'error': 'Missing required field: slot'}), 400
                try:
                    slot = int(data['slot'])
                except (TypeError, ValueError):
                    return jsonify({'error': 'Invalid slot (expected slot start time)'}), 400

                # ---- end 标识：该 slot 的用户交易全部发送完毕 ----
                if data.get('end') is True:
                    self.transaction_manager.mark_slot_end(slot)
                    self._on_slot_complete(slot)
                    return jsonify({'status': 'end_accepted', 'slot': slot}), 200

                # ---- 普通交易 ----
                required_fields = ['tx_id', 'signature']
                for field in required_fields:
                    if field not in data:
                        return jsonify({'error': f'Missing required field: {field}'}), 400

                transaction = Transaction(
                    tx_id=data['tx_id'],
                    slot=slot,
                    signature=data['signature'],
                    data=data
                )

                self.transaction_manager.add_transaction(transaction)

                # 按策略处理。immediate 策略在这里就会立刻发送给验证者，
                # 不用等到 slot 推进。
                self._process_transactions()

                return jsonify({
                    'status': 'accepted',
                    'tx_id': transaction.tx_id,
                    'slot': transaction.slot
                }), 200

            except Exception as e:
                return jsonify({'error': str(e)}), 500

        @self.app.route('/transactions/pending', methods=['GET'])
        def get_pending():
            pending_slots = self.transaction_manager.get_pending_slots()
            return jsonify({
                'pending_slots': sorted(list(pending_slots)),
                'count': len(pending_slots)
            })

        @self.app.route('/transactions/sent', methods=['GET'])
        def get_sent():
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
            """获取当前 slot start time"""
            return jsonify({
                'current_slot': self.slot_manager.get_current_slot()
            })

        @self.app.route('/slot/advance', methods=['POST'])
        def advance_slot_manual():
            """手动完成当前 slot 并推进（仅 debug 模式下可用）"""
            if not self.debug_mode:
                return jsonify({'error': 'Not in debug mode'}), 400
            current = self.slot_manager.get_current_slot()
            self._on_slot_complete(current)
            return jsonify({
                'status': 'ok',
                'message': 'Slot completed',
                'current_slot': self.slot_manager.get_current_slot()
            })

    def _register_socket_events(self):
        """注册 Socket.IO 事件"""

        @self.socketio.on('connect')
        def on_connect(auth=None):
            sid = request.sid
            logger.info(f"User 已连接 (sid={sid})，发送当前 slot")
            # user 一连上来就告诉它当前 slot，保证链路能启动。
            # 后续新 slot 由 _broadcast_slot 广播给所有连接的 user。
            self._broadcast_slot(to=sid)

        @self.socketio.on('disconnect')
        def on_disconnect():
            logger.info("User 已断开连接")

    def _broadcast_slot(self, to: str = None) -> None:
        """广播新的 slot start time 给 user"""
        slot = self.slot_manager.get_current_slot()
        payload = {
            'slot': slot,
            'leader': self.slot_manager.get_leader(slot),
        }
        if to is None:
            self.socketio.emit(NEW_SLOT_EVENT, payload)
            logger.info(f"广播新 slot (start time): {slot}")
        else:
            self.socketio.emit(NEW_SLOT_EVENT, payload, to=to)
            logger.info(f"向 {to} 发送当前 slot (start time): {slot}")

    def _process_transactions(self) -> None:
        """按当前策略把到期的交易发送给验证者"""
        current_slot = self.slot_manager.get_current_slot()
        leader = self.slot_manager.get_leader(current_slot)
        if not leader or leader == "unknown":
            logger.debug(f"Slot {current_slot} 没有对应的 leader")
            return

        for slot in sorted(self.transaction_manager.get_pending_slots()):
            transactions = self.transaction_manager.get_transactions_for_slot(slot)
            sent_ids = set()

            for tx in transactions:
                if self.strategy.should_send(tx, current_slot):
                    record = self.strategy.send_to_validator(tx, leader)
                    self.transaction_manager.add_sent_record(record)
                    sent_ids.add(tx.tx_id)

            # 只移除已发送的，延迟策略未到期的交易继续保留
            if sent_ids:
                self.transaction_manager.remove_transactions(slot, sent_ids)

    def _on_slot_complete(self, slot: int) -> None:
        """
        收到 slot 的 end 标识：
        先按策略处理完交易，再更新 slot，然后广播下一个 slot。
        """
        current_slot = self.slot_manager.get_current_slot()
        if slot != current_slot:
            logger.warning(
                f"收到 slot {slot} 的 end 标识，但当前 slot 是 {current_slot}，忽略"
            )
            return

        logger.info(f"========== Slot {current_slot} 交易接收完成 ==========")

        # 1. 处理完本 slot 收到的交易
        self._process_transactions()

        # 2. 更新 slot（不再等待固定时间）
        if not self.slot_manager.has_next_slot():
            logger.info("所有 slot 处理完成，模拟结束")
            self._running = False
            self.socketio.emit(SIMULATION_DONE_EVENT, {'last_slot': current_slot})
            return

        self.slot_manager.advance_slot()

        # 3. 新 slot 下，延迟策略里到期的交易可以发送
        self._process_transactions()

        # 4. 广播新 slot，开始下一轮
        self._broadcast_slot()

    def set_strategy(self, strategy: SendStrategy):
        """动态切换发送策略"""
        self.strategy = strategy
        logger.info(f"策略切换为: {strategy.get_strategy_name()}")

    def start(self):
        """启动服务器：slot 推进完全由 user 的 end 标识驱动"""
        self._running = True

        logger.info(f"启动服务器在 {SERVER_HOST}:{SERVER_PORT}")
        logger.info(f"使用策略: {self.strategy.get_strategy_name()}")
        logger.info("Slot 推进机制: 广播新slot -> user发交易+end -> 处理 -> 推进")
        if self.debug_mode:
            logger.info("🐛 DEBUG模式已启用")

        # Flask-SocketIO 的 run 自带 WSGI 服务器
        self.socketio.run(
            self.app,
            host=SERVER_HOST,
            port=SERVER_PORT,
            allow_unsafe_werkzeug=True,
        )

    def stop(self):
        """停止服务器"""
        self._running = False
        self.socketio.stop()


def create_server(strategy: SendStrategy = None) -> TransactionServer:
    """工厂函数：创建服务器实例"""
    return TransactionServer(strategy=strategy)


if __name__ == "__main__":
    server = create_server()
    server.start()
