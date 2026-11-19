"""HTTP / Socket.IO 适配层。

只负责「解析传输层数据 -> 调用 TransactionServer 的业务函数 -> 包装响应」，
不实现任何业务规则。sim 不加载本模块也能跑完整流程。
"""
from flask import request, jsonify

from type.models import BlockTransaction
from log import server_logger as logger


NEW_SLOT_EVENT = 'new_slot'
SIMULATION_DONE_EVENT = 'simulation_done'


def register_routes(app, server) -> None:
    """把 HTTP 路由注册到 Flask app 上"""

    @app.route('/health', methods=['GET'])
    def health():
        return jsonify(server.get_health())

    @app.route('/sendTransaction', methods=['POST'])
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

            if data.get('end') is True:
                server.complete_slot(slot)
                return jsonify({'status': 'end_accepted', 'slot': slot}), 200

            transaction = BlockTransaction.from_dict(data)
            server.submit_transaction(transaction, slot)

            return jsonify({
                'status': 'accepted',
                'signature': transaction.signature,
                'slot': transaction.slot,
            }), 200

        except Exception as e:
            return jsonify({'error': str(e)}), 500

    @app.route('/transactions/pending', methods=['GET'])
    def get_pending():
        return jsonify(server.get_pending())

    @app.route('/transactions/sent', methods=['GET'])
    def get_sent():
        """发送概况：未延迟只计数，延迟交易保留详情并给出统计"""
        return jsonify(server.get_report())

    @app.route('/risk/summary', methods=['GET'])
    def get_risk_summary():
        """查看历史 epoch 范围、阈值、高风险 validator 列表"""
        return jsonify(server.get_risk_summary())

    @app.route('/slot/current', methods=['GET'])
    def get_current_slot():
        return jsonify({'current_slot': server.get_current_slot()})


def register_socket_events(socketio, server) -> None:
    """注册 Socket.IO 事件"""

    @socketio.on('connect')
    def on_connect(auth=None):
        sid = request.sid
        logger.info(f"User 已连接 (sid={sid})，发送当前 slot")
        server._broadcast_slot(to=sid)

    @socketio.on('disconnect')
    def on_disconnect():
        logger.info("User 已断开连接")


def broadcast_slot(server, to: str = None) -> None:
    """广播当前 slot 给 user"""
    slot = server.slot_manager.get_current_slot()
    payload = {
        'slot': slot,
        'leader': server.slot_manager.get_leader(slot),
    }
    if to is None:
        server.socketio.emit(NEW_SLOT_EVENT, payload)
        logger.info(f"广播新 slot: {slot}")
    else:
        server.socketio.emit(NEW_SLOT_EVENT, payload, to=to)
        logger.info(f"向 {to} 发送当前 slot: {slot}")
