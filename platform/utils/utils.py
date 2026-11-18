
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

# blockTime 在区块 JSON 中是第一个字段，只读文件头部即可拿到，
# 避免为了拿一个时间戳而解析几十 GB 的交易数据。
_BLOCK_TIME_RE = re.compile(rb'"blockTime"\s*:\s*(\d+)')
_SLOT_FROM_NAME_RE = re.compile(r'(\d+)')


def build_block_time_index(block_path: str) -> Dict[int, List[Path]]:
    """
    扫描 BLOCK_PATH 下的区块 JSON，建立 slot start time -> 区块文件列表 的索引。

    Solana 的 blockTime 就是 slot 的开始时间（秒级 Unix 时间戳）。
    同一秒内可能产生多个 slot，因此多个区块文件会映射到同一个 start time，
    这里把它们归为同一个模拟 slot。

    Returns:
        {blockTime: [block_file_path, ...]}，key 已按时间升序插入。
        路径不存在时返回空字典。
    """
    index: Dict[int, List[Path]] = {}
    root = Path(block_path) if block_path else None
    if root is None or not root.exists():
        return index

    for block_file in sorted(root.glob("*.json")):
        try:
            with open(block_file, "rb") as f:
                head = f.read(512)
            match = _BLOCK_TIME_RE.search(head)
            if match is None:
                continue
            block_time = int(match.group(1))
            index.setdefault(block_time, []).append(block_file)
        except OSError:
            continue

    return index


def extract_slot_from_block_path(block_file: Path) -> Optional[int]:
    """
    从区块文件名（如 block_346769956.json）中解析出真实的 slot 号。
    解析失败返回 None。
    """
    match = _SLOT_FROM_NAME_RE.search(Path(block_file).name)
    return int(match.group(1)) if match else None
