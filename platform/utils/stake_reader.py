import json
import os
import pandas as pd
from collections import defaultdict

from type import Validator, Stake, Staker, ValidatorInfo, ValidatorStakewiz, ValidatorMarinade
from config import *
from utils import obj_to_dict, find_match_path
import utils


class Reader:
    staker_identifer = "staker"

    def __init__(self, epoch=800):
        self.epoch = epoch
        self.epoch_path = os.path.join(EPOCHS_PATH, str(self.epoch))
        self.validators_path = os.path.join(self.epoch_path, "validators.json")
        self.validator_info_path = os.path.join(self.epoch_path, "validator_info.json")
        self.stakewiz_path = os.path.join(self.epoch_path, "stakewiz.json")
        auction_path = os.path.join(SAM_PATH, "auctions")
        self.marinade_path = find_match_path(auction_path, epoch)
        pass

    def get_stakewizs(self) -> dict[str, ValidatorStakewiz]:
        stakewizs = dict()
        try:
            with open(self.stakewiz_path, "r") as f:
                json_data = json.load(f)
            for data in json_data:
                s = ValidatorStakewiz()
                s.stakewiz = True
                s.first_epoch_with_stake = data.get("first_epoch_with_stake", 0)

                latitude = data.get("ip_latitude", "")
                s.s_ip_latitude = float(latitude) if latitude != None else None
                longitude = data.get("ip_longitude", "")
                s.s_ip_longitude = float(longitude) if longitude != None else None

                s.s_ip_city = data.get("ip_city", "")
                s.s_ip_country = data.get("ip_country", "")
                s.s_ip_asn = data.get("ip_asn", "")
                s.s_ip_org = data.get("ip_org", "")
                s.s_is_jito = data.get("is_jito", None)
                s.s_staking_apy = data.get("staking_apy", None)
                s.s_jito_apy = data.get("jito_apy", None)
                s.s_total_apy = data.get("total_apy", None)
                stakewizs[data["vote_identity"]] = s
            print("Num of validators in stakewiz",len(stakewizs), flush=True)
        except:
            print("Warn: stakewiz file not loaded", flush=True)
        return stakewizs
        
    def get_marinades(self) -> dict[str, ValidatorMarinade]:
        # marinade
        marinades = dict()
        try:
            with open(os.path.join(self.marinade_path, "inputs", "validators.json")) as f:
                json_data = json.load(f)
            for data in json_data["validators"]:
                m = ValidatorMarinade()
                m.marinade = True
                m.m_dc_asn = data.get("dc_asn", "")
                m.m_dc_aso = data.get("dc_aso", "")
                m.m_dc_city = data.get("dc_city", "")
                m.m_dc_continent = data.get("dc_continent", "")
                m.m_dc_country = data.get("dc_country", "")
                m.m_dc_country_iso = data.get("dc_country_iso", "")
                m.m_dc_lat = data.get("dc_coordinates_lat", None)
                m.m_dc_lon = data.get("dc_coordinates_lon", None)
                m.m_node_ip = data.get("node_ip")
                m.m_activated_stake = int(data.get("activated_stake"))
                marinades[data["vote_account"]] = m
            print("Num of validators in marinades",len(marinades), flush=True)
        except:
            print("Warn: marinade ip file not loaded", flush=True)
        return marinades
        
    def get_validators(self, with_stake: bool = False, validator: str = None):
        with open(self.validators_path, "r") as f:
            json_data = json.load(f)
        validators: dict[str, Validator] = {}
        for v in json_data["validators"]:
            validators[v["voteAccountPubkey"]] = Validator(
                v["identityPubkey"],
                v["voteAccountPubkey"],
                v["commission"],
                v["epochCredits"],
                v["credits"],
                v["skipRate"],
                v["activatedStake"],
                v["version"],
                v["delinquent"],
            )

        # validator info
        with open(self.validator_info_path, "r") as f:
            json_data = json.load(f)
        infos = dict()
        for data in json_data:
            data: dict
            infos[data["identityPubkey"]] = ValidatorInfo(
                data["info"].get("details", ""),
                data["info"].get("keybaseUserName", ""),
                data["info"]["name"],
                data["info"].get("website", ""),
                data["info"].get("iconUrl", ""),
            )

        stakewizs = self.get_stakewizs()
        marinades = self.get_marinades()

        for v in validators.values():
            v.valid = os.path.exists(os.path.join(self.epoch_path, f"{v.voteAccountPubkey}.json"))

            v.update_validator_info(infos.get(v.identityPubkey, ValidatorInfo()))
            v.update_validator_stakewiz(
                stakewizs.get(v.voteAccountPubkey, ValidatorStakewiz())
            )
            v.update_validator_marinade(
                marinades.get(v.voteAccountPubkey, ValidatorMarinade())
            )
            if with_stake and validator == None:
                v.update_stakes(self.epoch)

        if with_stake and validator != None:
            validators[validator].update_stakes(self.epoch)
            return validators[validator]

        # In stakewiz set but not in validators set
        # stakewiz_set = set(stakewizs.keys())
        # validators_set = set(validators.keys())
        # diff = stakewiz_set.difference(validators_set)
        # for d in diff:
        #     print(d)
        
        # in validators info set but not in validators set
        # infos_set = set(infos.keys())
        # validators_set = set(v.identityPubkey for v in validators.values() )
        # diff = infos_set.difference(validators_set)
        # for d in diff:
        #     print(d)

        return validators

    def get_stakers(self) -> dict[str, Staker]:
        validators = self.get_validators(with_stake=True)
        staker_info: defaultdict[str, list[Stake]] = (
            defaultdict()
        )  # staker => all stakes of this staker
        for v in validators.values():
            for stake in v.stakes:
                if self.staker_identifer == "staker":
                    staker_info.setdefault(stake.staker, [])
                    staker_info[stake.staker].append(stake)
                if self.staker_identifer == "withdrawer":
                    staker_info.setdefault(stake.withdrawer, [])
                    staker_info[stake.withdrawer].append(stake)

        stakers: list[Staker] = []
        stakers: dict[str, Staker] = {}
        for addr, stakes in staker_info.items():
            vmap: dict[str, int] = {}
            totalStake = 0
            for stake in stakes:
                s = vmap.get(stake.voteAccountPubkey, 0)
                totalStake += stake.activeStake
                vmap[stake.voteAccountPubkey] = s + stake.activeStake

            stakers[addr] = Staker(addr, totalStake, stakes, vmap)

        return stakers

    # validator with valid stake response
    def valid_filter(self, validators: list[Validator]) -> list[Validator]:
        return [
            v
            for v in validators
            if os.path.exists(
                os.path.join(self.epoch_path, f"{v.voteAccountPubkey}.json")
            )
        ]

    def check(self):
        validators = self.get_validators(with_stake=True).values()
        active_validators = [v for v in validators if v.activatedStake > 0]
        nondelinquent = [v for v in validators if not v.delinquent]
        print(f"total validators", len(validators), flush=True)
        print(f"active validators (active stake > 0)", len(active_validators), flush=True)
        print(f"nondelinquent validators", len(nondelinquent), flush=True)
        print(f"total active stake", sum(v.activatedStake for v in validators), flush=True)

        error_validators = []
        valid_validators = []
        files = os.listdir(self.epoch_path)
        for f in files:
            f: str
            if f == "validators.json" or f == "validator_info.json":
                continue
            if f.startswith("ERROR"):
                error_validators.append(f[:-5])
                continue
            valid_validators.append(f[:-5])

        print("error validator count", len(error_validators), flush=True)
        print("valid validator count", len(valid_validators), flush=True)
        print(
            "total accessed validator count",
            len(error_validators) + len(valid_validators), flush=True
        )

        total_activated_stake = 0
        total_activating_stake = 0
        total_delegated_stake = 0
        stakewiz = 0
        mismatch = 0
        for v in validators:
            sum_activeStake = sum(s.activeStake for s in v.stakes)
            sum_activatingStake = sum(s.activatingStake for s in v.stakes)
            sum_delegatedStake = sum(s.delegatedStake for s in v.stakes)

            if sum_activeStake != v.activatedStake:
                mismatch += 1
                print(
                    f"{v.voteAccountPubkey}, activatedStake {v.activatedStake}, computed {sum_activeStake}", flush=True
                )

            total_activated_stake += sum_activeStake
            total_activating_stake += sum_activatingStake
            total_delegated_stake += sum_delegatedStake
            stakewiz += v.stakewiz 

        print("mismatch", mismatch, flush=True)
        print("computed activated stake", total_activated_stake, flush=True)
        print("computed activating stake", total_activating_stake, flush=True)
        print(
            "computed activated + activating stake",
            total_activated_stake + total_activating_stake,flush=True
        )
        print("computed delegated stake", total_delegated_stake, flush=True)
        print("Num of validators recorded by stakewiz", stakewiz, flush=True)

    def tocsv(self):
        validators = self.get_validators(with_stake=True).values()

        validators = self.valid_filter(validators)
        df = pd.DataFrame(obj_to_dict(obj) for obj in validators)
        # df = df.sort_values(by="activatedStake", ascending=False)
        df.to_csv("validators.csv", index=False)
        # df.to_csv("withdrawer.csv", index=False)
    
    def sfdp_min_date(self):
        stakers = self.get_stakers()
        sfdp = stakers["mpa4abUkjQoAvPzREkh5Mo75hZhPFQ2FSH6w7dWKuQ5"]
        sfdp.stakes.sort(key=lambda s: s.activationEpoch)
        print(sfdp.stakes[:10])


if __name__ == "__main__":
    epoch = 800
    r = Reader(epoch)
    r.check()