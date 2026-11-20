"""victim 数据源、三档评估与 ClickHouse leader 源测试（不依赖真实 CH）。"""
import sys
from pathlib import Path
from unittest.mock import patch

project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.evaluation import (
    evaluate_leader_risk_exposure,
    evaluate_victim_outcomes,
    load_risky_validators,
    load_risky_validators_detail,
    load_sandwich_leaders,
)
from mitigation.risk_store import ValidatorRiskStore
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


def test_load_risky_validators_threshold_and_nan():
    ratios = {
        'bad': 0.8,
        'edge': 0.5,          # 等于阈值不算
        'nan_validator': float('nan'),
        'missing_value': 'n/a',
        'safe': 0.2,
    }
    risky = load_risky_validators(
        802,
        loader=lambda epoch: ratios,
        threshold=0.5,
        marinade_loader=lambda epoch: {},
    )
    assert risky == {'bad'}


def test_load_risky_validators_unions_marinade():
    detail = load_risky_validators_detail(
        802,
        loader=lambda epoch: {
            'sw_only': 0.8,
            'both': 0.7,
            'edge': 0.5,
            'nan': float('nan'),
        },
        threshold=0.5,
        marinade_loader=lambda epoch: {
            'm_only': 0.9,
            'both': 0.6,
            'edge': 0.5,
            'nan_m': float('nan'),
        },
        marinade_threshold=0.5,
    )
    assert detail['sandwich_risky_validators'] == {'sw_only', 'both'}
    assert detail['marinade_risky_validators'] == {'m_only', 'both'}
    assert detail['both_risky_validators'] == {'both'}
    assert detail['risky_validators'] == {'sw_only', 'both', 'm_only'}
    assert detail['sandwich_validator_count'] == 4
    assert detail['marinade_validator_count'] == 4
    assert detail['sandwich_threshold'] == 0.5
    assert detail['marinade_threshold'] == 0.5

    # 薄封装返回同一个并集
    assert load_risky_validators(
        802,
        loader=lambda epoch: {'sw_only': 0.8},
        threshold=0.5,
        marinade_loader=lambda epoch: {'m_only': 0.9},
        marinade_threshold=0.5,
    ) == {'sw_only', 'm_only'}

    # 加载失败只 warning，退化为空集合
    def _boom(epoch):
        raise RuntimeError('down')

    assert load_risky_validators(
        802, loader=_boom, marinade_loader=_boom, threshold=0.5, marinade_threshold=0.5
    ) == set()


def test_leader_risk_exposure_counts_all_sent_transactions():
    # immediate 的 3 笔也要计入，不只是延迟的 2 笔
    report = {
        'total_processed': 5,
        'landed_validators': {'bad': 2, 'safe': 2, 'unknown': 1},
    }
    result = evaluate_leader_risk_exposure(report, risky_validators={'bad'})
    assert result['total_sent'] == 5
    assert result['on_risky_leader'] == 2
    assert result['on_safe_leader'] == 3
    assert result['ratios']['on_risky_leader'] == 0.4
    assert result['ratios']['on_safe_leader'] == 0.6
    assert 'by_source' not in result


def test_leader_risk_exposure_by_source():
    report = {
        'landed_validators': {
            'sw': 2,      # 仅 sandwich risky
            'm': 3,       # 仅 Marinade risky
            'both': 1,    # 两条线都命中
            'safe': 4,
        }
    }
    result = evaluate_leader_risk_exposure(
        report,
        risky_validators={'sw', 'm', 'both'},
        sandwich_risky={'sw', 'both'},
        marinade_risky={'m', 'both'},
    )
    assert result['total_sent'] == 10
    assert result['on_risky_leader'] == 6
    assert result['on_safe_leader'] == 4
    # by_source 是重叠细分，不互斥
    assert result['by_source']['sandwich']['count'] == 3
    assert result['by_source']['marinade']['count'] == 4
    assert result['by_source']['both']['count'] == 1
    assert result['by_source']['marinade']['ratio'] == 0.4


def test_leader_risk_exposure_empty():
    result = evaluate_leader_risk_exposure(
        {'landed_validators': {}}, risky_validators={'bad'}
    )
    assert result['total_sent'] == 0
    assert result['ratios']['on_risky_leader'] == 0.0


def test_risk_store_unions_sandwich_and_marinade():
    # sandwich: sw_only=0.8, both=0.7
    # marinade: m_only=0.9, both=0.6, edge=0.5
    store = ValidatorRiskStore(
        start_epoch=800, end_epoch=800,
        threshold=0.5,
        loader=lambda epoch: {'sw_only': 0.8, 'both': 0.7},
        marinade_threshold=0.5,
        marinade_loader=lambda epoch: {
            'm_only': 0.9, 'both': 0.6, 'edge': 0.5, 'nan': float('nan'),
        },
    )
    # 并集判定
    assert store.is_risky('sw_only')
    assert store.is_risky('m_only')
    assert store.is_risky('both')
    assert not store.is_risky('edge'), "等于阈值不算"
    assert not store.is_risky('nan')
    assert not store.is_risky('unknown')
    # 来源分列
    assert store.sandwich_risky_validators() == ['both', 'sw_only']
    assert store.marinade_risky_validators() == ['both', 'm_only']
    assert store.both_risky_validators() == ['both']
    assert store.risky_validators() == ['both', 'm_only', 'sw_only']
    assert store.is_marinade_risky('m_only') and not store.is_sandwich_risky('m_only')

    summary = store.summary()
    assert summary['marinade_threshold'] == 0.5
    assert summary['marinade_risky_validators'] == ['both', 'm_only']
    assert summary['both_risky_validators'] == ['both']
    assert summary['marinade_validator_counts'] == {800: 4}


def test_marinade_risk_any_epoch_within_range():
    store = ValidatorRiskStore(
        start_epoch=800, end_epoch=801,
        threshold=0.5,
        loader=lambda epoch: {},
        marinade_threshold=0.5,
        marinade_loader=lambda epoch: {
            800: {'early': 0.7},
            801: {'late': 0.9},
        }[epoch],
    )
    assert store.is_risky('early') and store.is_risky('late')
    assert store.marinade_risky_validators() == ['early', 'late']


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
    test_load_risky_validators_threshold_and_nan()
    test_load_risky_validators_unions_marinade()
    test_leader_risk_exposure_counts_all_sent_transactions()
    test_leader_risk_exposure_by_source()
    test_leader_risk_exposure_empty()
    test_risk_store_unions_sandwich_and_marinade()
    test_marinade_risk_any_epoch_within_range()
    test_victim_tx_goes_through_existing_server()
    print("\nOK all victim tests passed")
