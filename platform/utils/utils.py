
import os
import re
from pathlib import Path
from typing import Dict, List, Optional

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
