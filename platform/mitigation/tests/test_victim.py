"""victim 数据源、三档评估与 ClickHouse leader 源测试（不依赖真实 CH）。"""
import sys
from pathlib import Path
from unittest.mock import patch

project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.evaluation import evaluate_victim_outcomes, load_sandwich_leaders
from mitigation.server import create_server
from mitigation.slot_manager import SlotManager
from mitigation.strategy import ImmediateStrategy
from mitigation.user.victim_source import VictimReplaySource
from type.models import RoutableTransaction, SandwichTransaction
from utils.sandwich import get_victim_transactions


def _victim(slot, signature, position=1):
    return SandwichTransaction(
        slot=slot,
        signature=signature,
        position=position,
        type='victim',
        signer='searcher',
        fromToken='SOL',
        toToken='TOKEN',
    )


def _row(slot, signature):
    return {
        'slot': slot,
        'signature': signature,
        'position': 1,
        'signer': 'searcher',
        'accountKeys': ['a', 'b'],
        'consecutive': True,
        'programs': ['p'],
        'inbundle': False,
        'signerSame': False,
        'crossBlock': False,
        'oppositeVictim': False,
        'fromToken': 'SOL',
        'toToken': 'TOKEN',
        'fromAmount': 1.0,
        'toAmount': 2.0,
        'fromDiff': 0.0,
        'toDiff': 0.0,
        'timestamp': '2025-01-01 00:00:00',
    }


def test_sandwich_transaction_satisfies_protocol():
    tx = _victim(100, 'sig')
    assert isinstance(tx, RoutableTransaction)
    assert tx.slot == 100
    assert tx.signature == 'sig'


def test_slot_manager_uses_injected_leaders():
    sm = SlotManager(leaders={100: 'a', 101: 'b'})
    assert sm.get_leader(100) == 'a'
    assert sm.get_leader(101) == 'b'
    assert sm.get_leader(999) is None
    assert sm._slot_leaders == {100: 'a', 101: 'b'}


def test_victim_query_dedups_by_signature():
    rows = [_row(100, 'sig1'), _row(100, 'sig1'), _row(101, 'sig2')]
    with patch('utils.sandwich.query_clickhouse', return_value=rows):
        txs = get_victim_transactions(802)
    assert [tx.signature for tx in txs] == ['sig1', 'sig2']
    assert txs[0].type == 'victim'
    assert txs[0].accountKeys == ['a', 'b']


def test_victim_replay_groups_by_slot():
    txs = [_victim(100, 'sig_b', position=2), _victim(100, 'sig_a', position=1),
           _victim(101, 'sig_c')]
    source = VictimReplaySource(802, loader=lambda epoch: txs)
    assert source.available_slots() == [100, 101]
    assert [tx.signature for tx in source.transactions_for(100)] == ['sig_a', 'sig_b']
    assert source.transactions_for(999) == []
    assert len(source.all_transactions()) == 3


def test_sandwich_leaders_loader_injection():
    assert load_sandwich_leaders(802, loader=lambda epoch: {'bad', 'bad2'}) == {'bad', 'bad2'}


class _FakeSlotManager:
    def __init__(self, leaders):
        self.leaders = leaders

    def get_leader(self, slot):
        return self.leaders.get(slot)


class _FakeServer:
    def __init__(self, leaders):
        self.slot_manager = _FakeSlotManager(leaders)


class _FakeReplay:
    def __init__(self, txs):
        self.txs = txs

    def all_transactions(self):
        return self.txs


def test_three_tier_labels():
    txs = [
        _victim(100, 'sig_stay'),
        _victim(101, 'sig_bad'),
        _victim(103, 'sig_clean'),
        _victim(105, 'sig_untouched'),
    ]
    report = {
        'delayed_records': [
            {'signature': 'sig_stay', 'target_slot': 100},
            {'signature': 'sig_bad', 'target_slot': 102},
            {'signature': 'sig_clean', 'target_slot': 104},
        ]
    }
    server = _FakeServer({100: 'bad', 102: 'bad', 104: 'clean'})
    replay = _FakeReplay(txs)

    result = evaluate_victim_outcomes(
        report, server, replay, sandwich_leaders={'bad'}
    )

    assert result['evaluated_victims'] == 4
    assert result['counts'] == {
        'confirmed_sandwiched': 2,
        'leader_observed_sandwich': 1,
        'clean_leader': 1,
    }
    assert result['ratios']['clean_leader'] == 0.25


def test_victim_tx_goes_through_existing_server():
    sm = SlotManager(leaders={100: 'a', 101: 'b'})
    server = create_server(strategy=ImmediateStrategy(), with_transport=False, slot_manager=sm)
    server.set_slot(100)
    assert server.submit_transaction(_victim(100, 'sig_victim'))
    report = server.get_report()
    assert report['total_processed'] == 1
    assert report['immediate_count'] == 1


if __name__ == "__main__":
    test_sandwich_transaction_satisfies_protocol()
    test_slot_manager_uses_injected_leaders()
    test_victim_query_dedups_by_signature()
    test_victim_replay_groups_by_slot()
    test_sandwich_leaders_loader_injection()
    test_three_tier_labels()
    test_victim_tx_goes_through_existing_server()
    print("\nOK all victim tests passed")
