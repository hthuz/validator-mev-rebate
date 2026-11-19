
import os
import re
import json
from pathlib import Path
from typing import Any, Dict, List, Optional

import numpy as np


def find_match_path(auction_path: str, epoch: int) -> str:
    dirs = os.listdir(auction_path)
    epoch_path = ""
    for dir in dirs:
        if dir.split('.')[0] == str(epoch):
            epoch_path = os.path.join(auction_path, dir)
    return epoch_path



def tosol(amount: int) -> float:
    return amount / 1e9

def abbreviate_addr(addr: str) -> str:
    return addr[:4] + ".." + addr[-4:]


# ---------------------------------------------------------------------------
# 区块数据索引（mitigation 模块共享）
# ---------------------------------------------------------------------------

# slot 就是 block 号，直接写在区块文件名里（block_346769956.json），
# 不需要读文件内容，更跟 blockTime / timestamp 无关。
_SLOT_FROM_NAME_RE = re.compile(r'(\d+)')


def extract_slot_from_block_path(block_file: Path) -> Optional[int]:
    """
    从区块文件名（如 block_346769956.json）中解析出 slot 号。
    解析失败返回 None。
    """
    match = _SLOT_FROM_NAME_RE.search(Path(block_file).name)
    return int(match.group(1)) if match else None


def build_block_index(block_path: str) -> Dict[int, Path]:
    """
    扫描 BLOCK_PATH 下的区块 JSON，建立 slot 号 -> 区块文件路径 的索引。

    slot 与 block 是同一个概念，slot 号取自区块文件名，与 blockTime 无关。
    每个区块文件对应一个 slot。

    Returns:
        {slot: block_file_path}。路径不存在时返回空字典。
    """
    index: Dict[int, Path] = {}
    root = Path(block_path) if block_path else None
    if root is None or not root.exists():
        return index

    for block_file in sorted(root.glob("*.json")):
        slot = extract_slot_from_block_path(block_file)
        if slot is not None:
            index[slot] = block_file

    return index


# ---------------------------------------------------------------------------
# 区块交易加载（公共：mitigation / sandwich 都可复用）
# ---------------------------------------------------------------------------

def parse_block_transactions(
    block_data: Dict[str, Any], slot: Optional[int] = None
) -> List[Any]:
    """
    从区块 JSON（dict）里提取交易，只保留 BlockTransaction 定义的字段。

    兼容两种输入：
      1. RPC 响应外壳：{"result": {"transactions": [...], "blockTime": ...}}
      2. 裸区块结构：{"transactions": [...], "blockTime": ...}

    slot 解析优先级：显式参数 > JSON 里的 slot 字段。
    两者都没有时抛 ValueError（调用方可用文件名解析后显式传入）。

    Returns:
        List[BlockTransaction]；返回 Any 只是为了避免公共层循环导入，
        实际元素类型见 type.models.BlockTransaction。
    """
    # 延迟导入：type.validator 会 import utils，模块级导入会形成循环依赖
    from type.models import BlockTransaction

    result = block_data.get('result', block_data)

    if slot is None:
        slot = result.get('slot')
    if slot is None:
        raise ValueError("无法从区块数据解析 slot，请显式传入 slot")

    timestamp = result.get('blockTime')

    transactions: List[BlockTransaction] = []
    for raw_tx in result.get('transactions', []):
        tx_body = raw_tx.get('transaction', {}) or {}
        tx_meta = raw_tx.get('meta', {}) or {}
        message = tx_body.get('message', {}) or {}

        transactions.append(BlockTransaction(
            slot=int(slot),
            timestamp=timestamp,
            signatures=tx_body.get('signatures', []),
            message=message,
            fee=tx_meta.get('fee', 0),
            preBalances=tx_meta.get('preBalances', []),
            postBalances=tx_meta.get('postBalances', []),
            instructions=message.get('instructions', []),
            logMessages=tx_meta.get('logMessages', []),
        ))

    return transactions


def load_block_transactions(
    block_file: Any, slot: Optional[int] = None
) -> List[Any]:
    """
    从区块 JSON 文件加载交易。

    slot 默认从文件名解析（block_346769956.json -> 346769956），
    也可显式传入；显式传入优先。

    Returns:
        List[BlockTransaction]
    """
    path = Path(block_file)
    if slot is None:
        slot = extract_slot_from_block_path(path)
    if slot is None:
        raise ValueError(f"无法从文件名解析 slot: {path.name}，请显式传入 slot")

    with open(path, 'r') as f:
        block_data = json.load(f)

    return parse_block_transactions(block_data, slot=slot)
