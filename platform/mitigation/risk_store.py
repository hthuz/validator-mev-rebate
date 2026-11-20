"""历史 epoch 的 validator sandwich ratio + Marinade 质押风险存储与判定。

数据来源：
  - sandwich ratio: utils.sandwich.get_sandwich_ratios()，计算口径与
    SlotInfo.sandwich_ratio() 一致（SandwichSlotNum / SlotNum）
  - Marinade 质押占比: utils.stake_reader，m_activated_stake / activatedStake

判定规则：在配置的历史 epoch 范围内，任意一个 epoch
  - sandwich ratio 严格超过 SANDWICH_RATIO_THRESHOLD，或
  - Marinade 质押占比严格超过 MARINADE_STAKE_THRESHOLD
即认为该 validator 是高风险 leader。两者取并集，并分别保留来源以便展示。
"""

import math
from typing import Callable, Dict, List, Mapping, Optional, Set

from config import (
    MARINADE_STAKE_THRESHOLD,
    SANDWICH_HISTORY_END_EPOCH,
    SANDWICH_HISTORY_START_EPOCH,
    SANDWICH_RATIO_THRESHOLD,
)
from log import strategy_logger as logger

from .marinade_store import MarinadeRatioLoader


RatioLoader = Callable[[int], Mapping[str, float]]


class ValidatorRiskStore:
    """维护历史各 epoch 的 validator 风险，并提供风险判定。

    - epoch 范围由构造参数（默认来自 config）限定，两端 inclusive
    - NaN 或缺失视为「无数据」，不判为高风险
    - 单个 epoch 加载失败只记 warning，不阻断模拟
    - sandwich 与 Marinade 两条判定线取并集，is_risky 对并集生效
    """

    def __init__(
        self,
        start_epoch: Optional[int] = None,
        end_epoch: Optional[int] = None,
        threshold: Optional[float] = None,
        loader: Optional[RatioLoader] = None,
        marinade_threshold: Optional[float] = None,
        marinade_loader: Optional[MarinadeRatioLoader] = None,
    ):
        self.start_epoch = (
            SANDWICH_HISTORY_START_EPOCH if start_epoch is None else start_epoch
        )
        self.end_epoch = (
            SANDWICH_HISTORY_END_EPOCH if end_epoch is None else end_epoch
        )
        self.threshold = (
            SANDWICH_RATIO_THRESHOLD if threshold is None else threshold
        )
        self.marinade_threshold = (
            MARINADE_STAKE_THRESHOLD
            if marinade_threshold is None else marinade_threshold
        )

        if self.start_epoch > self.end_epoch:
            raise ValueError(
                f"历史 epoch 范围非法：{self.start_epoch} > {self.end_epoch}"
            )

        self._ratios: Dict[int, Dict[str, float]] = {}
        self._marinade_ratios: Dict[int, Dict[str, float]] = {}
        self._sandwich_risky: Set[str] = set()
        self._marinade_risky: Set[str] = set()
        self._risky: Set[str] = set()

        self._load(loader or self._default_loader)
        self._load_marinade(marinade_loader or self._default_marinade_loader)

    # ------------------------------------------------------------------
    # 数据加载
    # ------------------------------------------------------------------
    @staticmethod
    def _default_loader(epoch: int) -> Mapping[str, float]:
        from utils.sandwich import get_sandwich_ratios

        return get_sandwich_ratios(epoch)

    @staticmethod
    def _default_marinade_loader(epoch: int) -> Mapping[str, float]:
        from .marinade_store import load_marinade_stake_ratios

        return load_marinade_stake_ratios(epoch)

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
                if self._exceeds(ratio, self.threshold)
            }
            self._sandwich_risky.update(risky_here)
            logger.info(
                f"加载 epoch {epoch} sandwich ratio: {len(ratios)} 个 validator，"
                f"其中 {len(risky_here)} 个超过阈值 {self.threshold}"
            )
        self._recompute_risky()

    def _load_marinade(self, loader: MarinadeRatioLoader) -> None:
        for epoch in range(self.start_epoch, self.end_epoch + 1):
            try:
                ratios = dict(loader(epoch))
            except Exception as exc:
                logger.warning(f"无法加载 epoch {epoch} 的 Marinade 质押占比: {exc}")
                ratios = {}

            self._marinade_ratios[epoch] = ratios
            risky_here = {
                identity
                for identity, ratio in ratios.items()
                if self._exceeds(ratio, self.marinade_threshold)
            }
            self._marinade_risky.update(risky_here)
            logger.info(
                f"加载 epoch {epoch} Marinade 质押: {len(ratios)} 个 validator，"
                f"其中 {len(risky_here)} 个占比超过阈值 {self.marinade_threshold}"
            )
        self._recompute_risky()

    def _recompute_risky(self) -> None:
        """两条判定线取并集。"""
        self._risky = self._sandwich_risky | self._marinade_risky

    @staticmethod
    def _exceeds(ratio: float, threshold: float) -> bool:
        """ratio 是否严格超过阈值；NaN / 非法值视为无数据。"""
        try:
            value = float(ratio)
        except (TypeError, ValueError):
            return False
        if math.isnan(value):
            return False
        return value > threshold

    # ------------------------------------------------------------------
    # 查询
    # ------------------------------------------------------------------
    def is_risky(self, identity: Optional[str]) -> bool:
        """该 validator 在任意历史 epoch 是否高风险（sandwich ∪ Marinade）。"""
        return bool(identity) and identity in self._risky

    def is_sandwich_risky(self, identity: Optional[str]) -> bool:
        return bool(identity) and identity in self._sandwich_risky

    def is_marinade_risky(self, identity: Optional[str]) -> bool:
        return bool(identity) and identity in self._marinade_risky

    def get_ratios(self, identity: str) -> Dict[int, float]:
        """返回该 validator 在各历史 epoch 的 sandwich ratio（含 nan）。"""
        return {
            epoch: ratios[identity]
            for epoch, ratios in sorted(self._ratios.items())
            if identity in ratios
        }

    def get_marinade_ratios(self, identity: str) -> Dict[int, float]:
        """返回该 validator 在各历史 epoch 的 Marinade 质押占比。"""
        return {
            epoch: ratios[identity]
            for epoch, ratios in sorted(self._marinade_ratios.items())
            if identity in ratios
        }

    def risky_validators(self) -> List[str]:
        """并集：sandwich 或 Marinade 任一判为高风险的 validator。"""
        return sorted(self._risky)

    def sandwich_risky_validators(self) -> List[str]:
        return sorted(self._sandwich_risky)

    def marinade_risky_validators(self) -> List[str]:
        return sorted(self._marinade_risky)

    def both_risky_validators(self) -> List[str]:
        """两条判定线都命中的 validator。"""
        return sorted(self._sandwich_risky & self._marinade_risky)

    def summary(self) -> dict:
        """给 /risk/summary、sim 输出和排障用的概览。"""
        return {
            "start_epoch": self.start_epoch,
            "end_epoch": self.end_epoch,
            "threshold": self.threshold,
            "marinade_threshold": self.marinade_threshold,
            "loaded_epochs": sorted(self._ratios.keys()),
            "validator_counts": {
                epoch: len(ratios) for epoch, ratios in sorted(self._ratios.items())
            },
            "marinade_validator_counts": {
                epoch: len(ratios)
                for epoch, ratios in sorted(self._marinade_ratios.items())
            },
            "risky_validators": self.risky_validators(),
            "sandwich_risky_validators": self.sandwich_risky_validators(),
            "marinade_risky_validators": self.marinade_risky_validators(),
            "both_risky_validators": self.both_risky_validators(),
        }
