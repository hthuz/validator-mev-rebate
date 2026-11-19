"""进程内模拟：不启动 HTTP/Socket.IO，直接调用 server 与 client 的业务函数。

slot 范围来自 config 的 MITIGATION_START_SLOT / MITIGATION_END_SLOT，
由本模块自己的循环推进；结束时把交易处理概况打印到终端。

用法：
    python -m mitigation.sim
"""
import json

from config import BLOCK_PATH
from log import server_logger as logger

from .server import create_server
from .strategy import build_strategy_from_config
from .user.replay import BlockReplaySource


def run_simulation(server=None, replay=None) -> dict:
    """跑一遍完整的 slot 循环，返回并打印交易处理概况。"""
    if server is None:
        server = create_server(
            strategy=build_strategy_from_config(),
            with_transport=False,
        )
    if replay is None:
        replay = BlockReplaySource(BLOCK_PATH)

    slots = server.slot_manager.get_available_slots()

    print("=== 风险数据概况 ===")
    print(json.dumps(server.get_risk_summary(), ensure_ascii=False, indent=2))

    logger.info(
        f"开始模拟: slot {slots[0]}-{slots[-1]} 共 {len(slots)} 个, "
        f"策略 {server.strategy.get_strategy_name()}"
    )

    for slot in slots:
        server.set_slot(slot)
        transactions = replay.transactions_for(slot)
        for tx in transactions:
            server.submit_transaction(tx, slot)
        server.close_slot(slot)

        logger.info(
            f"slot {slot}: leader={server.slot_manager.get_leader(slot)}, "
            f"提交交易={len(transactions)}"
        )

    report = server.get_report()
    print("=== 交易处理概况 ===")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return report


if __name__ == "__main__":
    run_simulation()
