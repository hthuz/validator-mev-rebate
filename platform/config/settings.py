"""Platform configuration settings"""

from dotenv import load_dotenv
import os
from pathlib import Path

# Load .env from project root
root_dir = Path(__file__).parent.parent
load_dotenv(root_dir / ".env")

# Solana related
SOLANA_BIN = os.getenv("SOLANA_BIN")
EPOCHS_PATH = os.getenv("EPOCHS_PATH")
COLLECT_WORKER = int(os.getenv("COLLECT_WORKER", "1"))
SAM_PATH = os.getenv("SAM_PATH")

# Database
MYSQL_ENGINE = os.getenv("MYSQL_ENGINE")
CLICKHOUSE_HOST = os.getenv("CLICKHOUSE_HOST")
CLICKHOUSE_PORT = os.getenv("CLICKHOUSE_PORT")
CLICKHOUSE_UESR = os.getenv("CLICKHOUSE_UESR")
CLICKHOUSE_PASSWORD = os.getenv("CLICKHOUSE_PASSWORD")
CLICKHOUSE_DBNAME = os.getenv("CLICKHOUSE_DBNAME")

# Data paths
SANDWICH_PATH = os.getenv("SANDWICH_PATH")
BLOCK_PATH = os.getenv("BLOCK_PATH")
SLOT_LEADER_PATH = os.getenv("SLOT_LEADER_PATH")

# Mitigation: slot 范围（inclusive，slot == block），每次推进 +1
MITIGATION_START_SLOT = int(os.getenv("MITIGATION_START_SLOT", "346769956"))
MITIGATION_END_SLOT = int(os.getenv("MITIGATION_END_SLOT", "346769956"))

# Mitigation: replay 数据源，block = 原始区块交易，victim = sandwich victim 交易
MITIGATION_TX_SOURCE = os.getenv("MITIGATION_TX_SOURCE", "block")
# victim 模式取哪个 epoch 的 victim 交易（slot 范围仍是 MITIGATION_START_SLOT / END_SLOT）
MITIGATION_REPLAY_EPOCH = int(os.getenv("MITIGATION_REPLAY_EPOCH", "802"))

# Mitigation: 发送策略，可选 immediate / delayed / risk-aware
MITIGATION_STRATEGY = os.getenv("MITIGATION_STRATEGY", "immediate")
# DelayedStrategy 的延迟 slot 数
MITIGATION_DELAY_SLOTS = int(os.getenv("MITIGATION_DELAY_SLOTS", "1"))

# Mitigation: 历史 sandwich ratio 数据范围（inclusive，用于判断 leader 是否需要跳过）
SANDWICH_HISTORY_START_EPOCH = int(os.getenv("SANDWICH_HISTORY_START_EPOCH", "800"))
SANDWICH_HISTORY_END_EPOCH = int(os.getenv("SANDWICH_HISTORY_END_EPOCH", "801"))
# 任意一个历史 epoch 的 ratio 严格超过该阈值，即认为该 validator 高风险
SANDWICH_RATIO_THRESHOLD = float(os.getenv("SANDWICH_RATIO_THRESHOLD", "0.5"))
# 连续跳过高风险 leader 的最大 slot 窗口；达到后无论 leader 风险如何都强制发送
MITIGATION_MAX_SKIP_SLOTS = int(os.getenv("MITIGATION_MAX_SKIP_SLOTS", "10"))

# Mode settings
DEBUG_MODE = os.getenv("DEBUG_MODE", "False").lower() == "true"

# Server settings (for mitigation and other services)
SERVER_HOST = os.getenv("SERVER_HOST", "0.0.0.0")
SERVER_PORT = int(os.getenv("SERVER_PORT", "5000"))
