
import clickhouse_connect

import numpy as np
from collections import defaultdict
from config import *


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
    client = get_ch_client()
    map = defaultdict(SlotInfo)
    epoch_start = 432000 * epoch
    epoch_end = epoch_start + 432000 - 1

    sandwich_block_query = f"""
    SELECT l.leader, COUNT(DISTINCT slot), COUNT(DISTINCT signature)
    FROM sandwiches_nobundle s JOIN slot_leaders l ON s.slot = l.slot
    WHERE slot between {epoch_start} and {epoch_end} and consecutive = true and type = 'backrun' and fromToken != ''
    GROUP BY l.leader
    """
    # sandwich_block_query = f"""
    # SELECT l.leader, COUNT(DISTINCT slot)
    # FROM sandwiches s JOIN slot_leaders l ON s.slot = l.slot
    # WHERE slot between {epoch_start} and {epoch_end} and signerSame = true and type = 'backrun'
    # GROUP BY l.leader
    # """

    result = client.query(sandwich_block_query)
    for row in result.result_rows:
        leader, sandwich_slot_num, sandwich_num = row
        map[leader].identity = leader
        map[leader].SandwichSlotNum = sandwich_slot_num
        map[leader].SandwichNum = sandwich_num

    slot_query = f"""
    SELECT leader, COUNT(DISTINCT slot)
    FROM slot_leaders
    WHERE slot between {epoch_start} and {epoch_end}
    GROUP BY leader
    """

    result = client.query(slot_query)
    for row in result.result_rows:
        leader, slot_num = row
        map[leader].identity = leader
        map[leader].SlotNum = slot_num

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

if __name__ == "__main__":
    map = get_sandwich_info(801)

    # sort map by sandwich ratio
    sorted_map = dict(sorted(map.items(), key=lambda item: item[1].sandwich_ratio(), reverse=True))

    # print sorted map top 100
    for k, v in list(sorted_map.items())[:100]:
        print(k,v, v.sandwich_ratio())

    # for k, v in map.items():
    #     print(k,v)

        