
from dataclasses import dataclass

from labels import get_pub_name

@dataclass
class Stake:
    staker: str
    withdrawer: str
    stakePubkey: str
    delegatedStake: int
    activationEpoch: int
    deactivationEpoch: int
    activeStake: int
    activatingStake: int
    voteAccountPubkey: str

@dataclass
class Staker:
    addr: str
    totalStake: int
    stakes: list[Stake] # a staker may stake to same validator in different epochs
    vmap: dict[str,int] # validator => total stake

    def identity(self) -> str:
        addr_info = get_pub_name(self.addr)
        return self.addr if addr_info.alias == "" else addr_info.alias
        # return self.addr if addr_info.pubname == "" else addr_info.pubname