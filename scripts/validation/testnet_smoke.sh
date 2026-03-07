#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEFAULT_BTCCTL="${REPO_ROOT}/cmd/btcctl/btcctl"

BTCCTL_BIN="${BTCCTL_BIN:-${DEFAULT_BTCCTL}}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
RPC_SERVER="${RPC_SERVER:-127.0.0.1:19528}"
STRICT_EXPIRYINDEX=0

usage() {
    cat <<EOF
OBTC Testnet Smoke Check

Usage:
  $0 --rpcuser <user> --rpcpass <pass> [--rpcserver host:port] [--btcctl path] [--strict-expiryindex]

Options:
  --rpcuser <user>          RPC username (required)
  --rpcpass <pass>          RPC password (required)
  --rpcserver <host:port>   RPC endpoint (default: 127.0.0.1:19528)
  --btcctl <path>           btcctl binary path (default: ./cmd/btcctl/btcctl)
  --strict-expiryindex      fail when getexpiryindexstats is unavailable
  -h, --help                show this help
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --rpcuser)
            RPC_USER="$2"
            shift 2
            ;;
        --rpcpass)
            RPC_PASS="$2"
            shift 2
            ;;
        --rpcserver)
            RPC_SERVER="$2"
            shift 2
            ;;
        --btcctl)
            BTCCTL_BIN="$2"
            shift 2
            ;;
        --strict-expiryindex)
            STRICT_EXPIRYINDEX=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage
            exit 1
            ;;
    esac
done

if [[ -z "${RPC_USER}" || -z "${RPC_PASS}" ]]; then
    echo "[ERROR] --rpcuser and --rpcpass are required" >&2
    usage
    exit 1
fi

if [[ ! -x "${BTCCTL_BIN}" ]]; then
    echo "[ERROR] btcctl not found or not executable: ${BTCCTL_BIN}" >&2
    exit 1
fi

run_rpc() {
    "${BTCCTL_BIN}" \
        --obtctestnet \
        "--rpcuser=${RPC_USER}" \
        "--rpcpass=${RPC_PASS}" \
        "--rpcserver=${RPC_SERVER}" \
        "$@"
}

check_required() {
    local method="$1"
    echo "[CHECK] ${method}"
    if run_rpc "${method}" >/dev/null; then
        echo "[ OK ] ${method}"
    else
        echo "[FAIL] ${method}" >&2
        return 1
    fi
}

check_optional_expiryindex() {
    echo "[CHECK] getexpiryindexstats (optional unless --strict-expiryindex)"

    local output
    if output="$(run_rpc getexpiryindexstats 2>&1)"; then
        echo "[ OK ] getexpiryindexstats"
        return 0
    fi

    echo "[WARN] getexpiryindexstats unavailable"
    echo "       details: ${output}" >&2

    if [[ ${STRICT_EXPIRYINDEX} -eq 1 ]]; then
        echo "[FAIL] strict mode enabled; treating expiryindex RPC as required" >&2
        return 1
    fi

    echo "[INFO] continuing (node may be running without --expiryindex)"
    return 0
}

main() {
    local failed=0

    check_required getblockchaininfo || failed=1
    check_required getpeerinfo || failed=1
    check_required getchaintips || failed=1
    check_required getmempoolinfo || failed=1
    check_optional_expiryindex || failed=1

    if [[ ${failed} -ne 0 ]]; then
        echo "[RESULT] SMOKE CHECK FAILED" >&2
        exit 1
    fi

    echo "[RESULT] SMOKE CHECK PASSED"
}

main
