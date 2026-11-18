import threading
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
      slot 与 block 是同一个概念（整数 slot 号），跟 blockTime/timestamp 无关。
      1. 进入新 slot -> 广播 new_slot 给 user，告诉它要发送哪个 slot 的交易
      2. user 从 BLOCKS_DATA 取该 slot 的交易并逐笔发送，最后发 end 标识
      3. server 按当前策略处理交易（immediate 策略收到就立刻发给验证者）
      4. 收到 end 标识 -> 处理收尾交易 -> 推进到下一个 slot（+1）-> 回到 1

    Debug 模式下第 4 步多一道手动确认：
      end + 交易处理完之后，还要按 Enter 或 POST /slot/advance 才推进 slot。
    """

    def __init__(self, strategy: SendStrategy = None):
        self.app = Flask(__name__)
        self.socketio = SocketIO(self.app, cors_allowed_origins="*", async_mode="threading")
        self.transaction_manager = TransactionManager()
        self.slot_manager = SlotManager()
        self.strategy = strategy or ImmediateStrategy()
        self._running = False

        self.debug_mode = DEBUG_MODE

        # 当前 slot 已收到 end 且交易处理完，等待手动确认才推进（仅 debug 模式使用）
        self._slot_ready_to_advance = False
        # 防止 Enter 和 HTTP 端点同时触发推进
        self._advance_lock = threading.Lock()
        self._keyboard_thread = None

        # 一开始就指向 START_SLOT，保证 connect 时广播的是有效值
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
                'debug_mode': self.debug_mode,
                'waiting_for_confirmation': self._slot_ready_to_advance
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
                    return jsonify({'error': 'Invalid slot'}), 400

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
            """获取当前 slot"""
            return jsonify({
                'current_slot': self.slot_manager.get_current_slot()
            })

        @self.app.route('/slot/advance', methods=['POST'])
        def advance_slot_manual():
            """debug 模式下手动确认推进 slot（需先收到 end 并处理完交易）"""
            if not self.debug_mode:
                return jsonify({'error': 'Not in debug mode'}), 400

            if self._confirm_slot_advance():
                return jsonify({
                    'status': 'ok',
                    'message': 'Slot advanced',
                    'current_slot': self.slot_manager.get_current_slot()
                })

            return jsonify({
                'status': 'waiting',
                'message': '当前 slot 还未收到 end 标识并处理完交易，无法推进',
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
        """广播新的 slot 给 user"""
        slot = self.slot_manager.get_current_slot()
        payload = {
            'slot': slot,
            'leader': self.slot_manager.get_leader(slot),
        }
        if to is None:
            self.socketio.emit(NEW_SLOT_EVENT, payload)
            logger.info(f"广播新 slot: {slot}")
        else:
            self.socketio.emit(NEW_SLOT_EVENT, payload, to=to)
            logger.info(f"向 {to} 发送当前 slot: {slot}")

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
        先按策略处理完交易，再推进到下一个 slot（debug 模式下需等待手动确认）。
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

        if self.debug_mode:
            # Debug 模式：处理完交易后先停住，等 Enter 或 /slot/advance 确认
            self._slot_ready_to_advance = True
            logger.info("⏸️  交易已处理完，等待手动确认推进 slot（按 Enter 或 POST /slot/advance）")
            return

        # 2. 非 debug 模式：直接推进到下一个 slot
        self._advance_and_broadcast()

    def _confirm_slot_advance(self) -> bool:
        """
        Debug 模式下的手动确认：按 Enter 或 POST /slot/advance 调用。
        只有当前 slot 已收到 end 且交易处理完，才会真正推进。
        """
        if not self.debug_mode:
            return False

        with self._advance_lock:
            if not self._slot_ready_to_advance:
                logger.info("当前 slot 交易还未接收完毕（未收到 end），不能推进")
                return False

            logger.info("收到手动确认，推进 slot")
            self._advance_and_broadcast()
            return True

    def _advance_and_broadcast(self) -> None:
        """推进到下一个 slot，并广播；同时重置待确认状态"""
        current_slot = self.slot_manager.get_current_slot()
        self._slot_ready_to_advance = False

        if not self.slot_manager.has_next_slot():
            logger.info("所有 slot 处理完成，模拟结束")
            self._running = False
            self.socketio.emit(SIMULATION_DONE_EVENT, {'last_slot': current_slot})
            return

        self.slot_manager.advance_slot()

        # 新 slot 下，延迟策略里到期的交易可以发送
        self._process_transactions()

        self._broadcast_slot()

    def _keyboard_listener(self):
        """键盘监听线程（debug 模式下按 Enter 确认推进 slot）"""
        logger.info("键盘监听已启动，按 Enter 确认推进 slot")

        while self._running:
            try:
                input()  # 等待 Enter
                self._confirm_slot_advance()
            except (EOFError, KeyboardInterrupt):
                logger.info("没有可用的交互输入，键盘监听结束（仍可用 POST /slot/advance）")
                break
            except Exception:
                break

    def set_strategy(self, strategy: SendStrategy):
        """动态切换发送策略"""
        self.strategy = strategy
        logger.info(f"策略切换为: {strategy.get_strategy_name()}")

    def start(self, start_keyboard_listener: bool = True):
        """启动服务器：slot 推进由 end 标识驱动，debug 模式下还需手动确认"""
        self._running = True

        logger.info(f"启动服务器在 {SERVER_HOST}:{SERVER_PORT}")
        logger.info(f"使用策略: {self.strategy.get_strategy_name()}")
        if self.debug_mode:
            logger.info("🐛 DEBUG模式已启用: 收到 end 并处理完交易后，需按 Enter 或 POST /slot/advance 才推进 slot")
            if start_keyboard_listener:
                self._keyboard_thread = threading.Thread(
                    target=self._keyboard_listener, daemon=True
                )
                self._keyboard_thread.start()
        else:
            logger.info("Slot 推进机制: 广播新slot -> user发交易+end -> 处理 -> 推进")

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
