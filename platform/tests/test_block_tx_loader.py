"""测试公共的 block JSON -> BlockTransaction 加载逻辑"""
import json
import pytest

from type import BlockTransaction
from utils import load_block_transactions, parse_block_transactions


def _raw_tx(sig="sig_1"):
    return {
        "transaction": {
            "signatures": [sig],
            "message": {
                "accountKeys": [{"pubkey": "acc1", "writable": True}],
                "recentBlockhash": "bh",
                "instructions": [{"program": "vote", "data": "x"}],
                "header": {"numRequiredSignatures": 1},
            },
        },
        "meta": {
            "fee": 5000,
            "preBalances": [100, 200],
            "postBalances": [90, 200],
            "logMessages": ["Program log"],
            "innerInstructions": [{"index": 0}],
            "err": None,
        },
        "version": "legacy",
    }


def test_parse_rpc_wrapped_block():
    """兼容 RPC 响应外壳 {result: {...}}"""
    data = {
        "jsonrpc": "2.0",
        "result": {
            "blockTime": 1749916801,
            "transactions": [_raw_tx()],
        },
    }
    txs = parse_block_transactions(data, slot=346769956)

    assert len(txs) == 1
    tx = txs[0]
    assert isinstance(tx, BlockTransaction)
    assert tx.slot == 346769956
    assert tx.timestamp == 1749916801
    assert tx.signature == "sig_1"
    assert tx.signatures == ["sig_1"]
    assert tx.fee == 5000
    assert tx.preBalances == [100, 200]
    assert tx.postBalances == [90, 200]
    assert tx.instructions == [{"program": "vote", "data": "x"}]
    assert tx.logMessages == ["Program log"]
    assert tx.message["recentBlockhash"] == "bh"

    # 只保留指定字段，innerInstructions / version / err 都被丢掉
    assert set(vars(tx)) == {
        "slot", "timestamp", "signatures", "message", "fee",
        "preBalances", "postBalances", "instructions", "logMessages",
    }


def test_parse_plain_block_and_slot_from_json():
    """兼容裸结构，slot 可从 JSON 里取"""
    data = {"blockTime": 1, "slot": 42, "transactions": [_raw_tx("sig_2")]}
    txs = parse_block_transactions(data)
    assert txs[0].slot == 42
    assert txs[0].signature == "sig_2"


def test_parse_requires_slot():
    with pytest.raises(ValueError, match="slot"):
        parse_block_transactions({"blockTime": 1, "transactions": []})


def test_load_from_file_slot_from_filename(tmp_path):
    """从文件名解析 slot，并正确读取 JSON"""
    block_file = tmp_path / "block_999.json"
    block_file.write_text(json.dumps({
        "result": {"blockTime": 123, "transactions": [_raw_tx("sig_3")]}
    }))

    txs = load_block_transactions(block_file)
    assert len(txs) == 1
    assert txs[0].slot == 999
    assert txs[0].timestamp == 123
    assert txs[0].signature == "sig_3"


def test_load_explicit_slot_overrides_filename(tmp_path):
    block_file = tmp_path / "block_999.json"
    block_file.write_text(json.dumps({
        "blockTime": 1, "transactions": [_raw_tx()]
    }))
    assert load_block_transactions(block_file, slot=777)[0].slot == 777


def test_empty_transaction_list(tmp_path):
    block_file = tmp_path / "block_1.json"
    block_file.write_text(json.dumps({"blockTime": 1, "transactions": []}))
    assert load_block_transactions(block_file) == []


def test_real_ref_block(tmp_path):
    """用 ref 里的真实样例跑一遍（跳过文件不存在的情况）"""
    from pathlib import Path
    ref = Path(__file__).resolve().parent.parent / "ref" / "block_346769956.json"
    if not ref.exists():
        pytest.skip("ref block not present")

    txs = load_block_transactions(ref)
    assert len(txs) > 0
    assert all(tx.slot == 346769956 for tx in txs)
    assert all(isinstance(tx, BlockTransaction) for tx in txs)
    assert txs[0].signature
    assert txs[0].fee >= 0
