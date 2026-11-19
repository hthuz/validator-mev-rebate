from dataclasses import replace
from typing import Optional

from flask import Flask
from flask_socketio import SocketIO

from .transaction_manager import TransactionManager
from .slot_manager import SlotManager
from .risk_store import ValidatorRiskStore
from .strategy import ImmediateStrategy, SendStrategy, StrategyContext, RiskAwareStrategy
from .routes import (
    NEW_SLOT_EVENT,
    SIMULATION_DONE_EVENT,
    register_routes,
    register_socket_events,
    broadcast_slot,
)
from type.models import BlockTransaction
from config import SERVER_HOST, SERVER_PORT, DEBUG_MODE
from log import server_logger as logger


class TransactionServer:
    """
    交易处理核心（业务函数层）。

    这里只放业务函数，不关心请求是 HTTP / Socket.IO 过来的，还是 sim 直接函数调用过来的：
      - set_slot / submit_transaction / close_slot / advance_slot 驱动单个 slot 的生命周期
      - process 按当前策略决定哪些交易可以发给当前 leader
      - get_report 等查询函数返回纯 dict，由适配层决定怎么序列化

    slot 与 block 是同一个概念（整数 slot 号），范围由 config 的
    MITIGATION_START_SLOT / MITIGATION_END_SLOT 决定（inclusive），每次推进 +1。

    with_transport=True（默认）时额外创建 Flask / Socket.IO 并注册路由，供原有
    server + client 模式使用；with_transport=False 时只保留业务 core，供 sim 使用。
    """

    def __init__(
        self,
        strategy: SendStrategy = None,
        risk_store: ValidatorRiskStore = None,
        with_transport: bool = True,
    ):
        self.transaction_manager = TransactionManager()
        self.slot_manager = SlotManager()
        self.strategy = strategy or ImmediateStrategy()
        self.risk_store = risk_store
        self._running = False

        # 仅作信息展示，不影响 slot 推进逻辑
        self.debug_mode = DEBUG_MODE

        # 一开始就指向 MITIGATION_START_SLOT，保证 connect 时广播的是有效值
        self.slot_manager.reset_to_first_slot()
        self._apply_strategy_context(self.strategy)

        self.with_transport = with_transport
        if with_transport:
            self.app = Flask(__name__)
            self.socketio = SocketIO(
                self.app, cors_allowed_origins="*", async_mode="threading"
            )
            register_routes(self.app, self)
            register_socket_events(self.socketio, self)
        else:
            self.app = None
            self.socketio = None

    def _apply_strategy_context(self, strategy: SendStrategy) -> None:
        """给策略注入 leader 查询与风险存储；只有 risk-aware 策略才加载历史数据。"""
        if isinstance(strategy, RiskAwareStrategy) and self.risk_store is None:
            self.risk_store = ValidatorRiskStore()

        strategy.set_context(StrategyContext(
            get_leader=self.slot_manager.get_leader,
            risk_store=self.risk_store,
        ))

    # ------------------------------------------------------------------
    # 业务函数：slot 生命周期
    # ------------------------------------------------------------------
    def set_slot(self, slot: int) -> int:
        """把当前 slot 设为指定值（sim 由自己的循环驱动，无需广播）"""
        self.slot_manager.set_current_slot(slot)
        return slot

    def get_current_slot(self) -> int:
        return self.slot_manager.get_current_slot()

    def submit_transaction(
        self, transaction: BlockTransaction, slot: Optional[int] = None
    ) -> bool:
        """接收一笔交易并按当前策略尝试发送。"""
        if slot is not None and slot != transaction.slot:
            transaction = replace(transaction, slot=slot)
        self.transaction_manager.add_transaction(transaction)
        self.process()
        return True

    def close_slot(self, slot: int) -> None:
        """标记该 slot 交易接收完毕并处理；不推进 slot，也不广播。"""
        self.transaction_manager.mark_slot_end(slot)
        self.process()

    def complete_slot(self, slot: int) -> None:
        """HTTP 兼容路径：处理完当前 slot 后推进并广播。"""
        current_slot = self.get_current_slot()
        if slot != current_slot:
            logger.warning(
                f"收到 slot {slot} 的 end 标识，但当前 slot 是 {current_slot}，忽略"
            )
            return

        logger.info(f"========== Slot {current_slot} 交易接收完成 ==========")
        self.close_slot(slot)
        self._advance_and_broadcast()

    def _on_slot_complete(self, slot: int) -> None:
        """旧调用点兼容"""
        self.complete_slot(slot)

    def advance_slot(self) -> int:
        """纯业务推进：slot +1 后处理到期的延迟交易，不广播。"""
        next_slot = self.slot_manager.advance_slot()
        self.process()
        return next_slot

    def _advance_and_broadcast(self) -> None:
        """HTTP 路径：推进 slot 并广播；到 END 则只发 simulation_done。"""
        current_slot = self.get_current_slot()

        if not self.slot_manager.has_next_slot():
            logger.info(
                f"MITIGATION_END_SLOT ({current_slot}) 处理完成，模拟结束，不再广播 new_slot"
            )
            self._running = False
            if self.socketio is not None:
                self.socketio.emit(SIMULATION_DONE_EVENT, {'last_slot': current_slot})
            return

        self.advance_slot()
        self._broadcast_slot()

    def process(self) -> None:
        """按当前策略把到期的交易发送给当前 slot 的 leader。"""
        current_slot = self.get_current_slot()
        leader = self.slot_manager.get_leader(current_slot)
        if not leader or leader == "unknown":
            logger.debug(f"Slot {current_slot} 没有对应的 leader")
            return

        for slot in sorted(self.transaction_manager.get_pending_slots()):
            transactions = self.transaction_manager.get_transactions_for_slot(slot)
            sent_signatures = set()

            for tx in transactions:
                if self.strategy.should_send(tx, current_slot):
                    record = self.strategy.send_to_validator(tx, leader, current_slot)
                    self.transaction_manager.add_sent_record(record)
                    sent_signatures.add(tx.signature)

            # 只移除已发送的，延迟策略未到期的交易继续保留
            if sent_signatures:
                self.transaction_manager.remove_transactions(slot, sent_signatures)

    def _process_transactions(self) -> None:
        """旧调用点兼容"""
        self.process()

    def _broadcast_slot(self, to: str = None) -> None:
        """传输层委托；sim 模式下没有 socketio，直接跳过。"""
        if self.socketio is None:
            return
        broadcast_slot(self, to=to)

    # ------------------------------------------------------------------
    # 业务函数：查询（返回纯 dict）
    # ------------------------------------------------------------------
    def get_health(self) -> dict:
        return {
            'status': 'ok',
            'current_slot': self.get_current_slot(),
            'strategy': self.strategy.get_strategy_name(),
            'debug_mode': self.debug_mode,
        }

    def get_pending(self) -> dict:
        pending_slots = self.transaction_manager.get_pending_slots()
        return {
            'pending_slots': sorted(list(pending_slots)),
            'count': len(pending_slots),
        }

    def get_report(self) -> dict:
        return self.transaction_manager.get_report()

    def get_risk_summary(self) -> dict:
        if self.risk_store is None:
            return {'loaded': False}
        return {'loaded': True, **self.risk_store.summary()}

    def set_strategy(self, strategy: SendStrategy):
        """动态切换发送策略"""
        self.strategy = strategy
        self._apply_strategy_context(strategy)
        logger.info(f"策略切换为: {strategy.get_strategy_name()}")

    def start(self):
        """启动 HTTP/Socket.IO 服务器；slot 推进完全由 end 标识驱动。"""
        if not self.with_transport:
            raise RuntimeError("当前 server 以 with_transport=False 创建，不能调用 start()")

        self._running = True

        logger.info(f"启动服务器在 {SERVER_HOST}:{SERVER_PORT}")
        logger.info(f"使用策略: {self.strategy.get_strategy_name()}")
        logger.info(
            f"Slot 范围: {self.slot_manager.start_slot}-{self.slot_manager.end_slot} (inclusive)"
        )
        logger.info("Slot 推进机制: 广播新slot -> user发交易+end -> 处理 -> 推进")
        if self.debug_mode:
            logger.info("DEBUG模式已启用（仅日志详细程度，不影响 slot 推进）")

        self.socketio.run(
            self.app,
            host=SERVER_HOST,
            port=SERVER_PORT,
            allow_unsafe_werkzeug=True,
        )

    def stop(self):
        """停止服务器"""
        self._running = False
        if self.socketio is not None:
            self.socketio.stop()


def create_server(
    strategy: SendStrategy = None,
    risk_store: ValidatorRiskStore = None,
    with_transport: bool = True,
) -> TransactionServer:
    """工厂函数：创建服务器实例。sim 传 with_transport=False 只拿业务 core。"""
    return TransactionServer(
        strategy=strategy,
        risk_store=risk_store,
        with_transport=with_transport,
    )


if __name__ == "__main__":
    from .strategy import build_strategy_from_config

    server = create_server(strategy=build_strategy_from_config())
    server.start()
