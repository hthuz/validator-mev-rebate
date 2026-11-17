

import json
import os

class AddrInfo:
    def __init__(self, pubname="", alias="", *tags):
        self.pubname: str = pubname
        self.alias: str= alias
        self.tags: list[str] = []
        for i, tag in enumerate(tags):
            self.tags.append(tag)
    def __repr__(self):
        return f"{self.pubname} ({self.alias}) {self.tags}"


def get_pub_name(addr: str) -> AddrInfo:
    if addr in ScanPubNames.keys():
        return ScanPubNames[addr]
    if addr in SolanaFmTagged.keys():
        return SolanaFmTagged[addr]
    return AddrInfo()
    

def is_protocol(addr: str, protocol: str) -> bool:
    if addr not in ScanPubNames.keys():
        return False
    return ScanPubNames[addr].alias == protocol

def protocol_addrs(addrs: list[str], protocol: str) -> list[str]:
    return [addr for addr in addrs if is_protocol(addr, protocol)]
            


# top one hundred of an epoch by count and by stake num
# record at 2025-5-20
ScanPubNames = {
    "UefNb6z6yvArqe4cJHTXCqStRsKmWhGxnZzuHbikP5Q": AddrInfo("Marinade SOL Leg Account", "marinade"),
    "EyaSjUtSgo9aRD1f8LWXwdvkpDTmXAW54yoSHZRF14WL": AddrInfo("Marinade (mSOL) Leg Authority", "marinade"),
    "B1aLzaNMeFVAyQ6f3XbbUyKcH2YPHu2fqiEagmiF23VR": AddrInfo("Marinade Treasury (mSOL) Vault", "marinade"),
    "Du3Ysj1wKbxPKkuPPnvzQLQh8oMSVifs3jGZjJWXFmHN": AddrInfo("Marinade Reserve (SOL) PDA", "marinade"),
    "7GgPYjS5Dza89wV6FpZ23kUJRG5vbQ1GM25ezspYFSoE": AddrInfo("Marinade (mSOL) Leg Account", "marinade"),
    "89SrbjbuNyqSqAALKBsKBqMSh463eLvzS4iVWCeArBgB": AddrInfo("Marinade Treasury Vault Authority", "marinade", "token creator"),
    "opNS8ENpEMWdXcJUgJCsJTDp7arTXayoBEeBUg6UezP": AddrInfo("Marinade: Native Staking Operator", "marinade"),
    "opLSF7LdfyWNBby5o6FT8UFsr2A4UGKteECgtLSYrSm": AddrInfo("Marinade Finance Operation Sol", "marinade"),
    "VaLV7StdK6A3KfqyePbHyaBefjeibq9gakECFtXNM4m": AddrInfo("Marinade Validator Manager Authority", "marinade"),
    "J5BEceL5z1EQ7JBqEFu4BfPN4PYCeQaW3GXrzXFfCzhs": AddrInfo("Marinade Treasury #2", "marinade"),
    "B56RWQGf9RFw7t8gxPzrRvk5VRmB5DoF94aLoJ25YtvG": AddrInfo("Marinade Treasury #1", "marinade"),
    "7cgg6KhPd1G8oaoB48RyPDWu7uZs51jUpDYB3eq4VebH": AddrInfo("Bond Stake Authority", "marinade"),
    "8ZUcztoAEhpAeC2ixWewJKQJsSUGYSGPVAjkhDJYf5Gd": AddrInfo("Marinade Treasury (mSOL) Account", "marinade"),
    "9eG63CdHjsfhHmobHgLtESGC8GabbmRcaSpHAZrtmhco": AddrInfo("Marinade (mSOL) Stake Pool Withdraw Authority", "marinade"),
    # "4bZ6o3eUUNXhKuqjdCnCoPAoLgWiuLYixKaxoa8PpiKk": AddrInfo("Marinade (mSOL) Liquid Stake Pool Authority", "marinade pool"),
    "4bZ6o3eUUNXhKuqjdCnCoPAoLgWiuLYixKaxoa8PpiKk": AddrInfo("Marinade (mSOL) Liquid Stake Pool Authority", "marinade"),
    "ex9CfkBZZd6Nv9XdnoDmmB45ymbu4arXVk7g5pWnt3N": AddrInfo("Marinade Native 1", "marinade"),
    "stWirqFCf2Uts1JBL1Jsd3r6VBWhgnpdPxCTe1MFjrq": AddrInfo("Marinade Native 2", "marinade"),
    "3JLPCS1qM2zRw3Dp6V4hZnYHd4toMNPkNesXdX9tg6KM": AddrInfo("Marinade (mSOL) Mint Authority", "marinade"),
    "HZsepB79dnpvH6qfVgvMpS738EndHw3qSHo4Gv5WX1KA": AddrInfo("Marinade Lp Mint Authority", "marinade"),
    "DwFYJNnhLmw19FBTrVaLWZ8SZJpxdPoSYVSJaio9tjbY": AddrInfo("Marinade Validator List State", "marinade"),

    "3rBnnH9TTgd3xwu48rnzGsaQkSr1hR64nY71DrDt6VrQ": AddrInfo("Sanctum Pool (SOL) Reserves", "sanctum"),
    "mpa4abUkjQoAvPzREkh5Mo75hZhPFQ2FSH6w7dWKuQ5": AddrInfo("Solana Foundation Delegation", "SFDP"),
    "6WecYymEARvjG5ZyqkrVQ6YkhPfujNzWpSPwNKXHCbV2": AddrInfo("BlazeStake (bSOL) Stake Pool Withdraw Authority", "blazestake", "stake pool", "state account"),
    "6iQKfEyhr3bZMotVkW6beNZz5CPAkiwvgV2CTje9pVSS": AddrInfo("Jito (JitoSOL) Stake Pool Withdraw Authority", "jito", "stake pool", "state account"),
    "7TdbUsGdmK2PfyLYQmnvJJcK6xWrs2AFtHjc575WRsmW": AddrInfo("Stake Pool Withdraw Authority", "", "stake pool"),
    "3b7XQeZ8nSMyjcQGTFJS5kBw4pXS2SqtB9ooHCnF2xV9": AddrInfo("Liquid (LST) Stake Pool Withdraw Authority", "liquid", "stake pool", "state account"),
    "HbJTxftxnXgpePCshA8FubsRj9MW4kfPscfuUfn44fnt": AddrInfo("JPool (JSOL) Stake Pool Withdraw Authority", "jpool", "stake pool", "state account"),
    "GdNXJobf8fbTR5JSE7adxa6niaygjx4EEbnnRaDCHMMW": AddrInfo("The Vault (vSOL) Stake Pool Withdraw Authority", "thevault", "stake pool", "state account"),
    "9Lfnu9kR87CunquitZhqLSFUym6etNmj2vhFb8HYgh57": AddrInfo("", "", "token creator"),
    "FZEaZMmrRC3PDPFMzqooKLS2JjoyVkKNd2MkHjr7Xvyq": AddrInfo("Edgevana (edgeSOL) Stake Pool Withdraw Authority", "edgevana", "stake pool", "state account"),
    "AKJt3m2xJ6ANda9adBGqb5BMrheKJSwxyCfYkLuZNmjn": AddrInfo("(aeroSOL) Stake Pool Withdraw Authority", "areoSOL", "stake pool", "pool owner"),
    "EpH4ZKSeViL5qAHA9QANYVHxdmuzbUH2T79f32DmSCaM": AddrInfo("Shinobi Performance (xSHIN) Stake Pool Withdraw Authority", "shinobi", "stake pool", "state account"),
    "F3VpnAqPfSbp7T3DUa8BtoUE13F9513vN6T8Yev2BxK1": AddrInfo("", "", "token creator", "pump.fun"),
    "59L2oxymiQQ9Hvhh92nt8Y7nDYjsauFkdb3SybdnsG6h": AddrInfo("Coinbase 5", "coinbase", "exchange wallet"),
    "3etKXcW2fzEJR5YXoSKSmP6UZ633g9uiFv5yuqFUf66k": AddrInfo("Socean Stake Pool", "socean", "validator"),
    "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM": AddrInfo("Binance 3", "binance", "exchange wallet"),
    "75NPzpxoh8sXGuSENFMREidq6FMzEx4g2AfcBEB6qjCV": AddrInfo("Binance Staked SOL (BNSOL) Pool Withdraw Authority", "binance stake pool"),
    "H4yiPhdSsmSMJTznXzmZvdqWuhxDRzzkoQMEWXZ6agFZ": AddrInfo("FTX/Alameda Staking Address", "alameda"),
    "EMjuABxELpYWYEwjkKmQKBNCwdaFAy4QYAs6W9bDQDNw": AddrInfo("Jupiter (JupSOL) Stake Pool Withdraw Authority", "jupiter", "stake pool", "pool owner"),
    "HzKVUmEAuaf8nV3tcJk2uZKohmLwtk1351ASCdqT5B8q": AddrInfo("", "", "kraken exchange", "deposit address"),
    "6727ZvQ2YEz8jky1Z9fqDFG5mYuAvC9G34o2MxwzmrUK": AddrInfo("Drift (dSOL) Stake Pool Withdraw Authority", "drift", "stake pool", "pool owner"),
    "2rMuGTyXCqCHZBSu6NZR9Aq8MhZX9gLkCHoQsPhSj2YF": AddrInfo("Helius (hSOL) Stake Pool Withdraw Authority", "helius", "stake pool", "pool owner"),
    "H5rmot8ejBUWzMPt6E44h27xj5obbSz3jVuK4AsJpHmv": AddrInfo("Solayer (LP-SOLAYER) Stake Pool Withdraw Authority", "solayer", "stake pool", "stake account"),
    "4GC3a1RkRXx5shwGP8pTY6cxXgSWkbfc66vM53a6qSKj": AddrInfo("CoinSpot Hot Wallet", "", "exchange wallet"),
    "FyJBKcfcEBzGN74uNxZ95GxnCxeuJJujQCELpPv14ZfN": AddrInfo("Bitfinex Cold Wallet", "", "exchange wallet"),

    # Not in Top 100 but solana fm labeled
    "6b4aypBhH337qSzzkbeoHWzTLt4DjG2aG8GkrrTQJfQA": AddrInfo("FTX Cold Storage#1", "ftx", "exchange wallet"),
    # This address is reported differently by solanafm
    "9uyDy9VDBw4K7xoSkhmCAm8NAFCwu4pkF6JeHUCtVKcX": AddrInfo("FTX Cold Storage#2", "ftx", "exchange wallet"),
    "FyJBKcfcEBzGN74uNxZ95GxnCxeuJJujQCELpPv14ZfN": AddrInfo("Bitfinex Cold Wallet", "bitfinex", "exchange wallet"),
    "5tgfd6XgwiXB9otEnzFpXK11m7Q7yZUaAJzWK4oT5UGF": AddrInfo("Mango DAO", ""),

    # not in top 100 but added later
    "FZEaZMmrRC3PDPFMzqooKLS2JjoyVkKNd2MkHjr7Xvyq": AddrInfo("Edgevana (edgeSOL) Stake Pool Withdraw Ahority", "edgevana", "stake pool", "state account")
}


