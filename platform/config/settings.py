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

# Mitigation: 起始 slot 号（slot == block），每次推进 +1
START_SLOT = int(os.getenv("START_SLOT", "346769956"))

# Mode settings
DEBUG_MODE = os.getenv("DEBUG_MODE", "False").lower() == "true"

# Server settings (for mitigation and other services)
SERVER_HOST = os.getenv("SERVER_HOST", "0.0.0.0")
SERVER_PORT = int(os.getenv("SERVER_PORT", "5000"))
