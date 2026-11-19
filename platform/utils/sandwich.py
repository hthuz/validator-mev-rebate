
import clickhouse_connect

import numpy as np
from collections import defaultdict
from config import *

from .ch import query_clickhouse


class SlotInfo:
    def __init__(self):
        self.identity: str
        self.SlotNum: int = 0
        self.SandwichSlotNum: int = 0
        self.SandwichNum: int = 0
        self.SandwichSolProfit: float = 0

        self.TopSigner: str
        self.TopSignerPercent: float = 0
        self.TopSignerSandwichNum: int = 0
        self.TopSignerSolProfit: float = 0
    
    def __repr__(self):
        return f"SlotNum: {self.SlotNum}, SandwichSlotNum: {self.SandwichSlotNum}, SandwichNum: {self.SandwichNum}"

    def sandwich_ratio(self):
        if self.SlotNum != 0:
            return self.SandwichSlotNum / self.SlotNum
        return np.nan
        


def get_ch_client():
    client = clickhouse_connect.get_client(
        host=CLICKHOUSE_HOST,
        port=int(CLICKHOUSE_PORT),
        username=CLICKHOUSE_UESR,
        password=CLICKHOUSE_PASSWORD, 
        database=CLICKHOUSE_DBNAME
    )
    return client
    
# def get_slot_leader(epoch: int):
#     epoch_start = 432000 * epoch
#     epoch_end = epoch_start + 432000 - 1
#     query = f"""
#     SELECT leader, COUNT(DISTINCT slot)
#     FROM slot_leaders
#     WHERE slot between {epoch_start} and {epoch_end}
#     GROUP BY leader
#     """
    

# identity => slot info
def get_sandwich_info(epoch: int) -> dict[str, SlotInfo]:
    map = defaultdict(SlotInfo)
    epoch_start = 432000 * epoch
    epoch_end = epoch_start + 432000 - 1

    sandwich_block_query = f"""
    SELECT l.leader AS leader,
           COUNT(DISTINCT s.slot) AS sandwich_slot_num,
           COUNT(DISTINCT s.signature) AS sandwich_num
    FROM sandwiches_nobundle s JOIN slot_leaders l ON s.slot = l.slot
    WHERE s.slot between {epoch_start} and {epoch_end}
      and s.consecutive = true and s.type = 'backrun' and s.fromToken != ''
    GROUP BY l.leader
    """
    # sandwich_block_query = f"""
    # SELECT l.leader, COUNT(DISTINCT slot)
    # FROM sandwiches s JOIN slot_leaders l ON s.slot = l.slot
    # WHERE slot between {epoch_start} and {epoch_end} and signerSame = true and type = 'backrun'
    # GROUP BY l.leader
    # """

    for row in query_clickhouse(sandwich_block_query):
        leader = row['leader']
        map[leader].identity = leader
        map[leader].SandwichSlotNum = row['sandwich_slot_num']
        map[leader].SandwichNum = row['sandwich_num']

    slot_query = f"""
    SELECT leader, COUNT(DISTINCT slot) AS slot_num
    FROM slot_leaders
    WHERE slot between {epoch_start} and {epoch_end}
    GROUP BY leader
    """

    for row in query_clickhouse(slot_query):
        leader = row['leader']
        map[leader].identity = leader
        map[leader].SlotNum = row['slot_num']

    # # Sandwich SOL profit
    # profit_query = f"""
    # SELECT l.leader, SUM(fromDiff)
    # FROM sandwiches_nobundle s JOIN slot_leaders l ON s.slot = l.slot
    # WHERE slot between {epoch_start} and {epoch_end} and consecutive = true and type = 'backrun' and toToken='SOL'
    # GROUP BY l.leader
    # """

    # result = client.query(profit_query)
    # for row in result.result_rows:
    #     leader, profit = row
    #     map[leader].SandwichSolProfit = profit
    
    # top_signer_query = f"""
    #     WITH signer_counts AS (
    #     SELECT 
    #         l.leader AS validator,
    #         s.signer AS signer,
    #         COUNT(*) AS count
    #     FROM sandwiches_nobundle s JOIN slot_leaders l ON s.slot = l.slot
    #     WHERE slot between {epoch_start} and {epoch_end} and consecutive = true and type = 'backrun'
    #     GROUP BY validator, signer
    #     ),
    #     ranked_signers AS (
    #         SELECT 
    #             validator,
    #             signer,
    #             count,
    #             SUM(count) OVER (PARTITION BY validator) AS total_count,
    #             ROW_NUMBER() OVER (PARTITION BY validator ORDER BY count DESC) AS rank
    #         FROM signer_counts
    #     )
    #     SELECT 
    #         validator,
    #         signer AS most_common_signer,
    #         count AS signer_count,
    #         total_count AS total_signatures,
    #         ROUND(count * 100.0 / total_count, 2) AS percentage
    #     FROM ranked_signers
    #     WHERE rank = 1
    # """
    # result = client.query(top_signer_query)
    # for row in result.result_rows:
    #     leader, signer, count, total_count, percent = row
    #     map[leader].TopSigner = signer
    #     map[leader].TopSignerSandwichNum = count
    #     map[leader].TopSignerPercent = percent
    
    # top_signer_profit_query = f"""
    #     WITH signer_counts AS (
    #     SELECT 
    #         l.leader AS validator,
    #         s.signer AS signer,
    #         COUNT(*) AS count,
    #         SUM(s.fromDiff) AS profit
    #     FROM sandwiches_nobundle s JOIN slot_leaders l ON s.slot = l.slot
    #     WHERE slot between {epoch_start} and  {epoch_end} and consecutive = true and type = 'backrun' and toToken = 'SOL'
    #     GROUP BY validator, signer
    #     ),
    #     ranked_signers AS (
    #         SELECT 
    #             validator,
    #             signer,
    #             count,
    #             SUM(count) OVER (PARTITION BY validator) AS total_count,
    #             SUM(profit) OVER (PARTITION BY validator) AS total_profit,
    #             ROW_NUMBER() OVER (PARTITION BY validator ORDER BY count DESC) AS rank
    #         FROM signer_counts
    #     )
    #     SELECT 
    #         validator,
    #         total_profit as total_profit
    #     FROM ranked_signers
    #     WHERE rank = 1
    # """

    # result = client.query(top_signer_profit_query)
    # for row in result.result_rows:
    #     leader, profit = row
    #     map[leader].TopSigner = signer
    #     map[leader].TopSignerSolProfit = profit
    
    return map


