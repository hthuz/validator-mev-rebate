"""验证发送统计：未延迟只计数，延迟交易留详情并给出汇总。"""
import sys
from pathlib import Path

project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from mitigation.transaction_manager import TransactionManager
from type.models import TransactionRecord


def _record(signature, slot, validator, delay):
    return TransactionRecord(
        signature=signature,
        slot=slot,
        validator=validator,
        strategy="RiskAwareStrategy",
        target_slot=slot + delay,
        delay_slots=delay,
        leader_risky=delay > 0,
    )


def test_empty_report():
    tm = TransactionManager()
    report = tm.get_report()
    assert report['total_processed'] == 0
    assert report['immediate_count'] == 0
    assert report['delayed_count'] == 0
    assert report['delay_stats']['all']['avg'] == 0.0
    assert report['delay_stats']['delayed']['max'] == 0
    assert report['slot_overview']['moves'] == []
    assert report['delayed_records'] == []
    print("OK empty report")


def test_report_counts_and_stats():
    tm = TransactionManager()
    for signature, slot, validator in [
        ("imm1", 10, "a"), ("imm2", 11, "b"), ("imm3", 12, "c"),
    ]:
        tm.add_sent_record(_record(signature, slot, validator, 0))
    for signature, slot, validator, delay in [
        ("d1", 10, "x", 2), ("d2", 11, "y", 5), ("d3", 12, "z", 1),
    ]:
        tm.add_sent_record(_record(signature, slot, validator, delay))

    report = tm.get_report()
    assert report['total_processed'] == 6
    assert report['immediate_count'] == 3
    assert report['delayed_count'] == 3

    all_stats = report['delay_stats']['all']
    assert all_stats['count'] == 6
    assert all_stats['sum'] == 8
    assert all_stats['avg'] == round(8 / 6, 6)
    assert all_stats['min'] == 0
    assert all_stats['max'] == 5

    delayed_stats = report['delay_stats']['delayed']
    assert delayed_stats['count'] == 3
    assert delayed_stats['sum'] == 8
    assert delayed_stats['avg'] == round(8 / 3, 6)
    assert delayed_stats['min'] == 1
    assert delayed_stats['max'] == 5

    overview = report['slot_overview']
    assert overview['original'] == {"10": 1, "11": 1, "12": 1}
    assert overview['target'] == {"12": 1, "13": 1, "16": 1}
    assert overview['moves'] == [
        {"original_slot": 10, "target_slot": 12, "count": 1},
        {"original_slot": 11, "target_slot": 16, "count": 1},
        {"original_slot": 12, "target_slot": 13, "count": 1},
    ]

    assert [r['signature'] for r in report['delayed_records']] == ["d1", "d2", "d3"]
    assert all(r['delay_slots'] > 0 for r in report['delayed_records'])
    assert len(tm.get_delayed_records()) == 3
    print("OK report counts and stats")


def test_clear_resets_counts():
    tm = TransactionManager()
    tm.add_sent_record(_record("imm", 10, "a", 0))
    tm.add_sent_record(_record("d", 10, "b", 3))
    tm.clear()
    report = tm.get_report()
    assert report['total_processed'] == 0
    assert report['delayed_count'] == 0
    assert report['delay_stats']['all']['sum'] == 0
    print("OK clear resets counts")


if __name__ == "__main__":
    test_empty_report()
    test_report_counts_and_stats()
    test_clear_resets_counts()
    print("\nOK all send report tests passed")
