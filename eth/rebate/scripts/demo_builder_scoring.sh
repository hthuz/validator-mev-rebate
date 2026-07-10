#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DEMO_SCRIPT="${ROOT_DIR}/scripts/run_demo.sh"
SERVER_LOG="${ROOT_DIR}/logs/server.log"

SERVER_URL="${SERVER_URL:-http://localhost:8080}"
WINDOW_SECONDS="${WINDOW_SECONDS:-10}"
USER_INTERVAL="${USER_INTERVAL:-1s}"
SEARCHER_MAX_CHAIN_DEPTH="${SEARCHER_MAX_CHAIN_DEPTH:-1}"

print_header() {
  echo
  echo "============================================================"
  echo "$1"
  echo "============================================================"
}

wait_for_health() {
  local timeout_seconds="${1:-20}"
  local start_ts
  start_ts="$(date +%s)"

  while true; do
    if curl -fsS "${SERVER_URL}/health" >/dev/null 2>&1; then
      return 0
    fi

    if (( "$(date +%s)" - start_ts >= timeout_seconds )); then
      echo "server health check timed out after ${timeout_seconds}s"
      return 1
    fi
    sleep 1
  done
}

print_scores() {
  local title="$1"
  print_header "${title}"

  local response
  response="$(curl -fsS "${SERVER_URL}/builders/scores")"

  RESPONSE_JSON="${response}" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["RESPONSE_JSON"])
builders = payload.get("builders", [])
total = sum(float(b.get("score", 0.0)) for b in builders) or 1.0

print(f"{'builder':<16} {'base':>8} {'score':>8} {'weight':>8} {'succ/att':>12} {'sandwich':>10} {'well':>8} {'valueFlow':>10}")
for b in builders:
    stats = b.get("stats", {})
    score = float(b.get("score", 0.0))
    attempts = int(stats.get("DispatchAttempts", 0))
    successes = int(stats.get("DispatchSuccesses", 0))
    print(
        f"{b.get('name', ''):<16} "
        f"{float(b.get('baseScore', 0.0)):>8.2f} "
        f"{score:>8.2f} "
        f"{(score / total * 100):>7.1f}% "
        f"{successes:>5}/{attempts:<6} "
        f"{int(stats.get('SandwichAttacks', 0)):>10} "
        f"{int(stats.get('WellBehavedEvents', 0)):>8} "
        f"{int(stats.get('ValuableOrderFlow', 0)):>10}"
    )
PY
}

observe_builder() {
  local builder="$1"
  local attempts="$2"
  local successes="$3"
  local sandwich="$4"
  local well="$5"
  local value_created_wei="$6"

  curl -fsS -X POST "${SERVER_URL}/builders/observe" \
    -H 'Content-Type: application/json' \
    -d "{
      \"builder\": \"${builder}\",
      \"dispatchAttempts\": ${attempts},
      \"dispatchSuccesses\": ${successes},
      \"sandwichAttacks\": ${sandwich},
      \"wellBehavedEvents\": ${well},
      \"valueCreatedWei\": \"${value_created_wei}\"
    }" >/dev/null
}

sample_dispatch_window() {
  local label="$1"
  local window_seconds="$2"
  local start_line
  start_line="$(wc -l < "${SERVER_LOG}" | tr -d ' ')"

  print_header "${label}（采样 ${window_seconds}s）"
  echo "从 server.log 中统计新增的 'Dispatching bundle to builder' 记录..."
  sleep "${window_seconds}"

  START_LINE="${start_line}" SERVER_LOG="${SERVER_LOG}" python3 - <<'PY'
import os
import re
from collections import Counter

path = os.environ["SERVER_LOG"]
start_line = int(os.environ["START_LINE"])

counter = Counter()
pattern = re.compile(r"builder=(\S+)")
dispatch_lines = 0

with open(path, "r", encoding="utf-8", errors="replace") as f:
    for idx, line in enumerate(f, start=1):
        if idx <= start_line:
            continue
        if "Dispatching bundle to builder" not in line:
            continue
        dispatch_lines += 1
        match = pattern.search(line)
        if match:
            counter[match.group(1)] += 1

if dispatch_lines == 0:
    print("这段时间没有观察到新的 dispatch 记录。")
else:
    print(f"新 dispatch 总数: {dispatch_lines}")
    for builder, count in sorted(counter.items()):
        pct = count / dispatch_lines * 100
        print(f"  {builder:<16} {count:>4}  ({pct:>5.1f}%)")
PY
}

main() {
  print_header "Builder Score 动态分发演示"
  echo "SERVER_URL=${SERVER_URL}"
  echo "WINDOW_SECONDS=${WINDOW_SECONDS}"
  echo "USER_INTERVAL=${USER_INTERVAL}"
  echo "SEARCHER_MAX_CHAIN_DEPTH=${SEARCHER_MAX_CHAIN_DEPTH}"

  print_header "Step 1: 重启干净演示环境"
  SERVER_URL="${SERVER_URL}" \
  USER_INTERVAL="${USER_INTERVAL}" \
  SEARCHER_MAX_CHAIN_DEPTH="${SEARCHER_MAX_CHAIN_DEPTH}" \
  "${RUN_DEMO_SCRIPT}" restart

  wait_for_health 30
  print_scores "Step 2: 初始 Builder 分数"
  sample_dispatch_window "Step 3: baseline dispatch 分布" "${WINDOW_SECONDS}"

  print_header "Step 4: 注入行为信号"
  echo "builder-alpha: 模拟高价值、well-behaved、稳定交付"
  observe_builder "builder-alpha" 8 8 0 5 900000000000000000
  echo "builder-beta: 模拟 sandwich 报告、失败增加"
  observe_builder "builder-beta" 8 3 4 0 0

  print_scores "Step 5: 注入后 Builder 分数"
  sample_dispatch_window "Step 6: 注入后 dispatch 分布" "${WINDOW_SECONDS}"

  print_header "Step 7: 可手动继续观察"
  echo "当前服务仍在运行。你可以继续执行："
  echo "  curl ${SERVER_URL}/builders/scores"
  echo "  tail -f ${SERVER_LOG}"
  echo "停止环境："
  echo "  ${RUN_DEMO_SCRIPT} stop"
}

main "$@"
