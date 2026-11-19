from .base_strategy import SendStrategy
from config import MITIGATION_MAX_SKIP_SLOTS
from type.models import RoutableTransaction


class RiskAwareStrategy(SendStrategy):
    """基于历史 sandwich ratio 的分发策略。

    当前 slot 的 leader 高风险时继续持有交易；遇到安全 leader 立即发送；
    若一直高风险并达到 max_skip_slots，则在当前 slot 强制发送。
    """

    def __init__(self, max_skip_slots: int = MITIGATION_MAX_SKIP_SLOTS):
        if max_skip_slots < 0:
            raise ValueError(f"max_skip_slots 不能为负数: {max_skip_slots}")
        self.max_skip_slots = max_skip_slots

    def should_send(self, transaction: RoutableTransaction, current_slot: int) -> bool:
        if current_slot < transaction.slot:
            return False

        context = self.context
        if context is None:
            # 独立使用（未经 server 注入上下文）时退化为立即发送
            return True

        leader = context.get_leader(current_slot)
        risk_store = context.risk_store
        if risk_store is None or not risk_store.is_risky(leader):
            return True

        # 高风险 leader：达到窗口上限就强制发送
        return (current_slot - transaction.slot) >= self.max_skip_slots

    def get_strategy_name(self) -> str:
        return f"RiskAwareStrategy(max_skip={self.max_skip_slots})"
