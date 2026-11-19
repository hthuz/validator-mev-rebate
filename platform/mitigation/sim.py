"""进程内模拟：不启动 HTTP/Socket.IO，直接调用 server 与 client 的业务函数。

数据源与 slot 范围都来自 config：
  - MITIGATION_TX_SOURCE=block  -> 从区块 JSON 重放全量原始交易
  - MITIGATION_TX_SOURCE=victim -> 从 ClickHouse 重放指定 epoch 的 victim 交易

slot 由本模块自己的循环推进，结束时把交易处理概况打印到终端；victim 模式额外
打印三档落地评估。

用法：
    python -m mitigation.sim
"""
import json

from config import (
    BLOCK_PATH,
    MITIGATION_REPLAY_EPOCH,
    MITIGATION_TX_SOURCE,
)
from log import server_logger as logger

from .evaluation import evaluate_victim_outcomes, load_sandwich_leaders
from .server import create_server
from .strategy import build_strategy_from_config
from .user.replay import BlockReplaySource
from .user.victim_source import VictimReplaySource


def _build_replay():
    if MITIGATION_TX_SOURCE == "block":
        return BlockReplaySource(BLOCK_PATH)
    if MITIGATION_TX_SOURCE == "victim":
        return VictimReplaySource(MITIGATION_REPLAY_EPOCH)
    raise ValueError(
        f"未知的 MITIGATION_TX_SOURCE: {MITIGATION_TX_SOURCE!r}"
        "（可选: block / victim）"
    )


def run_simulation(server=None, replay=None) -> dict:
    """跑一遍完整的 slot 循环，返回并打印交易处理概况。"""
    if server is None:
        server = create_server(
            strategy=build_strategy_from_config(),
            with_transport=False,
        )
    if replay is None:
        replay = _build_replay()

    slots = server.slot_manager.get_available_slots()

    print("=== 风险数据概况 ===")
    print(json.dumps(server.get_risk_summary(), ensure_ascii=False, indent=2))

    logger.info(
        f"开始模拟: slot {slots[0]}-{slots[-1]} 共 {len(slots)} 个, "
        f"策略 {server.strategy.get_strategy_name()}, "
        f"数据源 {MITIGATION_TX_SOURCE}"
    )

    submitted = set()
    for slot in slots:
        server.set_slot(slot)
        transactions = replay.transactions_for(slot)
        for tx in transactions:
            server.submit_transaction(tx, slot)
            submitted.add(tx.signature)
        server.close_slot(slot)

        logger.info(
            f"slot {slot}: leader={server.slot_manager.get_leader(slot)}, "
            f"提交交易={len(transactions)}"
        )

    report = server.get_report()
    print("=== 交易处理概况 ===")
    print(report['total_processed'])
    print(report['immediate_count'])
    print(report['delayed_count'])
    print(report['delay_stats'])
    # print(report['slot_overview'])
    # print(json.dumps(report, ensure_ascii=False, indent=2))


    if isinstance(replay, VictimReplaySource):
        leaders = load_sandwich_leaders(replay.epoch)
        evaluation = evaluate_victim_outcomes(
            report, server, replay, leaders, submitted
        )
        print("=== Victim 落地评估 ===")
        print(json.dumps(evaluation, ensure_ascii=False, indent=2))

    return report


if __name__ == "__main__":
    run_simulation()
