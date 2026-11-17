

import json
import numpy as np
from dataclasses import dataclass, field
from collections import Counter

import utils
from config import *
from .staker import *
from labels import is_protocol, get_validator_name


class ValidatorInfo:
    def __init__(self, details="", keybaseUserName="", name="", website="", iconUrl=""):
        self.details: str = details
        self.keybaseUserName: str = keybaseUserName
        self.name: str = name
        self.website: str = website
        self.iconUrl: str = iconUrl
    

class ValidatorStakewiz:
    def __init__(self):
        self.stakewiz: bool = False
        self.first_epoch_with_stake: int = 0
        self.s_ip_latitude: float = None
        self.s_ip_longitude: float = None
        self.s_ip_city: str =""
        self.s_ip_country: str = ""
        self.s_ip_asn: str = ""
        self.s_ip_org: str = ""
        self.s_is_jito: bool = None
        self.s_staking_apy: float = None
        self.s_jito_apy: float = None
        self.s_total_apy: float = None

class ValidatorMarinade:
    def __init__(self):
        self.marinade: bool = False
        self.m_node_ip: str = ""
        self.m_dc_lat: float = None
        self.m_dc_lon: float = None
        self.m_dc_continent: str = ""
        self.m_dc_country_iso: str = ""
        self.m_dc_country: str = ""
        self.m_dc_city: str = ""
        self.m_dc_asn: int = None
        self.m_dc_aso : str = None

        self.m_activated_stake: int = None


