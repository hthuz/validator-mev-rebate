"""验证策略完全由 config 驱动，不经 CLI。"""
import sys
from pathlib import Path

project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

import mitigation.strategy.factory as factory
from mitigation.strategy import ImmediateStrategy, DelayedStrategy, RiskAwareStrategy


def test_factory_follows_config():
    names = (
        "MITIGATION_STRATEGY",
        "MITIGATION_DELAY_SLOTS",
        "MITIGATION_MAX_SKIP_SLOTS",
    )
    original = tuple(getattr(factory, name) for name in names)
    try:
        factory.MITIGATION_STRATEGY = "immediate"
        assert isinstance(factory.build_strategy_from_config(), ImmediateStrategy)

        factory.MITIGATION_STRATEGY = "delayed"
        factory.MITIGATION_DELAY_SLOTS = 4
        delayed = factory.build_strategy_from_config()
        assert isinstance(delayed, DelayedStrategy) and delayed.delay_slots == 4

        factory.MITIGATION_STRATEGY = "risk-aware"
        factory.MITIGATION_MAX_SKIP_SLOTS = 3
        risk_aware = factory.build_strategy_from_config()
        assert isinstance(risk_aware, RiskAwareStrategy)
        assert risk_aware.max_skip_slots == 3

        factory.MITIGATION_STRATEGY = "bogus"
        try:
            factory.build_strategy_from_config()
            assert False, "未知策略名应抛 ValueError"
        except ValueError:
            pass
    finally:
        for name, value in zip(names, original):
            setattr(factory, name, value)
    print("OK strategy factory follows config")


if __name__ == "__main__":
    test_factory_follows_config()
