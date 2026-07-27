#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/logs"
EXPERIMENT_DIR="${LOG_DIR}/experiment"
PLOTS_DIR="${EXPERIMENT_DIR}/plots"

RUN_SECONDS="${RUN_SECONDS:-45}"
CLEAN_EXPERIMENT_DIR="${CLEAN_EXPERIMENT_DIR:-1}"
TARGET_BLOCKS="${TARGET_BLOCKS:-0}"
REPLAY_BLOCKS_PER_ROUND="${REPLAY_BLOCKS_PER_ROUND:-15}"

print_usage() {
  cat <<EOF
Usage:
  $(basename "$0")

Environment overrides:
  RUN_SECONDS             default: ${RUN_SECONDS}
  CLEAN_EXPERIMENT_DIR    default: ${CLEAN_EXPERIMENT_DIR}
  TARGET_BLOCKS           default: ${TARGET_BLOCKS}
  REPLAY_BLOCKS_PER_ROUND default: ${REPLAY_BLOCKS_PER_ROUND}

Example:
  RUN_SECONDS=60 ./scripts/$(basename "$0")
  TARGET_BLOCKS=100 ./scripts/$(basename "$0")
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  print_usage
  exit 0
fi

cd "${ROOT_DIR}"

if [[ "${CLEAN_EXPERIMENT_DIR}" == "1" ]]; then
  rm -rf "${EXPERIMENT_DIR}"
fi
mkdir -p "${EXPERIMENT_DIR}"

cleanup() {
  ./scripts/run_demo.sh stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "starting experiment..."
echo "  run_seconds=${RUN_SECONDS}"
echo "  target_blocks=${TARGET_BLOCKS}"
echo "  experiment_dir=${EXPERIMENT_DIR}"
echo

if [[ "${TARGET_BLOCKS}" -gt 0 ]]; then
  echo "collecting data until block_summary >= ${TARGET_BLOCKS} ..."
  round=0
  while true; do
    count=0
    if [[ -f "${EXPERIMENT_DIR}/block_summary.jsonl" ]]; then
      count="$(wc -l < "${EXPERIMENT_DIR}/block_summary.jsonl" | tr -d ' ')"
    fi
    if [[ "${count}" -ge "${TARGET_BLOCKS}" ]]; then
      break
    fi

    round=$((round + 1))
    target_count=$((count + REPLAY_BLOCKS_PER_ROUND))
    echo
    echo "starting replay round ${round} ..."
    echo "  current_block_count=${count}"
    echo "  round_target_count=${target_count}"
    ./scripts/run_demo.sh start

    while true; do
      count="$(wc -l < "${EXPERIMENT_DIR}/block_summary.jsonl" | tr -d ' ')"
      echo "  round=${round} block_count=${count}"
      if [[ "${count}" -ge "${target_count}" ]]; then
        break
      fi
      sleep 5
    done

    ./scripts/run_demo.sh stop
    echo "completed replay round ${round}"
  done
else
  ./scripts/run_demo.sh start

  echo
  echo "collecting data for ${RUN_SECONDS}s ..."
  sleep "${RUN_SECONDS}"

  echo
  echo "stopping services ..."
  ./scripts/run_demo.sh stop
fi

echo
echo "generating plots ..."
MPLBACKEND=Agg python3 scripts/plot_experiment_metrics.py \
  --input-dir "${EXPERIMENT_DIR}" \
  --output-dir "${PLOTS_DIR}"

echo
echo "experiment completed"
echo "raw data : ${EXPERIMENT_DIR}"
echo "plots    : ${PLOTS_DIR}"
echo "summary  : ${PLOTS_DIR}/summary.json"
