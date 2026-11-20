"""victim 落地结果的三档评估标签 + 验证者视角的风险暴露评估。

策略用历史 epoch（默认 800-801）的 ratio 做决策，效果评估用当前 replay epoch
（默认 802）自身的 sandwich 观测做标签，时间上是干净的留出验证。

评估侧的 risky validator 与策略侧口径一致：sandwich ratio 与 Marinade 质押
占比两条线取并集，只是数据取自 replay epoch 自身。

三档标签口径：
  confirmed_sandwiched      落地 slot == 原始 slot，数据里确认被 sandwich
  leader_observed_sandwich  换了 slot，但落地 leader 在本 epoch 观测到过 victim sandwich
  clean_leader              换了 slot，落地 leader 没有观测到 sandwich
"""
from collections import Counter
from typing import Callable, Dict, Mapping, Optional, Set

from config import MARINADE_STAKE_THRESHOLD, SANDWICH_RATIO_THRESHOLD
from log import server_logger as logger
from utils.ch import query_clickhouse


LeaderSetLoader = Callable[[int], Set[str]]
RatioLoader = Callable[[int], Mapping[str, float]]


def load_risky_validators_detail(
    epoch: int,
    loader: Optional[RatioLoader] = None,
    threshold: Optional[float] = None,
    marinade_loader: Optional[RatioLoader] = None,
    marinade_threshold: Optional[float] = None,
) -> Dict[str, object]:
    """某个 epoch 自身的 risky validator，按来源分列（sandwich ∪ Marinade）。

    与策略用的历史 epoch 范围（默认 800-801）区分：这里默认读 replay epoch
    自己的数据，作为留出评估口径。NaN / 缺失视为安全；加载失败只 warning，
    退化为空集合，不中断评估。
    """
    if threshold is None:
        threshold = SANDWICH_RATIO_THRESHOLD
    if marinade_threshold is None:
        marinade_threshold = MARINADE_STAKE_THRESHOLD
    if loader is None:
        from utils.sandwich import get_sandwich_ratios

        loader = get_sandwich_ratios
    if marinade_loader is None:
        from .marinade_store import load_marinade_stake_ratios

        marinade_loader = load_marinade_stake_ratios

    ratios = _safe_load(loader, epoch, "sandwich ratio")
    sandwich_risky = {
        identity for identity, ratio in ratios.items()
        if _exceeds_threshold(ratio, threshold)
    }

    marinade_ratios = _safe_load(marinade_loader, epoch, "Marinade 质押占比")
    marinade_risky = {
        identity for identity, ratio in marinade_ratios.items()
        if _exceeds_threshold(ratio, marinade_threshold)
    }

    risky = sandwich_risky | marinade_risky
    logger.info(
        f"epoch {epoch} risky validator: sandwich {len(sandwich_risky)} / "
        f"{len(ratios)} 个有数据，Marinade {len(marinade_risky)} / "
        f"{len(marinade_ratios)} 个有数据，并集 {len(risky)} 个"
    )
    return {
        "epoch": epoch,
        "sandwich_threshold": threshold,
        "marinade_threshold": marinade_threshold,
        "sandwich_validator_count": len(ratios),
        "marinade_validator_count": len(marinade_ratios),
        "sandwich_risky_validators": sandwich_risky,
        "marinade_risky_validators": marinade_risky,
        "both_risky_validators": sandwich_risky & marinade_risky,
        "risky_validators": risky,
    }


def load_risky_validators(
    epoch: int,
    loader: Optional[RatioLoader] = None,
    threshold: Optional[float] = None,
    marinade_loader: Optional[RatioLoader] = None,
    marinade_threshold: Optional[float] = None,
) -> Set[str]:
    """某个 epoch 自身的 risky validator 并集（sandwich ∪ Marinade）。

    保留这个薄封装，兼容只关心并集、不关心来源分列的调用方式。
    """
    detail = load_risky_validators_detail(
        epoch, loader, threshold, marinade_loader, marinade_threshold
    )
    return detail["risky_validators"]  # type: ignore[return-value]


def _safe_load(loader: RatioLoader, epoch: int, label: str) -> Mapping[str, float]:
    try:
        return dict(loader(epoch))
    except Exception as exc:
        logger.warning(f"无法加载 epoch {epoch} 的 {label}: {exc}")
        return {}


def _exceeds_threshold(ratio: float, threshold: float) -> bool:
    """ratio 是否严格超过阈值；NaN / 非法值视为安全。"""
    try:
        value = float(ratio)
    except (TypeError, ValueError):
        return False
    if value != value:  # NaN，避免引入 math 依赖
        return False
    return value > threshold


def evaluate_leader_risk_exposure(
    report: dict,
    risky_validators: Set[str],
    marinade_risky: Optional[Set[str]] = None,
    sandwich_risky: Optional[Set[str]] = None,
) -> dict:
    """验证者视角：所有已发送交易里，落地 leader 是否为 risky。

    统计对象是**全部**发送交易（TransactionManager 按 validator 聚合的计数），
    不只是被延迟的那一批。风险集合由调用方给出（如 epoch 802 的 risky
    validator 并集），与策略决策所用历史数据无关，因此能回答"路由后还有多少
    落在评估 epoch 认定的 risky leader 手里"。

    传入 marinade_risky / sandwich_risky 时额外输出来源细分。两者可重叠，
    因此 by_source 内的计数不是互斥划分，之和可以不等于 on_risky_leader。
    """
    landed = report['landed_validators']
    total = sum(landed.values())
    on_risky = sum(count for validator, count in landed.items() if validator in risky_validators)
    on_safe = total - on_risky

    result = {
        'total_sent': total,
        'on_risky_leader': on_risky,
        'on_safe_leader': on_safe,
        'ratios': {
            'on_risky_leader': round(on_risky / total, 6) if total else 0.0,
            'on_safe_leader': round(on_safe / total, 6) if total else 0.0,
        },
        'risky_validators_in_epoch': len(risky_validators),
    }

    if marinade_risky is not None or sandwich_risky is not None:
        marinade_risky = marinade_risky or set()
        sandwich_risky = sandwich_risky or set()
        by_source = {
            'marinade': marinade_risky,
            'sandwich': sandwich_risky,
            'both': marinade_risky & sandwich_risky,
        }
        result['by_source'] = {
            name: {
                'count': sum(
                    count for validator, count in landed.items()
                    if validator in validators
                ),
                'ratio': round(
                    sum(
                        count for validator, count in landed.items()
                        if validator in validators
                    ) / total, 6
                ) if total else 0.0,
                'risky_validators': len(validators),
            }
            for name, validators in by_source.items()
        }

    return result


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
