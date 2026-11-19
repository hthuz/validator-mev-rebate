"""ClickHouse 查询封装。

本环境的 8123 端点只接受 URL 上的 query 参数，clickhouse_connect 1.13.0
用 POST body 传 query 会拿到 404。这里统一走 requests + query 参数，
返回 dict 列表，供 ratio / victim / slot leader 查询共用。
"""
import json
from typing import Any, Dict, List

import requests

from config import (
    CLICKHOUSE_HOST,
    CLICKHOUSE_PORT,
    CLICKHOUSE_PASSWORD,
    CLICKHOUSE_UESR,
    CLICKHOUSE_DBNAME,
)


def query_clickhouse(sql: str, timeout: int = 120) -> List[Dict[str, Any]]:
    """执行 SQL 并返回 JSONEachRow 解析后的 dict 列表。SQL 里不要自带 FORMAT。"""
    url = f"http://{CLICKHOUSE_HOST}:{CLICKHOUSE_PORT}/"
    response = requests.post(
        url,
        params={
            'database': CLICKHOUSE_DBNAME,
            'query': f"{sql.rstrip().rstrip(';')} FORMAT JSONEachRow",
        },
        auth=(CLICKHOUSE_UESR, CLICKHOUSE_PASSWORD),
        timeout=timeout,
    )
    response.raise_for_status()
    return [
        json.loads(line)
        for line in response.text.splitlines()
        if line.strip()
    ]
