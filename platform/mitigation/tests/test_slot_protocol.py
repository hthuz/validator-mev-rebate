"""测试基于 slot 号的广播 + end 标识机制"""
import sys
from pathlib import Path
from unittest.mock import patch

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.server import create_server, NEW_SLOT_EVENT, SIMULATION_DONE_EVENT
from mitigation.strategy import ImmediateStrategy, DelayedStrategy
from mitigation.transaction_manager import TransactionManager
from type.models import BlockTransaction


def _tx_payload(slot, signature):
    """构造 user client 会发送的 BlockTransaction 完整字段"""
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
    ).to_dict()


def _make_test_server(strategy=None):
    """构造一个使用固定 slot 号的测试服务器"""
    server = create_server(strategy=strategy or ImmediateStrategy())
    sm = server.slot_manager
    sm._start_slot = 1000
    sm._end_slot = 1002
    sm._slot_leaders = {1000: 'validator_a', 1001: 'validator_b', 1002: 'validator_c'}
    sm.reset_to_first_slot()
    return server


def test_slot_manager_slot_number():
    """slot 是 slot 号，每次 +1 推进"""
    server = _make_test_server()
    sm = server.slot_manager

    assert sm.get_available_slots() == [1000, 1001, 1002]
    assert sm.get_current_slot() == 1000
    assert sm.get_leader(1000) == 'validator_a'
    assert sm.has_next_slot()

    assert sm.advance_slot() == 1001
    assert sm.get_current_slot() == 1001

    sm.advance_slot()
    assert sm.get_current_slot() == 1002
    assert not sm.has_next_slot()


def test_end_marker_tracking():
    """TransactionManager 能正确记录 slot 的结束标识"""
    tm = TransactionManager()
    assert not tm.is_slot_ended(1000)

    tm.mark_slot_end(1000)
    assert tm.is_slot_ended(1000)

    tm.clear_slot_end(1000)
    assert not tm.is_slot_ended(1000)


def test_immediate_send_and_end_advances_slot():
    """immediate 策略收到交易就发送；收到 end 后才推进 slot"""
    server = _make_test_server()
    client = server.app.test_client()

    resp = client.post('/sendTransaction', json=_tx_payload(1000, 'sig_1'))
    assert resp.status_code == 200

    # 未延迟交易只计数，不留存逐笔详情
    report = server.transaction_manager.get_report()
    assert report['total_processed'] == 1
    assert report['immediate_count'] == 1
    assert report['delayed_count'] == 0
    assert report['delayed_records'] == []
    assert server.slot_manager.get_current_slot() == 1000

    client.post('/sendTransaction', json={'slot': 1000, 'end': True})
    assert server.slot_manager.get_current_slot() == 1001


def test_stale_end_marker_is_ignored():
    """非当前 slot 的 end 标识不应该推进 slot"""
    server = _make_test_server()
    client = server.app.test_client()

    resp = client.post('/sendTransaction', json={'slot': 9999, 'end': True})
    assert resp.status_code == 200
    assert server.slot_manager.get_current_slot() == 1000


def test_delayed_strategy_sends_on_slot_advance():
    """延迟策略在收到交易时不发送，slot 推进后才发送"""
    server = _make_test_server(strategy=DelayedStrategy(delay_slots=1))
    client = server.app.test_client()

    client.post('/sendTransaction', json=_tx_payload(1000, 'sig_delay'))
    assert server.transaction_manager.get_report()['total_processed'] == 0

    client.post('/sendTransaction', json={'slot': 1000, 'end': True})
    report = server.transaction_manager.get_report()
    assert report['total_processed'] == 1
    assert report['delayed_count'] == 1
    assert report['delayed_records'][0]['signature'] == 'sig_delay'
    assert report['delayed_records'][0]['target_slot'] == 1001
    assert server.slot_manager.get_current_slot() == 1001


def test_manual_advance_route_removed():
    """/slot/advance 路由已移除"""
    server = _make_test_server()
    client = server.app.test_client()
    assert client.post('/slot/advance').status_code == 404


def test_end_slot_no_more_new_slot_broadcast():
    """MITIGATION_END_SLOT 处理完后不再广播 new_slot，只发 simulation_done"""
    server = _make_test_server()
    client = server.app.test_client()
    sm = server.slot_manager

    # 直接把当前 slot 设为 END slot
    sm.set_current_slot(sm._end_slot)

    with patch.object(server.socketio, 'emit') as mock_emit:
        client.post('/sendTransaction', json={'slot': sm._end_slot, 'end': True})

    events = [call.args[0] for call in mock_emit.call_args_list]
    assert NEW_SLOT_EVENT not in events, "END slot 处理完后不应广播 new_slot"
    assert SIMULATION_DONE_EVENT in events, "应发出 simulation_done"
    assert server.slot_manager.get_current_slot() == sm._end_slot, "END slot 不应再推进"
    assert server._running is False


def test_health_has_no_waiting_for_confirmation():
    """/health 不再暴露 waiting_for_confirmation"""
    server = _make_test_server()
    data = server.app.test_client().get('/health').get_json()
    assert 'waiting_for_confirmation' not in data
    assert 'current_slot' in data


if __name__ == "__main__":
    test_slot_manager_slot_number()
    test_end_marker_tracking()
    test_immediate_send_and_end_advances_slot()
    test_stale_end_marker_is_ignored()
    test_delayed_strategy_sends_on_slot_advance()
    test_manual_advance_route_removed()
    test_end_slot_no_more_new_slot_broadcast()
    test_health_has_no_waiting_for_confirmation()
    print("\n✅ 所有 slot 协议测试通过!")