@dataclass
class Validator(ValidatorInfo, ValidatorStakewiz, ValidatorMarinade):
    valid: bool = field(init=False)

    identityPubkey: str
    voteAccountPubkey: str
    commission: int 
    epochCredits: int
    credits: int
    skipRate: float
    activatedStake: int
    version: str
    delinquent: bool
    stakes: list[Stake] = field(init=False)
    smap: dict[str, int] = field(init=False)

    def update_validator_info(self, info: ValidatorInfo):
        self.details = info.details
        self.keybaseUserName = info.keybaseUserName
        self.name = info.name
        self.website = info.website
        self.iconUrl = info.iconUrl
    def update_validator_stakewiz(self, stakewiz: ValidatorStakewiz) :
        self.stakewiz = stakewiz.stakewiz
        self.first_epoch_with_stake = stakewiz.first_epoch_with_stake
        self.s_ip_latitude = stakewiz.s_ip_latitude
        self.s_ip_longitude = stakewiz.s_ip_longitude
        self.s_ip_city = stakewiz.s_ip_city
        self.s_ip_country = stakewiz.s_ip_country
        self.s_ip_asn = stakewiz.s_ip_asn
        self.s_ip_org = stakewiz.s_ip_org
        self.s_is_jito = stakewiz.s_is_jito
        self.s_staking_apy = stakewiz.s_staking_apy
        self.s_jito_apy = stakewiz.s_jito_apy
        self.s_total_apy = stakewiz.s_total_apy
    def update_validator_marinade(self, marinade: ValidatorMarinade):
        self.marinade = marinade.marinade
        self.m_node_ip = marinade.m_node_ip
        self.m_dc_lat = marinade.m_dc_lat
        self.m_dc_lon = marinade.m_dc_lon
        self.m_dc_continent = marinade.m_dc_continent
        self.m_dc_country_iso = marinade.m_dc_country_iso
        self.m_dc_country = marinade.m_dc_country
        self.m_dc_city = marinade.m_dc_city
        self.m_dc_asn = marinade.m_dc_asn
        self.m_dc_aso = marinade.m_dc_aso

        self.m_activated_stake = marinade.m_activated_stake

    def update_stakes(self, epoch: int):
        self.stakes = []
        self.smap = Counter()

        epoch_path = os.path.join(EPOCHS_PATH, str(epoch))
        path = os.path.join(epoch_path, f"{self.voteAccountPubkey}.json")
        try:
            with open(path, 'r') as f:
                stakes_json = json.load(f)
        except Exception as e:
            return []
        stakes = []
        for s in stakes_json:
            s: dict
            stakes.append(Stake(
                s["staker"],
                s["withdrawer"],
                s["stakePubkey"],
                s["delegatedStake"],
                s["activationEpoch"],
                s.get("deactivationEpoch", 0),
                s.get("activeStake", 0),
                s.get("activatingStake", 0),
                self.voteAccountPubkey
                ))
            self.smap[s["staker"]] +=  s.get("activeStake", 0)
            
        self.stakes = stakes
    
    def identity(self) -> str:
        return self.voteAccountPubkey if self.name == "" else self.name

    def self_stake(self) -> int:
        return self.smap.get(self.identityPubkey, 0) + self.smap.get(self.voteAccountPubkey, 0)

    def protocol_stake(self, protocol: str) -> int:
        res = 0
        for s, amount in self.smap.items():
            if is_protocol(s, protocol):
                res += amount
        return res

    def validator_stake(self, epoch) -> int:
        res = 0
        for s, amount in self.smap.items():
            if "validator" in get_validator_name(s, epoch).tags:
                res += amount
        return res
    
    def max_stake(self) -> int:
        if len(self.smap) == 0:
            return 0
        return max(self.smap.values())

    def top_stakers(self, N=5):
        if not self.smap or len(self.smap) == 0:
            # return np.zeros(N, dtype=float)
            return [np.nan] * N
        sorted_items = sorted(self.smap.items(), key=lambda x: x[1], reverse=True)
        stakers = []
        for i in range(N):
            if i >= len(sorted_items):
                stakers.append(None)
                continue
            staker = sorted_items[i][0]
            
            if staker == self.identityPubkey or staker == self.voteAccountPubkey:
                stakers.append("self")
                continue

            addr_info =  get_pub_name(staker)
            if addr_info.alias != "":
                stakers.append(addr_info.alias)
            else:
                stakers.append("unknown")
        return stakers
            
        
    
    def top_staker(self):
        if len(self.smap) == 0:
            return None
        staker = max(self.smap, key=self.smap.get)
        addr_info =  get_pub_name(staker)
        if addr_info.alias != "":
            return addr_info.alias
        return "unknown"

    def top2_staker(self):
        if len(self.smap) < 2:
            return None 
        # 按 value 降序排序，取第二个
        sorted_items = sorted(self.smap.items(), key=lambda x: x[1], reverse=True)
        staker = sorted_items[1][0]
        addr_info =  get_pub_name(staker)
        if addr_info.alias != "":
            return addr_info.alias
        return "unknown"

        
        

    def stake(self, protocol: str = None, epoch: int=None, tosol=False, sumup=False):
        res = Counter()
        for s in self.stakes:
            if protocol != None and not is_protocol(s.staker, protocol):
                continue
            res[s.activationEpoch] += s.activeStake

        if tosol:
            for k in res.keys():
                res[k] = utils.tosol(res[k])
        if epoch == None and not sumup:
            return res
        if sumup:
            return sum(res.values())

        return res[epoch]
    
    def stake_gini(self):
        values = np.array(list(self.smap.values()))
        return utils.gini(values)


    def top_n_staker_ratio(self, N=5):
        if not self.smap or len(self.smap) == 0:
            return [np.nan] * N
        
        stakes = np.array(list(self.smap.values()), dtype=float)
        total = stakes.sum()
        if total == 0:
            return np.zeros(N, dtype=float)
        
        # 降序排序
        stakes_sorted = np.sort(stakes)[::-1]
        
        # 前 N 个质押者占比
        top_n = stakes_sorted[:N] / total
        for i in range(len(top_n)):
            top_n[i] = round(top_n[i], 6)
        
        # 不足 N 个质押者时填 0
        if len(top_n) < N:
            top_n = np.pad(top_n, (0, N - len(top_n)), 'constant')
        
        return top_n

    def staker_count_per_percentile(self, n_bins=5):
        if not self.smap or len(self.smap) == 0:
            return np.zeros(n_bins, dtype=int)
        
        stakes = np.array(list(self.smap.values()), dtype=float)
        total = stakes.sum()
        if total == 0:
            return np.zeros(n_bins, dtype=int)
        
        # 降序排序
        stakes_sorted = np.sort(stakes)[::-1]
        cumulative = np.cumsum(stakes_sorted) / total
        
        # 定义区间边界
        bins = np.linspace(0, 1, n_bins + 1)
        counts = np.zeros(n_bins, dtype=int)
        
        bin_idx = 0
        for i, cum_value in enumerate(cumulative):
            # 找到该质押者落在哪个区间
            while bin_idx < n_bins and cum_value > bins[bin_idx+1]:
                bin_idx += 1
            if bin_idx < n_bins:
                counts[bin_idx] += 1
        
        return counts

    def staker_cumulative_vector(self, percentiles=[0.2, 0.4, 0.6, 0.8]):
        """
        将 staker->stake_amount 字典转换为累积百分比向量
        percentiles: list, 需要计算的累积占比点
        返回长度为 len(percentiles) 的向量
        """
        if not self.smap or len(self.smap) == 0:
            return np.zeros(len(percentiles), dtype=float)
        
        stakes = np.array(list(self.smap.values()), dtype=float)
        total = stakes.sum()
        if total == 0:
            return np.zeros(len(percentiles), dtype=float)
        
        # 降序排序
        stakes_sorted = np.sort(stakes)[::-1]
        cumulative = np.cumsum(stakes_sorted) / total  # 累积占比
        
        vector = []
        for p in percentiles:
            # 找到第一个累积占比 >= p 的位置
            idx = np.searchsorted(cumulative, p, side='right')
            # 如果 idx 超出长度，则取最后一个
            idx = min(idx, len(stakes_sorted)-1)
            vector.append(stakes_sorted[idx])
        
        return np.array(vector, dtype=float)


        
    def stake_bucket_vector(self, M=5):
        if not self.smap or len(self.smap) == 0:
            return np.zeros(M, dtype=float)
        
        # 取所有质押量，并归一化到总质押量
        stakes = np.array(list(self.smap.values()), dtype=float)
        total = stakes.sum()
        if total > 0:
            stakes = stakes / total
        else:
            stakes = np.zeros_like(stakes)
        
        # 按质押量降序排序
        stakes_sorted = np.sort(stakes)[::-1]
        
        # 分桶
        bucket_size = int(np.ceil(len(stakes_sorted) / M))
        vector = []
        for i in range(M):
            start = i * bucket_size
            end = min((i+1) * bucket_size, len(stakes_sorted))
            if start >= len(stakes_sorted):
                vector.append(0.0)  # 没有元素就填 0
            else:
                bucket_sum = stakes_sorted[start:end].sum()
                vector.append(bucket_sum)
        
        return np.array(vector, dtype=float)


        
    
    def __str__(self):
        attrs = {
            k: v
            for cls in self.__class__.mro()
            for k, v in vars(self).items()
            if not k.startswith('_')
        }
        return f"{self.__class__.__name__}({attrs})"