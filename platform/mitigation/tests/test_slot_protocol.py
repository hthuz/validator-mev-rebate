"""测试基于 slot 号的广播 + end 标识机制"""
import sys
from pathlib import Path

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.server import create_server
from mitigation.strategy import ImmediateStrategy, DelayedStrategy
from mitigation.transaction_manager import TransactionManager


def _make_test_server(strategy=None, debug_mode=False):
    """构造一个使用固定 slot 号的测试服务器"""
    server = create_server(strategy=strategy or ImmediateStrategy())
    server.debug_mode = debug_mode
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
    """immediate 策略收到交易就发送；收到 end 后才推进 slot（非 debug 模式）"""
    server = _make_test_server(debug_mode=False)
    client = server.app.test_client()

    resp = client.post('/sendTransaction', json={
        'tx_id': 'tx_1', 'slot': 1000, 'signature': 'sig_1'
    })
    assert resp.status_code == 200

    records = server.transaction_manager.get_sent_records()
    assert len(records) == 1
    assert records[0].tx_id == 'tx_1'
    assert records[0].validator == 'validator_a'
    assert server.slot_manager.get_current_slot() == 1000

    client.post('/sendTransaction', json={'slot': 1000, 'end': True})
    assert server.slot_manager.get_current_slot() == 1001


def test_stale_end_marker_is_ignored():
    """非当前 slot 的 end 标识不应该推进 slot"""
    server = _make_test_server(debug_mode=False)
    client = server.app.test_client()

    resp = client.post('/sendTransaction', json={'slot': 9999, 'end': True})
    assert resp.status_code == 200
    assert server.slot_manager.get_current_slot() == 1000


def test_delayed_strategy_sends_on_slot_advance():
    """延迟策略在收到交易时不发送，slot 推进后才发送"""
    server = _make_test_server(strategy=DelayedStrategy(delay_slots=1), debug_mode=False)
    client = server.app.test_client()

    client.post('/sendTransaction', json={
        'tx_id': 'tx_delay', 'slot': 1000, 'signature': 'sig_delay'
    })
    assert len(server.transaction_manager.get_sent_records()) == 0

    client.post('/sendTransaction', json={'slot': 1000, 'end': True})
    records = server.transaction_manager.get_sent_records()
    assert len(records) == 1
    assert records[0].tx_id == 'tx_delay'
    assert server.slot_manager.get_current_slot() == 1001


def test_debug_mode_waits_for_manual_confirmation():
    """debug 模式：收到 end 并处理完交易后，仍需手动确认才推进"""
    server = _make_test_server(debug_mode=True)
    client = server.app.test_client()

    # 先发交易，immediate 策略立即发送
    client.post('/sendTransaction', json={
        'tx_id': 'tx_d1', 'slot': 1000, 'signature': 'sig_d1'
    })
    assert len(server.transaction_manager.get_sent_records()) == 1

    # 没收到 end 之前，手动推进被拒绝
    resp = client.post('/slot/advance')
    assert resp.status_code == 200
    assert resp.get_json()['status'] == 'waiting'
    assert server.slot_manager.get_current_slot() == 1000

    # 收到 end 后，交易已处理完，进入待确认状态，但还不推进
    client.post('/sendTransaction', json={'slot': 1000, 'end': True})
    assert server.slot_manager.get_current_slot() == 1000
    assert server._slot_ready_to_advance
    assert client.get('/health').get_json()['waiting_for_confirmation'] is True

    # 手动确认后才推进
    resp = client.post('/slot/advance')
    assert resp.get_json()['status'] == 'ok'
    assert server.slot_manager.get_current_slot() == 1001
    assert not server._slot_ready_to_advance
    assert client.get('/health').get_json()['waiting_for_confirmation'] is False


def test_non_debug_mode_rejects_manual_advance():
    """非 debug 模式下 /slot/advance 不可用"""
    server = _make_test_server(debug_mode=False)
    client = server.app.test_client()
    resp = client.post('/slot/advance')
    assert resp.status_code == 400


if __name__ == "__main__":
    test_slot_manager_slot_number()
    test_end_marker_tracking()
    test_immediate_send_and_end_advances_slot()
    test_stale_end_marker_is_ignored()
    test_delayed_strategy_sends_on_slot_advance()
    test_debug_mode_waits_for_manual_confirmation()
    test_non_debug_mode_rejects_manual_advance()
    print("\n✅ 所有 slot 协议测试通过!")
