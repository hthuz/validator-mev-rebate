"""Marinade 质押占比加载。

口径：某个 validator 的 Marinade 质押占其总激活质押的比例
    = Validator.m_activated_stake / Validator.activatedStake

数据来源：utils.stake_reader.Reader(epoch).get_validators()
    - 返回的 dict key 是 voteAccountPubkey，这里映射到 identityPubkey
      （slot leader 与 sandwich ratio 用的都是 identity）
    - 只统计 marinade == True 的 validator，其余不进入结果
"""
from typing import Callable, Dict, Mapping

from log import strategy_logger as logger


MarinadeRatioLoader = Callable[[int], Mapping[str, float]]


def load_marinade_stake_ratios(epoch: int) -> Dict[str, float]:
    """返回 {identityPubkey: marinade 质押占比}（只含有 Marinade 质押的 validator）。"""
    # 延迟导入：stake_reader 依赖较多，避免模块 import 时的循环与开销
    from utils.stake_reader import Reader

    # validators = Reader(epoch).get_validators()
    r = Reader(epoch)
    validators = r.valid_filter(r.get_validators(with_stake=True).values())
    ratios: Dict[str, float] = {}
    for validator in validators:
        if not validator.activatedStake:
            continue

        # if validator.marinade and validator.activatedStake > 0:
        ratios[validator.identityPubkey] = (
            validator.protocol_stake("marinade") / validator.activatedStake
        )
    logger.info(
        f"epoch {epoch} 含 Marinade 质押的 validator: {len(ratios)} 个"
    )
    return ratios


if __name__ == "__main__":
    epoch = 800
    ratios = load_marinade_stake_ratios(epoch)
    sorted_ratios = sorted(ratios.items(), key=lambda x: x[1], reverse=True)
    for identity, ratio in sorted_ratios:
        print(f"{identity}: {ratio:.4f}")
