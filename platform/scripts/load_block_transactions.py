"""
示例：从 block JSON 加载交易，只保留指定字段。

用法：
    python scripts/load_block_transactions.py [block_json_path]

不传参数时用 ref/block_346769956.json 作为样例。

输出的 BlockTransaction 包含：
    signatures, message, fee, preBalances, postBalances,
    instructions, logMessages, timestamp，以及区块的 slot。
"""
import json
import sys
from pathlib import Path

# 直接运行脚本时把项目根目录加入 sys.path
PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from type import BlockTransaction
from utils import load_block_transactions

DEFAULT_BLOCK = PROJECT_ROOT / "ref" / "block_346769956.json"


def sample(tx: BlockTransaction) -> dict:
    """把一笔交易转成便于打印的 dict（长字段截断）"""
    return {
        "slot": tx.slot,
        "timestamp": tx.timestamp,
        "signature": tx.signature,
        "fee": tx.fee,
        "signatures": tx.signatures,
        "preBalances": tx.preBalances,
        "postBalances": tx.postBalances,
        "instructions": str(tx.instructions)[:300] + "...",
        "logMessages": tx.logMessages,
        "message_keys": list(tx.message.keys()),
        "message_accountKeys": str(tx.message.get("accountKeys"))[:200] + "...",
    }


def main() -> None:
    block_path = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_BLOCK

    txs = load_block_transactions(block_path)
    print(f"区块文件: {block_path}")
    print(f"加载交易数: {len(txs)}")
    print(f"slot: {txs[0].slot if txs else 'N/A'}")
    print("-" * 60)
    if txs:
        print("第一笔交易样例:")
        print(json.dumps(sample(txs[0]), indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
