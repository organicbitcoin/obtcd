#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEFAULT_BTCCTL="${REPO_ROOT}/cmd/btcctl/btcctl"

BTCCTL_BIN="${BTCCTL_BIN:-${DEFAULT_BTCCTL}}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
RPC_SERVER="${RPC_SERVER:-127.0.0.1:19528}"
P2P_PORT="${P2P_PORT:-19527}"
MIN_PEERS="${MIN_PEERS:-1}"
STRICT_EXPIRYINDEX=0

PASS_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0

usage() {
    cat <<EOF
OBTC Phase 6 Seed Preflight Check

Usage:
  $0 --rpcuser <user> --rpcpass <pass> [options]

Options:
  --rpcuser <user>           RPC username (required)
  --rpcuser=<user>
  --rpcpass <pass>           RPC password (required)
  --rpcpass=<pass>
  --rpcserver <host:port>    RPC endpoint (default: 127.0.0.1:19528)
  --rpcserver=<host:port>
  --btcctl <path>            btcctl binary path (default: ./cmd/btcctl/btcctl)
  --btcctl=<path>
  --p2p-port <port>          expected local P2P listen port (default: 19527)
  --p2p-port=<port>
  --min-peers <n>            minimum connected peers (default: 1)
  --min-peers=<n>
  --strict-expiryindex       fail if getexpiryindexstats is unavailable
  -h, --help                 show this help

Examples:
  $0 --rpcuser=u --rpcpass=p
  $0 --rpcuser=u --rpcpass=p --rpcserver=10.0.0.8:19528 --min-peers=2
  $0 --rpcuser=u --rpcpass=p --strict-expiryindex
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --rpcuser)
            RPC_USER="$2"
            shift 2
            ;;
        --rpcuser=*)
            RPC_USER="${1#*=}"
            shift
            ;;
        --rpcpass)
            RPC_PASS="$2"
            shift 2
            ;;
        --rpcpass=*)
            RPC_PASS="${1#*=}"
            shift
            ;;
        --rpcserver)
            RPC_SERVER="$2"
            shift 2
            ;;
        --rpcserver=*)
            RPC_SERVER="${1#*=}"
            shift
            ;;
        --btcctl)
            BTCCTL_BIN="$2"
            shift 2
            ;;
        --btcctl=*)
            BTCCTL_BIN="${1#*=}"
            shift
            ;;
        --p2p-port)
            P2P_PORT="$2"
            shift 2
            ;;
        --p2p-port=*)
            P2P_PORT="${1#*=}"
            shift
            ;;
        --min-peers)
            MIN_PEERS="$2"
            shift 2
            ;;
        --min-peers=*)
            MIN_PEERS="${1#*=}"
            shift
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
    echo "        run: go build ./..." >&2
    exit 1
fi

if ! [[ "${P2P_PORT}" =~ ^[0-9]+$ ]]; then
    echo "[ERROR] invalid --p2p-port: ${P2P_PORT}" >&2
    exit 1
fi

if ! [[ "${MIN_PEERS}" =~ ^[0-9]+$ ]]; then
    echo "[ERROR] invalid --min-peers: ${MIN_PEERS}" >&2
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

