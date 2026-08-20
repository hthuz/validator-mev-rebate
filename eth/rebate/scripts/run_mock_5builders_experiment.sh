#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/logs"
RUN_DIR="${ROOT_DIR}/run"

CONFIG_PATH="${CONFIG_PATH:-config/experiment_mock_5builders.yaml}"
EXPERIMENT_DIR="${EXPERIMENT_DIR:-logs/experiment_mock_5builders}"
PLOTS_DIR="${PLOTS_DIR:-${EXPERIMENT_DIR}/plots}"

TARGET_BLOCKS="${TARGET_BLOCKS:-0}"
CLEAN_EXPERIMENT_DIR="${CLEAN_EXPERIMENT_DIR:-1}"
MOCK_BUNDLES_PER_BLOCK="${MOCK_BUNDLES_PER_BLOCK:-5}"
MOCK_WORKERS="${MOCK_WORKERS:-64}"
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-2}"
DRAIN_SECONDS="${DRAIN_SECONDS:-20}"

INJECT_OBSERVATIONS="${INJECT_OBSERVATIONS:-1}"
STAGE_START_BLOCK="${STAGE_START_BLOCK:-1003000}"
STAGE_INTERVAL_BLOCKS="${STAGE_INTERVAL_BLOCKS:-1000}"
THETA_ATTACK_BLOCK="${THETA_ATTACK_BLOCK:-1006000}"
REGISTER_ZETA="${REGISTER_ZETA:-1}"
CLEAN_ORPHANS="${CLEAN_ORPHANS:-1}"

SERVER_LOG="${LOG_DIR}/mock_5builders_server.log"
USER_LOG="${LOG_DIR}/mock_5builders_user.log"
SERVER_PID_FILE="${RUN_DIR}/mock_5builders_server.pid"
USER_PID_FILE="${RUN_DIR}/mock_5builders_user.pid"

