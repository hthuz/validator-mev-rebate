#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/logs"
RUN_DIR="${ROOT_DIR}/run"

SERVER_LOG="${LOG_DIR}/server.log"
SEARCHER_LOG="${LOG_DIR}/searcher.log"
USER_LOG="${LOG_DIR}/user.log"

SERVER_PID_FILE="${RUN_DIR}/server.pid"
SEARCHER_PID_FILE="${RUN_DIR}/searcher.pid"
USER_PID_FILE="${RUN_DIR}/user.pid"

SERVER_URL="${SERVER_URL:-http://localhost:8080}"
EVENTS_URL="${EVENTS_URL:-${SERVER_URL}/events}"
DATASET_PATH="${DATASET_PATH:-data/ethereum_transactions.csv}"
USER_INTERVAL="${USER_INTERVAL:-5s}"
SEARCHER_MAX_CHAIN_DEPTH="${SEARCHER_MAX_CHAIN_DEPTH:-2}"

mkdir -p "${LOG_DIR}" "${RUN_DIR}"

print_usage() {
  cat <<EOF
Usage:
  $(basename "$0") start
  $(basename "$0") stop
  $(basename "$0") restart
  $(basename "$0") status

Environment overrides:
  SERVER_URL                  default: ${SERVER_URL}
  EVENTS_URL                  default: ${EVENTS_URL}
  DATASET_PATH                default: ${DATASET_PATH}
  USER_INTERVAL               default: ${USER_INTERVAL}
  SEARCHER_MAX_CHAIN_DEPTH    default: ${SEARCHER_MAX_CHAIN_DEPTH}

Logs:
  ${SERVER_LOG}
  ${SEARCHER_LOG}
  ${USER_LOG}
EOF
}

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

start_one() {
  local name="$1"
  local pid_file="$2"
  local log_file="$3"
  shift 3

  if is_running "${pid_file}"; then
    echo "${name} is already running (pid=$(cat "${pid_file}"))"
    return 0
  fi

  : > "${log_file}"
  (
    cd "${ROOT_DIR}"
    "$@" >>"${log_file}" 2>&1
  ) &
  local pid=$!
  echo "${pid}" > "${pid_file}"
  echo "started ${name} (pid=${pid})"
  echo "log: ${log_file}"
}

stop_one() {
  local name="$1"
  local pid_file="$2"

  if ! is_running "${pid_file}"; then
    rm -f "${pid_file}"
    echo "${name} is not running"
    return 0
  fi

  local pid
  pid="$(cat "${pid_file}")"
  kill "${pid}" 2>/dev/null || true

  for _ in {1..20}; do
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

status_one() {
  local name="$1"
  local pid_file="$2"
  local log_file="$3"

  if is_running "${pid_file}"; then
    echo "${name}: running (pid=$(cat "${pid_file}")) log=${log_file}"
    return 0
  fi

  echo "${name}: stopped log=${log_file}"
}

start_all() {
  echo "starting demo with:"
  echo "  SERVER_URL=${SERVER_URL}"
  echo "  EVENTS_URL=${EVENTS_URL}"
  echo "  DATASET_PATH=${DATASET_PATH}"
  echo "  USER_INTERVAL=${USER_INTERVAL}"
  echo "  SEARCHER_MAX_CHAIN_DEPTH=${SEARCHER_MAX_CHAIN_DEPTH}"
  echo

  start_one "server" "${SERVER_PID_FILE}" "${SERVER_LOG}" \
    go run ./cmd/server

  sleep 2

  start_one "searcher" "${SEARCHER_PID_FILE}" "${SEARCHER_LOG}" \
    go run ./cmd/searcher \
      -server "${SERVER_URL}" \
      -events "${EVENTS_URL}" \
      -dataset "${DATASET_PATH}" \
      -max-chain-depth "${SEARCHER_MAX_CHAIN_DEPTH}"

  sleep 1

  start_one "user" "${USER_PID_FILE}" "${USER_LOG}" \
    go run ./cmd/user \
      -server "${SERVER_URL}" \
      -dataset "${DATASET_PATH}" \
      -interval "${USER_INTERVAL}"

  echo
  echo "all services started"
  echo "server log   : ${SERVER_LOG}"
  echo "searcher log : ${SEARCHER_LOG}"
  echo "user log     : ${USER_LOG}"
  echo
  echo "watch logs:"
  echo "  tail -f ${SERVER_LOG}"
  echo "  tail -f ${SEARCHER_LOG}"
  echo "  tail -f ${USER_LOG}"
}

stop_all() {
  stop_one "user" "${USER_PID_FILE}"
  stop_one "searcher" "${SEARCHER_PID_FILE}"
  stop_one "server" "${SERVER_PID_FILE}"
}

status_all() {
  status_one "server" "${SERVER_PID_FILE}" "${SERVER_LOG}"
  status_one "searcher" "${SEARCHER_PID_FILE}" "${SEARCHER_LOG}"
  status_one "user" "${USER_PID_FILE}" "${USER_LOG}"
}

case "${1:-}" in
  start)
    start_all
    ;;
  stop)
    stop_all
    ;;
  restart)
    stop_all
    start_all
    ;;
  status)
    status_all
    ;;
  *)
    print_usage
    exit 1
    ;;
esac