pass() {
    echo "[PASS] $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

warn() {
    echo "[WARN] $1"
    WARN_COUNT=$((WARN_COUNT + 1))
}

fail() {
    echo "[FAIL] $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

json_get_field() {
    local json="$1"
    local field="$2"
    python3 - <<'PY' "$json" "$field"
import json
import sys
raw = sys.argv[1]
field = sys.argv[2]
try:
    obj = json.loads(raw)
    val = obj.get(field)
    if val is None:
        print("")
    elif isinstance(val, bool):
        print("true" if val else "false")
    else:
        print(val)
except Exception:
    print("")
PY
}

json_list_len() {
    local json="$1"
    python3 - <<'PY' "$json"
import json
import sys
try:
    obj = json.loads(sys.argv[1])
    print(len(obj) if isinstance(obj, list) else 0)
except Exception:
    print(0)
PY
}

json_active_tip_count() {
    local json="$1"
    python3 - <<'PY' "$json"
import json
import sys
try:
    tips = json.loads(sys.argv[1])
    if not isinstance(tips, list):
        print(0)
    else:
        print(sum(1 for t in tips if isinstance(t, dict) and t.get("status") == "active"))
except Exception:
    print(0)
PY
}

check_port_listener() {
    local port="$1"

    if command -v lsof >/dev/null 2>&1; then
        if lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | grep -q .; then
            return 0
        fi
        return 1
    fi

    if command -v ss >/dev/null 2>&1; then
        if ss -lnt 2>/dev/null | awk '{print $4}' | grep -E "(^|:|\.)${port}$" -q; then
            return 0
        fi
        return 1
    fi

    if command -v netstat >/dev/null 2>&1; then
        if netstat -an 2>/dev/null | grep -E "LISTEN|LISTENING" | grep -E "(^|[\.:])${port}[[:space:]]" -q; then
            return 0
        fi
        return 1
    fi

    return 2
}

main() {
    echo "[INFO] Running seed preflight checks against ${RPC_SERVER}"

    local chain_info
    if ! chain_info="$(run_rpc getblockchaininfo 2>&1)"; then
        fail "RPC getblockchaininfo failed: ${chain_info}"
        echo "[RESULT] FAILED (${FAIL_COUNT} failed, ${WARN_COUNT} warning, ${PASS_COUNT} passed)"
        exit 1
    fi

    local chain_name blocks headers ibd
    chain_name="$(json_get_field "${chain_info}" "chain")"
    blocks="$(json_get_field "${chain_info}" "blocks")"
    headers="$(json_get_field "${chain_info}" "headers")"
    ibd="$(json_get_field "${chain_info}" "initialblockdownload")"
    pass "RPC reachable (chain=${chain_name:-unknown}, blocks=${blocks:-unknown}, headers=${headers:-unknown}, ibd=${ibd:-unknown})"

    local peer_info
    if ! peer_info="$(run_rpc getpeerinfo 2>&1)"; then
        fail "RPC getpeerinfo failed: ${peer_info}"
    else
        local peer_count
        peer_count="$(json_list_len "${peer_info}")"
        if (( peer_count < MIN_PEERS )); then
            fail "Peer count below threshold: ${peer_count} < ${MIN_PEERS}"
        else
            pass "Peer count meets threshold: ${peer_count} >= ${MIN_PEERS}"
        fi
    fi

    local tips
    if ! tips="$(run_rpc getchaintips 2>&1)"; then
        fail "RPC getchaintips failed: ${tips}"
    else
        local active_count
        active_count="$(json_active_tip_count "${tips}")"
        if (( active_count < 1 )); then
            fail "No active chain tip found"
        else
            pass "Active tip detected (${active_count})"
        fi
    fi

    local mempool_info
    if ! mempool_info="$(run_rpc getmempoolinfo 2>&1)"; then
        fail "RPC getmempoolinfo failed: ${mempool_info}"
    else
        local mempool_size
        mempool_size="$(json_get_field "${mempool_info}" "size")"
        pass "Mempool RPC available (size=${mempool_size:-unknown})"
    fi

    local expiry_stats
    if ! expiry_stats="$(run_rpc getexpiryindexstats 2>&1)"; then
        if (( STRICT_EXPIRYINDEX == 1 )); then
            fail "Expiry index RPC unavailable in strict mode: ${expiry_stats}"
        else
            warn "Expiry index RPC unavailable: ${expiry_stats}"
        fi
    else
        local tip_height indexed_utxos
        tip_height="$(json_get_field "${expiry_stats}" "tip_height")"
        indexed_utxos="$(json_get_field "${expiry_stats}" "indexed_utxos")"
        pass "Expiry index RPC available (tip_height=${tip_height:-unknown}, indexed_utxos=${indexed_utxos:-unknown})"
    fi

    local port_check
    if check_port_listener "${P2P_PORT}"; then
        port_check=0
    else
        port_check=$?
    fi

    case "${port_check}" in
        0)
            pass "Local listener detected on TCP ${P2P_PORT}"
            ;;
        1)
            warn "No local listener detected on TCP ${P2P_PORT} (verify --listen or host firewall/NAT)"
            ;;
        2)
            warn "Port listener check skipped (no lsof/ss/netstat available)"
            ;;
    esac

    if (( FAIL_COUNT > 0 )); then
        echo "[RESULT] FAILED (${FAIL_COUNT} failed, ${WARN_COUNT} warning, ${PASS_COUNT} passed)"
        exit 1
    fi

    echo "[RESULT] PASSED (${FAIL_COUNT} failed, ${WARN_COUNT} warning, ${PASS_COUNT} passed)"
}

main
