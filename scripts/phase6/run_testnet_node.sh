#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

BTCD_BIN="${BTCD_BIN:-${REPO_ROOT}/btcd}"
BTCCTL_BIN="${BTCCTL_BIN:-${REPO_ROOT}/cmd/btcctl/btcctl}"

NETWORK="${NETWORK:-obtctestnet}"

case "${NETWORK}" in
obtctestnet)
    DEFAULT_DATA_DIR="${HOME}/.obtcd-testnet"
    DEFAULT_P2P_LISTEN="0.0.0.0:19527"
    DEFAULT_RPC_LISTEN="127.0.0.1:19528"
    DEFAULT_RPC_SERVER="127.0.0.1:19528"
    NETWORK_FLAG="--obtctestnet"
    ;;
obtcmainnet)
    DEFAULT_DATA_DIR="${HOME}/.obtcd-mainnet"
    DEFAULT_P2P_LISTEN="0.0.0.0:9527"
    DEFAULT_RPC_LISTEN="127.0.0.1:9528"
    DEFAULT_RPC_SERVER="127.0.0.1:9528"
    NETWORK_FLAG="--obtcmainnet"
    ;;
obtcmainnet72h)
    DEFAULT_DATA_DIR="${HOME}/.obtcd-mainnet72h"
    DEFAULT_P2P_LISTEN="0.0.0.0:39527"
    DEFAULT_RPC_LISTEN="127.0.0.1:39528"
    DEFAULT_RPC_SERVER="127.0.0.1:39528"
    NETWORK_FLAG="--obtcmainnet72h"
    ;;
*)
    echo "[ERROR] NETWORK must be obtctestnet, obtcmainnet, or obtcmainnet72h" >&2
    exit 1
    ;;
esac

DATA_DIR="${DATA_DIR:-${DEFAULT_DATA_DIR}}"
LOG_DIR="${LOG_DIR:-${DATA_DIR}/logs}"
LOG_FILE="${LOG_FILE:-${LOG_DIR}/obtcd.log}"
PID_FILE="${PID_FILE:-${DATA_DIR}/obtcd.pid}"

RPC_USER="${RPC_USER:-obtc}"
RPC_PASS="${RPC_PASS:-change-me-in-env}"

P2P_LISTEN="${P2P_LISTEN:-${DEFAULT_P2P_LISTEN}}"
RPC_LISTEN="${RPC_LISTEN:-${DEFAULT_RPC_LISTEN}}"
RPC_SERVER="${RPC_SERVER:-${DEFAULT_RPC_SERVER}}"

ENABLE_EXPIRYINDEX="${ENABLE_EXPIRYINDEX:-1}"
ADDPEERS="${ADDPEERS:-}" # comma-separated: host1:19527,host2:19527
EXTRA_FLAGS="${EXTRA_FLAGS:-}" # optional extra btcd flags

usage() {
    cat <<EOF
OBTC Phase6 Node Helper

Usage:
  $0 start|stop|restart|status|tail

Environment variables:
  BTCD_BIN            btcd binary path (default: ./btcd in repo root)
  BTCCTL_BIN          btcctl binary path (default: ./cmd/btcctl/btcctl)
  NETWORK             obtctestnet, obtcmainnet, or obtcmainnet72h (default: obtctestnet)
  DATA_DIR            data directory (default depends on NETWORK)
  RPC_USER            rpc username
  RPC_PASS            rpc password
  P2P_LISTEN          p2p listen endpoint (default depends on NETWORK)
  RPC_LISTEN          rpc listen endpoint (default depends on NETWORK)
  RPC_SERVER          rpc server endpoint for status calls
  ENABLE_EXPIRYINDEX  1 to enable --expiryindex, 0 to disable (default: 1)
  ADDPEERS            comma-separated peers
  EXTRA_FLAGS         additional btcd flags string

Examples:
  RPC_USER=u RPC_PASS=p $0 start
  ADDPEERS=10.0.0.2:19527,10.0.0.3:19527 $0 start
  NETWORK=obtcmainnet72h ADDPEERS=10.0.0.2:39527,10.0.0.3:39527 $0 start
  ENABLE_EXPIRYINDEX=0 $0 restart
EOF
}

