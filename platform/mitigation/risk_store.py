"""历史 epoch 的 validator sandwich ratio 存储与风险判定。

数据来源是 utils.sandwich.get_sandwich_ratios()，ratio 的计算口径与
SlotInfo.sandwich_ratio() 保持一致。判定规则：在配置的历史 epoch 范围内，
任意一个 epoch 的 ratio 严格超过阈值，就认为该 validator 是高风险 leader。
"""

import math
from typing import Callable, Dict, List, Mapping, Optional, Set

from config import (
    SANDWICH_HISTORY_END_EPOCH,
    SANDWICH_HISTORY_START_EPOCH,
    SANDWICH_RATIO_THRESHOLD,
)
from log import strategy_logger as logger


RatioLoader = Callable[[int], Mapping[str, float]]


class ValidatorRiskStore:
    """维护历史各 epoch 的 validator sandwich ratio，并提供风险判定。

    - epoch 范围由构造参数（默认来自 config）限定，两端 inclusive
    - NaN 或缺失的 ratio 视为「无数据」，不判为高风险
    - 单个 epoch 加载失败只记 warning，不阻断模拟
    """

    def __init__(
        self,
        start_epoch: Optional[int] = None,
        end_epoch: Optional[int] = None,
        threshold: Optional[float] = None,
        loader: Optional[RatioLoader] = None,
    ):
        self.start_epoch = (
            SANDWICH_HISTORY_START_EPOCH if start_epoch is None else start_epoch
        )
        self.end_epoch = SANDWICH_HISTORY_END_EPOCH if end_epoch is None else end_epoch
        self.threshold = SANDWICH_RATIO_THRESHOLD if threshold is None else threshold

        if self.start_epoch > self.end_epoch:
            raise ValueError(
                f"历史 epoch 范围非法：{self.start_epoch} > {self.end_epoch}"
            )

        self._ratios: Dict[int, Dict[str, float]] = {}
        self._risky: Set[str] = set()
        self._load(loader or self._default_loader)

    @staticmethod
    def _default_loader(epoch: int) -> Mapping[str, float]:
        from utils.sandwich import get_sandwich_ratios

        return get_sandwich_ratios(epoch)

    def _load(self, loader: RatioLoader) -> None:
        for epoch in range(self.start_epoch, self.end_epoch + 1):
            try:
                ratios = dict(loader(epoch))
            except Exception as exc:
                logger.warning(f"无法加载 epoch {epoch} 的 sandwich ratio: {exc}")
                ratios = {}

            self._ratios[epoch] = ratios
            risky_here = {
                identity
                for identity, ratio in ratios.items()
                if self._exceeds(ratio)
            }
            self._risky.update(risky_here)
            logger.info(
                f"加载 epoch {epoch} sandwich ratio: {len(ratios)} 个 validator，"
                f"其中 {len(risky_here)} 个超过阈值 {self.threshold}"
            )

    def _exceeds(self, ratio: float) -> bool:
        """ratio 是否严格超过阈值；NaN / 非法值视为无数据。"""
        try:
            value = float(ratio)
        except (TypeError, ValueError):
            return False
        if math.isnan(value):
            return False
        return value > self.threshold

    def is_risky(self, identity: Optional[str]) -> bool:
        """该 validator 在任意历史 epoch 是否高风险。"""
        return bool(identity) and identity in self._risky

    def get_ratios(self, identity: str) -> Dict[int, float]:
        """返回该 validator 在各历史 epoch 的 ratio（含 nan）。"""
        return {
            epoch: ratios[identity]
            for epoch, ratios in sorted(self._ratios.items())
            if identity in ratios
        }

    def risky_validators(self) -> List[str]:
        return sorted(self._risky)

    def summary(self) -> dict:
        """给 /risk/summary 和排障用的概览。"""
        return {
            "start_epoch": self.start_epoch,
            "end_epoch": self.end_epoch,
            "threshold": self.threshold,
            "loaded_epochs": sorted(self._ratios.keys()),
            "validator_counts": {
                epoch: len(ratios) for epoch, ratios in sorted(self._ratios.items())
            },
            "risky_validators": self.risky_validators(),
        }
