#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEFAULT_BTCCTL="${REPO_ROOT}/cmd/btcctl/btcctl"

BTCCTL_BIN="${BTCCTL_BIN:-${DEFAULT_BTCCTL}}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
RPC_SERVER="${RPC_SERVER:-127.0.0.1:19528}"
OUT_FILE=""
APPEND_FILE=""

usage() {
    cat <<EOF
Collect a Phase 6 validation snapshot from an OBTC testnet node.

Usage:
  $0 --rpcuser <user> --rpcpass <pass> [options]

Options:
  --rpcuser <user>          RPC username (required)
  --rpcpass <pass>          RPC password (required)
  --rpcserver <host:port>   RPC endpoint (default: 127.0.0.1:19528)
  --btcctl <path>           btcctl binary path (default: ./cmd/btcctl/btcctl)
  --out <file>              write snapshot markdown to this file
  --append <file>           append snapshot markdown to this file
  -h, --help                show this help

Examples:
  $0 --rpcuser=u --rpcpass=p --out /tmp/obtc-phase6-validation-snapshot.md
  $0 --rpcuser=u --rpcpass=p --append /tmp/obtc-phase6-validation.md
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
        --out)
            OUT_FILE="$2"
            shift 2
            ;;
        --out=*)
            OUT_FILE="${1#*=}"
            shift
            ;;
        --append)
            APPEND_FILE="$2"
            shift 2
            ;;
        --append=*)
            APPEND_FILE="${1#*=}"
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

if [[ -n "${OUT_FILE}" && -n "${APPEND_FILE}" ]]; then
    echo "[ERROR] use only one of --out or --append" >&2
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

safe_rpc() {
    local method="$1"
    if run_rpc "${method}" 2>/tmp/.phase6_snapshot_err.$$; then
        rm -f /tmp/.phase6_snapshot_err.$$
        return 0
    fi

    echo "RPC ${method} failed: $(cat /tmp/.phase6_snapshot_err.$$)" >&2
    rm -f /tmp/.phase6_snapshot_err.$$
    return 1
}

must_rpc() {
    local method="$1"
    safe_rpc "${method}"
}

optional_rpc() {
    local method="$1"
    if safe_rpc "${method}"; then
        return 0
    fi
    return 1
}

to_unix_utc() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

peer_count_from_json() {
    local peer_json="$1"
    python3 - <<'PY' "$peer_json"
import json
import sys
try:
    arr = json.loads(sys.argv[1])
    print(len(arr) if isinstance(arr, list) else 0)
except Exception:
    print("unknown")
PY
}

render_snapshot() {
    local ts="$1"
    local chain_info="$2"
    local peer_info="$3"
    local chain_tips="$4"
    local mempool_info="$5"
    local expiry_stats="$6"
    local expiry_status="$7"
    local peer_count="$8"

    cat <<EOF
## Phase 6 Snapshot (${ts})

- RPC server: ${RPC_SERVER}
- Peer count: ${peer_count}
- Expiry index RPC: ${expiry_status}

### getblockchaininfo

```json
${chain_info}
```

### getpeerinfo

```json
${peer_info}
```

### getchaintips

```json
${chain_tips}
```

### getmempoolinfo

```json
${mempool_info}
```

### getexpiryindexstats

```json
${expiry_stats}
```

EOF
}

main() {
    local ts
    ts="$(to_unix_utc)"

    echo "[INFO] collecting required RPC outputs..."
    local chain_info peer_info chain_tips mempool_info
    chain_info="$(must_rpc getblockchaininfo)"
    peer_info="$(must_rpc getpeerinfo)"
    chain_tips="$(must_rpc getchaintips)"
    mempool_info="$(must_rpc getmempoolinfo)"

    local expiry_stats expiry_status
    if expiry_stats="$(optional_rpc getexpiryindexstats)"; then
        expiry_status="available"
    else
        expiry_status="unavailable"
        expiry_stats="(not available: node may be running without --expiryindex)"
    fi

    local peer_count
    peer_count="$(peer_count_from_json "${peer_info}")"

    local snapshot
    snapshot="$(render_snapshot "${ts}" "${chain_info}" "${peer_info}" "${chain_tips}" "${mempool_info}" "${expiry_stats}" "${expiry_status}" "${peer_count}")"

    if [[ -n "${OUT_FILE}" ]]; then
        mkdir -p "$(dirname "${OUT_FILE}")"
        printf "%s\n" "${snapshot}" >"${OUT_FILE}"
        echo "[OK] snapshot written to ${OUT_FILE}"
        return 0
    fi

    if [[ -n "${APPEND_FILE}" ]]; then
        mkdir -p "$(dirname "${APPEND_FILE}")"
        printf "%s\n" "${snapshot}" >>"${APPEND_FILE}"
        echo "[OK] snapshot appended to ${APPEND_FILE}"
        return 0
    fi

    printf "%s\n" "${snapshot}"
}

main