is_running() {
    if [[ ! -f "${PID_FILE}" ]]; then
        return 1
    fi

    local pid
    pid="$(cat "${PID_FILE}")"
    if [[ -z "${pid}" ]]; then
        return 1
    fi

    kill -0 "${pid}" >/dev/null 2>&1
}

start_node() {
    if [[ ! -x "${BTCD_BIN}" ]]; then
        echo "[ERROR] btcd binary not found or not executable: ${BTCD_BIN}" >&2
        echo "        run: go build ./..." >&2
        exit 1
    fi

    if is_running; then
        echo "[INFO] node already running (pid=$(cat "${PID_FILE}"))"
        return 0
    fi

    mkdir -p "${DATA_DIR}" "${LOG_DIR}"

    local -a args
    args=(
        "${NETWORK_FLAG}"
        "--datadir=${DATA_DIR}"
        "--listen=${P2P_LISTEN}"
        "--rpclisten=${RPC_LISTEN}"
        --txindex
        --notls
        "--rpcuser=${RPC_USER}"
        "--rpcpass=${RPC_PASS}"
    )

    if [[ "${ENABLE_EXPIRYINDEX}" == "1" ]]; then
        args+=(--expiryindex)
    fi

    if [[ -n "${ADDPEERS}" ]]; then
        IFS=',' read -r -a peers <<<"${ADDPEERS}"
        for peer in "${peers[@]}"; do
            [[ -z "${peer}" ]] && continue
            args+=("--addpeer=${peer}")
        done
    fi

    if [[ -n "${EXTRA_FLAGS}" ]]; then
        # shellcheck disable=SC2206
        local -a extra=( ${EXTRA_FLAGS} )
        args+=("${extra[@]}")
    fi

    echo "[INFO] starting obtcd ${NETWORK} node..."
    nohup "${BTCD_BIN}" "${args[@]}" >>"${LOG_FILE}" 2>&1 &
    local pid=$!
    echo "${pid}" >"${PID_FILE}"

    sleep 1
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
        echo "[ERROR] node failed to start, check ${LOG_FILE}" >&2
        tail -n 50 "${LOG_FILE}" >&2 || true
        rm -f "${PID_FILE}"
        exit 1
    fi

    echo "[OK] node started (pid=${pid})"
    echo "     log: ${LOG_FILE}"
    echo "     rpc: ${RPC_SERVER}"
}

stop_node() {
    if ! is_running; then
        echo "[INFO] node is not running"
        rm -f "${PID_FILE}"
        return 0
    fi

    local pid
    pid="$(cat "${PID_FILE}")"

    echo "[INFO] stopping node (pid=${pid})"
    kill "${pid}" >/dev/null 2>&1 || true

    local waited=0
    while kill -0 "${pid}" >/dev/null 2>&1; do
        sleep 1
        waited=$((waited + 1))
        if [[ ${waited} -ge 15 ]]; then
            echo "[WARN] graceful stop timeout, forcing kill -9"
            kill -9 "${pid}" >/dev/null 2>&1 || true
            break
        fi
    done

    rm -f "${PID_FILE}"
    echo "[OK] node stopped"
}

status_node() {
    if is_running; then
        local pid
        pid="$(cat "${PID_FILE}")"
        echo "[OK] node is running (pid=${pid})"
    else
        echo "[INFO] node is not running"
    fi

    if [[ -x "${BTCCTL_BIN}" ]] && is_running; then
        echo
        echo "[INFO] getblockchaininfo"
        if ! "${BTCCTL_BIN}" \
            "${NETWORK_FLAG}" \
            "--rpcuser=${RPC_USER}" \
            "--rpcpass=${RPC_PASS}" \
            "--rpcserver=${RPC_SERVER}" \
            getblockchaininfo; then
            echo "[WARN] failed to fetch blockchain info (check RPC credentials or startup state)" >&2
        fi
    fi
}

case "${1:-}" in
start)
    start_node
    ;;
stop)
    stop_node
    ;;
restart)
    stop_node
    start_node
    ;;
status)
    status_node
    ;;
tail)
    mkdir -p "${LOG_DIR}"
    touch "${LOG_FILE}"
    tail -f "${LOG_FILE}"
    ;;
-h|--help|help)
    usage
    ;;
*)
    usage
    exit 1
    ;;
esac
