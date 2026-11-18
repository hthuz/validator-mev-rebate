"""测试mitigation基本功能"""
import sys
from pathlib import Path

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from type.models import Transaction
from mitigation.strategy import ImmediateStrategy, DelayedStrategy

def test_immediate_strategy():
    """测试立即发送策略"""
    strategy = ImmediateStrategy()

    tx = Transaction(
        tx_id="test_tx_1",
        slot=100,
        signature="test_sig",
        data={}
    )

    # 当前slot < 交易slot，不应该发送
    assert not strategy.should_send(tx, 99), "slot 99 不应该发送"

    # 当前slot == 交易slot，应该发送
    assert strategy.should_send(tx, 100), "slot 100 应该发送"

    # 当前slot > 交易slot，应该发送
    assert strategy.should_send(tx, 101), "slot 101 应该发送"

    print("✅ ImmediateStrategy 测试通过")

def test_delayed_strategy():
    """测试延迟发送策略"""
    strategy = DelayedStrategy(delay_slots=2)

    tx = Transaction(
        tx_id="test_tx_2",
        slot=100,
        signature="test_sig",
        data={}
    )

    # 当前slot < 交易slot + delay，不应该发送
    assert not strategy.should_send(tx, 100), "slot 100 不应该发送"
    assert not strategy.should_send(tx, 101), "slot 101 不应该发送"

    # 当前slot == 交易slot + delay，应该发送
    assert strategy.should_send(tx, 102), "slot 102 应该发送"

    # 当前slot > 交易slot + delay，应该发送
    assert strategy.should_send(tx, 103), "slot 103 应该发送"

    print("✅ DelayedStrategy 测试通过")

if __name__ == "__main__":
    test_immediate_strategy()
    test_delayed_strategy()
    print("\n✅ 所有功能测试通过!")