# 149 accouts labeled by solanafm
# 16 are not validators
# 8 are unique to solanafm

SolanaFmTagged = {
    "2FUvfvcuEJi8p1UYZi62WvpeyGwMjD1RZdEyqWUS42Kg": AddrInfo("Dropp Lands Treasury", "Dropp Lands Treasury", "nft", "nft project"),
    "Bq6NnW55ToJrAM7NxUYDmSMbKKCQ6f742ZemLiypqjzY": AddrInfo("CoinDCX Exchange", "", "exchange wallet"),
    "EhZLSqyWrazhqR5bmyyEfFRa3ycWu9KPgQgQ7G1R9zC9": AddrInfo("CoinDXC Exchange", "", "exchange wallet"),
    "BwgpxJAFX9wtZSWxp3JUd65aky6a6N8F5xMqBfA39ohL": AddrInfo("Solana Money Boys Treasury", "", "nft", "nft project"),
    "HHm4wK91XvL3hhEC4hQHo544rtvkaKohQPc59TvZeC71": AddrInfo("Cypher Exploiter Wallet", "", "fm_hacker"),
    "Ggut9tjWyAz2Q3q2FuAi19DEtXBtX9E3a6wFSY1oaJLA": AddrInfo("Malicious Wallet (Phishing)", ""),
    "pEsKYABNARLiDFYrjbjHDieD5h6gHrvYf9Vru62NX9k": AddrInfo("Pesky Penguins Treasury v1", "", "nft", "nft project"),
    "ppTeamTpw1cbC8ybJpppbnoL7xXD9froJNFb5uvoPvb": AddrInfo("Pesky Penguins Treasury v2", "", "nft", "nft project")
}

