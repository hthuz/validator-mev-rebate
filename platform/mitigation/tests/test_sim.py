"""验证进程内 sim 与 HTTP 路径共用同一套业务函数。"""
import sys
from pathlib import Path

project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.risk_store import ValidatorRiskStore
from mitigation.server import create_server
from mitigation.sim import run_simulation
from mitigation.strategy import ImmediateStrategy, RiskAwareStrategy
from type.models import BlockTransaction


def _tx(slot, signature):
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


class FakeReplay:
    def __init__(self, txs_by_slot):
        self.txs_by_slot = txs_by_slot
        self.visited = []

    def transactions_for(self, slot):
        self.visited.append(slot)
        return self.txs_by_slot.get(slot, [])


def _make_server(strategy, leaders, start=100, end=102, risk_store=None, transport=False):
    server = create_server(
        strategy=strategy, risk_store=risk_store, with_transport=transport
    )
    sm = server.slot_manager
    sm._start_slot = start
    sm._end_slot = end
    sm._slot_leaders = leaders
    sm.reset_to_first_slot()
    return server


def _without_timestamps(report):
    clean = dict(report)
    clean['delayed_records'] = [
        {k: v for k, v in r.items() if k != 'sent_at'}
        for r in report['delayed_records']
    ]
    return clean


def test_sim_has_no_transport():
    server = _make_server(ImmediateStrategy(), {100: 'a'})
    assert server.app is None
    assert server.socketio is None
    try:
        server.start()
    except RuntimeError:
        pass
    else:
        raise AssertionError("with_transport=False 的 server 不应能 start()")


def test_sim_uses_config_slot_range():
    replay = FakeReplay({slot: [_tx(slot, f'sig_{slot}')] for slot in (99, 100, 101, 102, 103)})
    server = _make_server(
        ImmediateStrategy(),
        {100: 'a', 101: 'b', 102: 'c'},
        start=100,
        end=102,
    )

    report = run_simulation(server=server, replay=replay)

    assert replay.visited == [100, 101, 102]
    assert report['total_processed'] == 3
    assert report['immediate_count'] == 3
    assert server.get_current_slot() == 102


def test_sim_risk_aware_skips_until_safe_leader():
    store = ValidatorRiskStore(
        start_epoch=800,
        end_epoch=801,
        threshold=0.5,
        loader=lambda epoch: {
            800: {'bad': 0.8},
            801: {'bad': 0.2},
        }[epoch],
        marinade_loader=lambda epoch: {},
    )
    replay = FakeReplay({100: [_tx(100, 'sig_risk')]})
    server = _make_server(
        RiskAwareStrategy(max_skip_slots=5),
        {100: 'bad', 101: 'bad', 102: 'safe'},
        start=100,
        end=102,
        risk_store=store,
    )

    report = run_simulation(server=server, replay=replay)

    assert report['total_processed'] == 1
    assert report['delayed_count'] == 1
    detail = report['delayed_records'][0]
    assert detail['validator'] == 'safe'
    assert detail['target_slot'] == 102
    assert detail['delay_slots'] == 2
    assert not detail['leader_risky']


def test_sim_matches_http_path():
    leaders = {100: 'a', 101: 'b', 102: 'c'}
    txs_by_slot = {
        100: [_tx(100, 'sig_100_1'), _tx(100, 'sig_100_2')],
        101: [_tx(101, 'sig_101_1')],
        102: [],
    }

    replay = FakeReplay(txs_by_slot)
    sim_server = _make_server(ImmediateStrategy(), leaders)
    sim_report = run_simulation(server=sim_server, replay=replay)

    http_server = _make_server(ImmediateStrategy(), leaders, transport=True)
    client = http_server.app.test_client()
    for slot in (100, 101, 102):
        for tx in txs_by_slot[slot]:
            resp = client.post('/sendTransaction', json=tx.to_dict())
            assert resp.status_code == 200
        client.post('/sendTransaction', json={'slot': slot, 'end': True})

    http_report = http_server.get_report()
    assert _without_timestamps(sim_report) == _without_timestamps(http_report)


if __name__ == "__main__":
    test_sim_has_no_transport()
    test_sim_uses_config_slot_range()
    test_sim_risk_aware_skips_until_safe_leader()
    test_sim_matches_http_path()
    print("\nOK all sim tests passed")
