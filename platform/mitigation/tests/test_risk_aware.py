"""验证 ValidatorRiskStore 与 RiskAwareStrategy 的核心行为。"""
import sys
from pathlib import Path

project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from type.models import BlockTransaction
from mitigation.risk_store import ValidatorRiskStore
from mitigation.strategy import StrategyContext, RiskAwareStrategy
from mitigation.server import create_server


def _make_tx(slot=100, signature="test_sig"):
    return BlockTransaction(
        slot=slot,
        timestamp=0,
        signatures=[signature],
        message={},
        fee=5000,
        preBalances=[],
        postBalances=[],
        instructions=[],
        logMessages=[],
    )


def _make_store(calls=None):
    """800: bad=0.8, edge=0.5, nan_validator=nan; 801: bad=0.2, edge=0.6"""
    data = {
        800: {"bad": 0.8, "edge": 0.5, "nan_validator": float("nan")},
        801: {"bad": 0.2, "edge": 0.6},
    }

    def loader(epoch):
        if calls is not None:
            calls.append(epoch)
        return data[epoch]

    return ValidatorRiskStore(
        start_epoch=800, end_epoch=801, threshold=0.5, loader=loader
    )


def _make_strategy(store, leaders, max_skip=5):
    strategy = RiskAwareStrategy(max_skip_slots=max_skip)
    strategy.set_context(StrategyContext(
        get_leader=lambda slot: leaders.get(slot),
        risk_store=store,
    ))
    return strategy


def test_risk_store_any_epoch_exceeds_threshold():
    store = _make_store()
    assert store.is_risky("bad"), "epoch 800 的 0.8 应判为高风险"
    assert store.is_risky("edge"), "epoch 801 的 0.6 应判为高风险（任一 epoch 即可）"
    assert not store.is_risky("nan_validator"), "NaN 视为无数据"
    assert not store.is_risky("unknown"), "缺失数据视为安全"
    assert not store.is_risky(None), "空 leader 视为安全"
    assert store.get_ratios("bad") == {800: 0.8, 801: 0.2}
    print("OK risk store threshold")


def test_risk_store_epoch_range():
    calls = []
    store = _make_store(calls)
    assert calls == [800, 801], f"只加载配置范围内的 epoch，实际 {calls}"
    assert store.summary()["loaded_epochs"] == [800, 801]

    only_801 = ValidatorRiskStore(
        start_epoch=801,
        end_epoch=801,
        threshold=0.5,
        loader=lambda epoch: {800: {"bad": 0.8}, 801: {"bad": 0.2}}[epoch],
    )
    assert not only_801.is_risky("bad"), "范围外的 ratio 不应参与判定"
    print("OK risk store epoch range")


def test_skip_until_safe_leader():
    store = _make_store()
    leaders = {100: "bad", 101: "bad", 102: "safe", 103: "safe"}
    strategy = _make_strategy(store, leaders, max_skip=5)
    tx = _make_tx(100)

    assert not strategy.should_send(tx, 99), "交易 slot 之前不能发送"
    assert not strategy.should_send(tx, 100), "高风险 leader 应跳过"
    assert not strategy.should_send(tx, 101), "连续高风险应继续持有"
    assert strategy.should_send(tx, 102), "遇到安全 leader 应发送"

    record = strategy.send_to_validator(tx, "safe", 102)
    assert record.target_slot == 102
    assert record.delay_slots == 2
    assert not record.leader_risky
    print("OK skip until safe leader")


def test_force_send_at_max_window():
    store = _make_store()
    leaders = {100: "bad", 101: "bad", 102: "bad", 103: "bad"}
    strategy = _make_strategy(store, leaders, max_skip=2)
    tx = _make_tx(100)

    assert not strategy.should_send(tx, 100)
    assert not strategy.should_send(tx, 101)
    assert strategy.should_send(tx, 102), "达到最大窗口应强制发送"

    record = strategy.send_to_validator(tx, "bad", 102)
    assert record.delay_slots == 2
    assert record.leader_risky, "强制发送时记录的 leader 仍是高风险"
    print("OK force send at max window")


def test_no_context_falls_back_to_immediate():
    strategy = RiskAwareStrategy(max_skip_slots=2)
    assert strategy.should_send(_make_tx(100), 100), "无上下文时退化为立即发送"
    print("OK no-context fallback")


def test_server_end_to_end_routing():
    store = _make_store()
    strategy = RiskAwareStrategy(max_skip_slots=2)
    server = create_server(strategy=strategy, risk_store=store)

    server.slot_manager._slot_leaders = {
        100: "bad", 101: "bad", 102: "safe",
    }
    server.transaction_manager.add_transaction(_make_tx(100, "sig_e2e"))

    for slot in (100, 101):
        server.slot_manager.set_current_slot(slot)
        server._process_transactions()
    assert server.transaction_manager.get_report()['total_processed'] == 0, "前两个 slot 不应发送"

    server.slot_manager.set_current_slot(102)
    server._process_transactions()
    report = server.transaction_manager.get_report()
    assert report['total_processed'] == 1
    assert report['delayed_count'] == 1
    detail = report['delayed_records'][0]
    assert detail['validator'] == "safe"
    assert detail['target_slot'] == 102
    assert detail['delay_slots'] == 2

    client = server.app.test_client()
    sent = client.get('/transactions/sent').get_json()
    assert sent['total_processed'] == 1
    assert sent['delayed_records'][0]['target_slot'] == 102
    assert sent['delayed_records'][0]['delay_slots'] == 2
    assert sent['delay_stats']['delayed'] == {
        "count": 1, "sum": 2, "avg": 2.0, "min": 2, "max": 2,
    }
    assert sent['slot_overview']['moves'] == [
        {"original_slot": 100, "target_slot": 102, "count": 1}
    ]

    risk = client.get('/risk/summary').get_json()
    assert risk['loaded'] and risk['threshold'] == 0.5
    assert 'bad' in risk['risky_validators']
    assert 'nan_validator' not in risk['risky_validators']
    print("OK server end-to-end routing")


if __name__ == "__main__":
    test_risk_store_any_epoch_exceeds_threshold()
    test_risk_store_epoch_range()
    test_skip_until_safe_leader()
    test_force_send_at_max_window()
    test_no_context_falls_back_to_immediate()
    test_server_end_to_end_routing()
    print("\nOK all risk-aware tests passed")
