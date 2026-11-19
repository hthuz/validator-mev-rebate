"""victim 落地结果的三档评估标签。

策略用历史 epoch（默认 800-801）的 ratio 做决策，效果评估用当前 replay epoch
（默认 802）自身的 sandwich 观测做标签，时间上是干净的留出验证。

标签口径：
  confirmed_sandwiched      落地 slot == 原始 slot，数据里确认被 sandwich
  leader_observed_sandwich  换了 slot，但落地 leader 在本 epoch 观测到过 victim sandwich
  clean_leader              换了 slot，落地 leader 没有观测到 sandwich
"""
from collections import Counter
from typing import Callable, Dict, Optional, Set

from log import server_logger as logger
from utils.ch import query_clickhouse


LeaderSetLoader = Callable[[int], Set[str]]


def load_sandwich_leaders(epoch: int, loader: Optional[LeaderSetLoader] = None) -> Set[str]:
    """当前 epoch 里观测到 victim sandwich 的 leader 集合。"""
    if loader is not None:
        return set(loader(epoch))

    epoch_start = 432000 * epoch
    epoch_end = epoch_start + 432000 - 1
    rows = query_clickhouse(
        "SELECT DISTINCT l.leader AS leader "
        "FROM sandwiches_nobundle s JOIN slot_leaders l ON s.slot = l.slot "
        f"WHERE s.slot BETWEEN {epoch_start} AND {epoch_end} AND s.type = 'victim'"
    )
    leaders = {row['leader'] for row in rows}
    logger.info(f"epoch {epoch} 观测到 victim sandwich 的 leader: {len(leaders)} 个")
    return leaders


def evaluate_victim_outcomes(
    report: dict,
    server,
    replay,
    sandwich_leaders: Set[str],
    submitted: Optional[Set[str]] = None,
) -> dict:
    """把每个已处理 victim 的落地结果打成三档标签并汇总。"""
    delayed = {record['signature']: record for record in report['delayed_records']}
    originals = {tx.signature: tx.slot for tx in replay.all_transactions()}
    if submitted is not None:
        originals = {sig: slot for sig, slot in originals.items() if sig in submitted}

    counts: Counter = Counter()
    for signature, original_slot in originals.items():
        record = delayed.get(signature)
        landing_slot = record['target_slot'] if record else original_slot

        if landing_slot == original_slot:
            label = 'confirmed_sandwiched'
        elif server.slot_manager.get_leader(landing_slot) in sandwich_leaders:
            label = 'leader_observed_sandwich'
        else:
            label = 'clean_leader'
        counts[label] += 1

    total = sum(counts.values())
    keys = (
        'confirmed_sandwiched',
        'leader_observed_sandwich',
        'clean_leader',
    )
    return {
        'evaluated_victims': total,
        'counts': {key: counts.get(key, 0) for key in keys},
        'ratios': {
            key: round(counts.get(key, 0) / total, 6) if total else 0.0
            for key in keys
        },
        'sandwich_leaders_in_epoch': len(sandwich_leaders),
    }
