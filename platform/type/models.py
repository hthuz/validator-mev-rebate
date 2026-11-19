from dataclasses import dataclass, field
from typing import Dict, Any, List, Optional, Protocol, runtime_checkable
from datetime import datetime


@dataclass
class BlockTransaction:
    """
    区块交易：全项目统一使用的交易类型。

    字段直接来自 RPC getBlock 的返回值，只保留需要的部分：
    signatures / message / fee / preBalances / postBalances /
    instructions / logMessages / timestamp，额外加区块 slot。

    slot      区块 slot 号（= block 号），由调用方从文件名或 JSON 里解析后传入
    timestamp 区块 blockTime（Unix 秒）
    """
    slot: int
    timestamp: int
    signatures: List[str]
    message: Dict[str, Any]
    fee: int
    preBalances: List[int]
    postBalances: List[int]
    instructions: List[Dict[str, Any]]
    logMessages: List[str]

    @property
    def signature(self) -> str:
        """主签名（第一笔签名），作为交易的唯一标识"""
        return self.signatures[0] if self.signatures else ''

    def to_dict(self) -> Dict[str, Any]:
        """序列化为 dict，供 user client 发送给 server"""
        return {
            'slot': self.slot,
            'timestamp': self.timestamp,
            'signatures': self.signatures,
            'message': self.message,
            'fee': self.fee,
            'preBalances': self.preBalances,
            'postBalances': self.postBalances,
            'instructions': self.instructions,
            'logMessages': self.logMessages,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'BlockTransaction':
        """从 user client 发来的 dict 构造，缺失字段用默认值补齐"""
        return cls(
            slot=int(data['slot']),
            timestamp=data.get('timestamp'),
            signatures=data.get('signatures', []),
            message=data.get('message', {}),
            fee=data.get('fee', 0),
            preBalances=data.get('preBalances', []),
            postBalances=data.get('postBalances', []),
            instructions=data.get('instructions', []),
            logMessages=data.get('logMessages', []),
        )


@dataclass
class SlotLeader:
    """Slot领导者信息"""
    slot: int
    leader: str


@dataclass
class TransactionRecord:
    """交易发送记录"""
    signature: str
    slot: int                 # 交易原始 slot
    validator: str            # 实际发送到的 leader
    strategy: str
    sent_at: float = field(default_factory=lambda: datetime.now().timestamp())
    target_slot: Optional[int] = None   # 实际落地 slot（模拟推进到的 slot）
    delay_slots: int = 0                # target_slot - slot
    leader_risky: bool = False          # 落地 leader 是否为高风险


@runtime_checkable
class RoutableTransaction(Protocol):
    """路由链路依赖的交易最小接口。

    server 与策略只读取 slot 和 signature，不依赖具体交易类型。
    BlockTransaction 与 SandwichTransaction 都满足这个协议。
    """
    slot: int
    signature: str


@dataclass
class SandwichTransaction:
    """从 sandwiches_nobundle 的 victim 行构造的交易。

    字段直接对应 CK 表列；没有 BlockTransaction 需要的 fee / balances /
    instructions / message，因此是独立类型而不是 BlockTransaction 的子类。
    """
    slot: int
    signature: str
    timestamp: Optional[str] = None
    position: Optional[int] = None
    type: Optional[str] = None
    signer: Optional[str] = None
    accountKeys: List[str] = field(default_factory=list)
    consecutive: bool = False
    programs: List[str] = field(default_factory=list)
    inbundle: bool = False
    signerSame: bool = False
    crossBlock: bool = False
    oppositeVictim: bool = False
    fromToken: Optional[str] = None
    toToken: Optional[str] = None
    fromAmount: float = 0.0
    toAmount: float = 0.0
    fromDiff: float = 0.0
    toDiff: float = 0.0