def get_sandwich_ratios(epoch: int) -> dict[str, float]:
    """获取指定 epoch 各 validator 的 sandwich ratio。

    ratio 的计算完全复用 SlotInfo.sandwich_ratio()，即
    SandwichSlotNum / SlotNum；没有出过块的 validator 返回 nan。
    """
    info = get_sandwich_info(epoch)
    return {
        identity: slot_info.sandwich_ratio()
        for identity, slot_info in info.items()
    }



def get_victim_transactions(epoch: int) -> list:
    """获取指定 epoch 内的 victim 交易，按 signature 去重。

    同一个 victim 可能出现在多个 sandwich 里，这里只保留一条；
    slot / 交易字段用 any() 取值。计算口径与 get_sandwich_info 共用 epoch 边界。
    """
    # 延迟导入避免 utils -> type.validator -> utils 的循环依赖
    from type.models import SandwichTransaction

    epoch_start = 432000 * epoch
    epoch_end = epoch_start + 432000 - 1

    sql = f"""
    SELECT slot, signature, position, signer, accountKeys, consecutive, programs,
           inbundle, signerSame, crossBlock, oppositeVictim,
           fromToken, toToken, fromAmount, toAmount, fromDiff, toDiff, timestamp
    FROM sandwiches_nobundle
    WHERE slot BETWEEN {epoch_start} AND {epoch_end} AND type = 'victim'
    """

    transactions = []
    seen = set()
    for row in query_clickhouse(sql):
        signature = row['signature']
        if signature in seen:
            continue
        seen.add(signature)
        data = dict(row)
        data['type'] = 'victim'
        transactions.append(SandwichTransaction(**data))
    return transactions


def export_to_csv(epoch: int) -> None:
    """导出指定 epoch 的 sandwich ratio 到 CSV 文件"""

    output_path = f"sandwich_ratios_epoch_{epoch}.csv"
    info = get_sandwich_info(epoch)
    with open(output_path, 'w') as f:
        f.write("validator,slot_num,sandwich_slot_num,sandwich_num,sandwich_ratio\n")
        for identity, slot_info in info.items():
            f.write(f"{identity},{slot_info.SlotNum},{slot_info.SandwichSlotNum},{slot_info.SandwichNum},{slot_info.sandwich_ratio()}\n")

if __name__ == "__main__":

    export_to_csv(800)
    # map = get_sandwich_info(801)

    # sort map by sandwich ratio
    # sorted_map = dict(sorted(map.items(), key=lambda item: item[1].sandwich_ratio(), reverse=True))

    # print sorted map top 100
    # for k, v in list(sorted_map.items())[:100]:
    #     print(k,v, v.sandwich_ratio())

    # for k, v in map.items():
    #     print(k,v)

        