VoteAccounts = {}

IdentityAccounts = {
    
}

def get_validator_name(addr: str, epoch: int) -> AddrInfo:
    global VoteAccounts
    global IdentityAccounts
    
    from config import EPOCHS_PATH
    # as cache
    if len(VoteAccounts) == 0 and len(IdentityAccounts) == 0:
        validators_path = os.path.join(EPOCHS_PATH, str(epoch), "validators.json")
        validator_info_path = os.path.join(EPOCHS_PATH, str(epoch), "validator_info.json")
        
        with open(validators_path, "r") as f:
            json_data = json.load(f)
        
        vote_identity = {}
        for v in json_data["validators"]:
            vote_identity[v["voteAccountPubkey"]] = v["identityPubkey"]

        with open(validator_info_path, "r") as f:
            json_data = json.load(f)
        for data in json_data:
            identity = data["identityPubkey"]
            name = data["info"].get("name", "")
            IdentityAccounts[identity] = AddrInfo(name, name, "validator")

        for vote, identity in vote_identity.items():
            if identity not in IdentityAccounts.keys():
                IdentityAccounts[identity] = AddrInfo("", "", "validator")
            VoteAccounts[vote] = IdentityAccounts[identity]
    
    if addr in VoteAccounts.keys():
        return VoteAccounts[addr]
    if addr in IdentityAccounts.keys():
        return IdentityAccounts[addr]
    return AddrInfo()