"""
链上协议常量。

这些是 Solana vote program 的固定标识，不是可配置项，
集中放在这里供各模块共用，避免魔法字符串散落。
"""

# vote 交易恰好只有一条 instruction
VOTE_INSTRUCTION_COUNT = 1

# instruction 交互的 program 名
VOTE_PROGRAM = 'vote'

# vote program 的 on-chain id
VOTE_PROGRAM_ID = 'Vote111111111111111111111111111111111111111'

# jsonParsed 编码下 instruction 的解析类型
VOTE_INSTRUCTION_TYPE = 'towersync'

__all__ = [
    'VOTE_INSTRUCTION_COUNT',
    'VOTE_PROGRAM',
    'VOTE_PROGRAM_ID',
    'VOTE_INSTRUCTION_TYPE',
]