print_usage() {
  printf '%s\n' \
    "Usage:" \
    "  $(basename "$0")" \
    "" \
    "Environment overrides:" \
    "  CONFIG_PATH              default: ${CONFIG_PATH}" \
    "  EXPERIMENT_DIR           default: ${EXPERIMENT_DIR}" \
    "  PLOTS_DIR                default: ${PLOTS_DIR}" \
    "  TARGET_BLOCKS            default: ${TARGET_BLOCKS}" \
    "  CLEAN_EXPERIMENT_DIR     default: ${CLEAN_EXPERIMENT_DIR}" \
    "  MOCK_BUNDLES_PER_BLOCK   default: ${MOCK_BUNDLES_PER_BLOCK}" \
    "  MOCK_WORKERS             default: ${MOCK_WORKERS}" \
    "  SERVER_URL               default: ${SERVER_URL}" \
    "  POLL_INTERVAL_SECONDS    default: ${POLL_INTERVAL_SECONDS}" \
    "  DRAIN_SECONDS            default: ${DRAIN_SECONDS}" \
    "  INJECT_OBSERVATIONS      default: ${INJECT_OBSERVATIONS}" \
    "  STAGE_START_BLOCK        default: ${STAGE_START_BLOCK}" \
    "  STAGE_INTERVAL_BLOCKS    default: ${STAGE_INTERVAL_BLOCKS}" \
    "  THETA_ATTACK_BLOCK       default: ${THETA_ATTACK_BLOCK}" \
    "  REGISTER_ZETA            default: ${REGISTER_ZETA}" \
    "  CLEAN_ORPHANS            default: ${CLEAN_ORPHANS}" \
    "" \
    "Examples:" \
    "  MOCK_WORKERS=256 ./scripts/$(basename "$0")" \
    "  TARGET_BLOCKS=10000 ./scripts/$(basename "$0")" \
    "  CLEAN_EXPERIMENT_DIR=0 ./scripts/$(basename "$0")"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  print_usage
  exit 0
fi

cd "${ROOT_DIR}"
mkdir -p "${LOG_DIR}" "${RUN_DIR}"

abs_path() {
  local path="$1"
  if [[ "${path}" = /* ]]; then
    echo "${path}"
  else
    echo "${ROOT_DIR}/${path}"
  fi
}

EXPERIMENT_DIR_ABS="$(abs_path "${EXPERIMENT_DIR}")"
PLOTS_DIR_ABS="$(abs_path "${PLOTS_DIR}")"

is_running() {
  local pid_file="$1"
  if [[ ! -f "${pid_file}" ]]; then
    return 1
  fi

  local pid
  pid="$(cat "${pid_file}")"
  if [[ -z "${pid}" ]]; then
    return 1
  fi

  kill -0 "${pid}" 2>/dev/null
}

stop_one() {
  local name="$1"
  local pid_file="$2"

  if ! is_running "${pid_file}"; then
    rm -f "${pid_file}"
    return 0
  fi

  local pid
  pid="$(cat "${pid_file}")"
  kill "${pid}" 2>/dev/null || true

  for _ in {1..30}; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      rm -f "${pid_file}"
      echo "stopped ${name}"
      return 0
    fi
    sleep 0.2
  done

  kill -9 "${pid}" 2>/dev/null || true
  rm -f "${pid_file}"
  echo "force stopped ${name}"
}

cleanup() {
  stop_one "mock user" "${USER_PID_FILE}"
  stop_one "mock server" "${SERVER_PID_FILE}"
}
trap cleanup EXIT

cleanup_orphans() {
  if [[ "${CLEAN_ORPHANS}" != "1" ]]; then
    return 0
  fi

  pkill -f "${ROOT_DIR}/server" 2>/dev/null || true
  pkill -f "${ROOT_DIR}/user" 2>/dev/null || true
  pkill -f 'go run ./cmd/server' 2>/dev/null || true
  pkill -f 'go run ./cmd/user' 2>/dev/null || true
  pkill -f '/var/folders/.*/go-build.*/exe/server' 2>/dev/null || true
  pkill -f '/var/folders/.*/go-build.*/exe/user' 2>/dev/null || true
}

current_block() {
  curl -fsS -X POST "${SERVER_URL}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' |
    python3 -c 'import json,sys; print(int(json.load(sys.stdin)["result"], 16))'
}

wait_for_server() {
  for _ in {1..30}; do
    if curl -fsS "${SERVER_URL}/health" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "server did not become healthy; check ${SERVER_LOG}" >&2
  return 1
}

register_zeta() {
  curl -fsS -X POST "${SERVER_URL}/builders/register" \
    -H 'Content-Type: application/json' \
    -d '{"name":"builder-zeta","url":"http://localhost:18550","score":60}' >/dev/null
}

observe_builder() {
  local payload="$1"
  curl -fsS -X POST "${SERVER_URL}/builders/observe" \
    -H 'Content-Type: application/json' \
    -d "${payload}" >/dev/null
}

inject_stage_observations() {
  local block="$1"
  local registered_ref="$2"

  echo "stage block=${block}"
  if [[ "${REGISTER_ZETA}" == "1" && "${registered_ref}" == "0" ]]; then
    if register_zeta; then
      echo "  registered builder-zeta"
      registered_ref=1
    else
      echo "  builder-zeta registration skipped or failed"
    fi
  fi

  local payload
  for payload in \
    '{"builder":"builder-alpha","dispatchAttempts":100,"dispatchSuccesses":99,"wellBehavedEvents":5,"valueCreatedWei":"100000000000000000"}' \
    '{"builder":"builder-beta","dispatchAttempts":100,"dispatchSuccesses":98,"wellBehavedEvents":4,"valueCreatedWei":"50000000000000000"}' \
    '{"builder":"builder-gamma","dispatchAttempts":100,"dispatchSuccesses":95,"wellBehavedEvents":2}' \
    '{"builder":"builder-delta","dispatchAttempts":100,"dispatchSuccesses":85}' \
    '{"builder":"builder-epsilon","dispatchAttempts":50,"dispatchSuccesses":30,"sandwichAttacks":5}'; do
    observe_builder "${payload}"
  done

  if [[ "${registered_ref}" == "1" ]]; then
    observe_builder '{"builder":"builder-zeta","dispatchAttempts":100,"dispatchSuccesses":99,"wellBehavedEvents":5,"valueCreatedWei":"100000000000000000"}'
  fi

  if [[ "${block}" -ge "${THETA_ATTACK_BLOCK}" ]]; then
    payload='{"builder":"builder-theta","dispatchAttempts":50,"dispatchSuccesses":20,"sandwichAttacks":8}'
  else
    payload='{"builder":"builder-theta","dispatchAttempts":100,"dispatchSuccesses":99,"wellBehavedEvents":5,"valueCreatedWei":"100000000000000000"}'
  fi
  observe_builder "${payload}"

  REGISTERED_ZETA_STATE="${registered_ref}"
}

print_metrics_summary() {
  export EXPERIMENT_DIR_ABS
  python3 - <<'PY'
import json
import os
from collections import Counter
from pathlib import Path

p = Path(os.environ["EXPERIMENT_DIR_ABS"])

def load(name):
    path = p / name
    if not path.exists():
        return []
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]

files = [
    "block_summary.jsonl",
    "bundle_events.jsonl",
    "builder_dispatches.jsonl",
    "builder_snapshots.jsonl",
]
data = {name: load(name) for name in files}
print({name: len(items) for name, items in data.items()})

bundles = data["bundle_events.jsonl"]
dispatches = data["builder_dispatches.jsonl"]
snapshots = data["builder_snapshots.jsonl"]
print(
    "layers",
    Counter(item.get("layer") for item in dispatches),
    "success",
    sum(1 for item in bundles if item.get("simulation_success")) / len(bundles) if bundles else 0,
)
print("profit", sum(int(item.get("profit_wei", 0)) for item in bundles) / 1e18)

for name in sorted({item.get("builder") for item in dispatches if item.get("builder")}):
    rows = [item for item in dispatches if item.get("builder") == name]
    print(name, len(rows), len({item.get("target_block") for item in rows}), Counter(item.get("layer") for item in rows))

last = {}
for item in snapshots:
    last[item.get("builder")] = item
for name in sorted(k for k in last if k):
    item = last[name]
    print(
        "score",
        name,
        round(item.get("effective_score", 0), 3),
        item.get("dispatch_attempts"),
        item.get("dispatch_successes"),
        item.get("dispatch_failures"),
        item.get("sandwich_attacks"),
    )

for name, key in [
    ("bundle_events.jsonl", "target_block"),
    ("builder_dispatches.jsonl", "target_block"),
    ("block_summary.jsonl", "block_number"),
]:
    values = [item[key] for item in data[name] if key in item]
    if values:
        print(name, min(values), max(values), len(values))
PY
}

echo "preparing mock 5 builders experiment..."
echo "  config=${CONFIG_PATH}"
echo "  experiment_dir=${EXPERIMENT_DIR_ABS}"
echo "  plots_dir=${PLOTS_DIR_ABS}"
echo "  target_blocks=${TARGET_BLOCKS}"
echo "  mock_bundles_per_block=${MOCK_BUNDLES_PER_BLOCK}"
echo "  mock_workers=${MOCK_WORKERS}"
echo "  inject_observations=${INJECT_OBSERVATIONS}"
echo "  clean_orphans=${CLEAN_ORPHANS}"
echo

cleanup
cleanup_orphans

if [[ "${CLEAN_EXPERIMENT_DIR}" == "1" ]]; then
  rm -rf "${EXPERIMENT_DIR_ABS}"
fi
mkdir -p "${EXPERIMENT_DIR_ABS}" "${PLOTS_DIR_ABS}"

echo "building binaries..."
go build -o ./server ./cmd/server
go build -o ./user ./cmd/user

echo
echo "starting server..."
: > "${SERVER_LOG}"
(
  cd "${ROOT_DIR}"
  exec env NO_COLOR=1 ./server -config "${CONFIG_PATH}" >>"${SERVER_LOG}" 2>&1
) &
echo $! > "${SERVER_PID_FILE}"
echo "  pid=$(cat "${SERVER_PID_FILE}")"
echo "  log=${SERVER_LOG}"

wait_for_server

echo
echo "starting mock user..."
: > "${USER_LOG}"
(
  cd "${ROOT_DIR}"
  exec env NO_COLOR=1 ./user \
    -server "${SERVER_URL}" \
    -mock \
    -mock-bundles-per-block "${MOCK_BUNDLES_PER_BLOCK}" \
    -mock-workers "${MOCK_WORKERS}" >>"${USER_LOG}" 2>&1
) &
echo $! > "${USER_PID_FILE}"
echo "  pid=$(cat "${USER_PID_FILE}")"
echo "  log=${USER_LOG}"

echo
if [[ "${TARGET_BLOCKS}" -gt 0 ]]; then
  echo "collecting data until block_summary >= ${TARGET_BLOCKS} ..."
  next="${STAGE_START_BLOCK}"
  REGISTERED_ZETA_STATE=0
  while true; do
    count=0
    if [[ -f "${EXPERIMENT_DIR_ABS}/block_summary.jsonl" ]]; then
      count="$(wc -l < "${EXPERIMENT_DIR_ABS}/block_summary.jsonl" | tr -d ' ')"
    fi
    if ! block="$(current_block)"; then
      if ! is_running "${SERVER_PID_FILE}"; then
        echo "server stopped"
        break
      fi
      echo "failed to fetch current block; check ${SERVER_LOG}" >&2
      exit 1
    fi
    while [[ "${INJECT_OBSERVATIONS}" == "1" && "${block}" -ge "${next}" ]]; do
      inject_stage_observations "${block}" "${REGISTERED_ZETA_STATE}"
      next=$((next + STAGE_INTERVAL_BLOCKS))
    done
    echo "  block_count=${count} current_block=${block}"
    if [[ "${count}" -ge "${TARGET_BLOCKS}" ]]; then
      break
    fi
    sleep "${POLL_INTERVAL_SECONDS}"
  done
else
  echo "waiting for server to stop; mock stop block is configured in ${CONFIG_PATH}"
  next="${STAGE_START_BLOCK}"
  REGISTERED_ZETA_STATE=0
  while true; do
    if ! is_running "${SERVER_PID_FILE}"; then
      echo "server stopped"
      break
    fi

    if ! block="$(current_block)"; then
      if ! is_running "${SERVER_PID_FILE}"; then
        echo "server stopped"
        break
      fi
      echo "failed to fetch current block; check ${SERVER_LOG}" >&2
      exit 1
    fi
    while [[ "${INJECT_OBSERVATIONS}" == "1" && "${block}" -ge "${next}" ]]; do
      inject_stage_observations "${block}" "${REGISTERED_ZETA_STATE}"
      next=$((next + STAGE_INTERVAL_BLOCKS))
    done
    echo "  current_block=${block}"
    sleep "${POLL_INTERVAL_SECONDS}"
  done
fi

if [[ "${DRAIN_SECONDS}" -gt 0 ]]; then
  echo
  echo "waiting ${DRAIN_SECONDS}s for in-flight mock bundles..."
  sleep "${DRAIN_SECONDS}"
fi

echo
echo "stopping services..."
cleanup
trap - EXIT

echo
echo "generating plots and summary..."
MPLBACKEND=Agg python3 scripts/plot_experiment_metrics.py \
  --input-dir "${EXPERIMENT_DIR_ABS}" \
  --output-dir "${PLOTS_DIR_ABS}"

echo
echo "experiment completed"
echo "raw data : ${EXPERIMENT_DIR_ABS}"
echo "plots    : ${PLOTS_DIR_ABS}"
echo "summary  : ${PLOTS_DIR_ABS}/summary.json"

echo
echo "metrics:"
print_metrics_summary
