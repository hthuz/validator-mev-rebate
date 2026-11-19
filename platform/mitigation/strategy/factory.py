"""按 config 构造发送策略，避免通过 CLI 传参。"""

from config import (
    MITIGATION_DELAY_SLOTS,
    MITIGATION_MAX_SKIP_SLOTS,
    MITIGATION_STRATEGY,
)

from .base_strategy import SendStrategy
from .delayed_strategy import DelayedStrategy
from .immediate_strategy import ImmediateStrategy
from .risk_aware_strategy import RiskAwareStrategy


def build_strategy_from_config() -> SendStrategy:
    """根据 MITIGATION_STRATEGY 返回对应策略实例。"""
    if MITIGATION_STRATEGY == "immediate":
        return ImmediateStrategy()
    if MITIGATION_STRATEGY == "delayed":
        return DelayedStrategy(delay_slots=MITIGATION_DELAY_SLOTS)
    if MITIGATION_STRATEGY == "risk-aware":
        return RiskAwareStrategy(max_skip_slots=MITIGATION_MAX_SKIP_SLOTS)
    raise ValueError(
        f"未知的 MITIGATION_STRATEGY: {MITIGATION_STRATEGY!r}"
        "（可选: immediate / delayed / risk-aware）"
    )
