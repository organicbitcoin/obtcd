#!/usr/bin/env bash

set -euo pipefail

HOST="${HOST:-}"
P2P_PORT="${P2P_PORT:-9527}"
RPC_PORT="${RPC_PORT:-9528}"
EXPECT_RPC_CLOSED=1
PLAN=0

usage() {
    cat <<EOF
Check OBTC mainnet-candidate seed/fallback firewall exposure.

Usage:
  $0 --host <host> [options]

Options:
  --host <host>             node hostname or IP to check
  --host=<host>
  --p2p-port <port>         expected public P2P port (default: 9527)
  --p2p-port=<port>
  --rpc-port <port>         RPC port that should remain private (default: 9528)
  --rpc-port=<port>
  --allow-public-rpc        mark public RPC reachability as expected
  --plan                    print checklist without network probes
  -h, --help                show this help

Examples:
  $0 --plan --host seed1.example.com
  $0 --host seed1.example.com
  $0 --host 203.0.113.10 --p2p-port=9527 --rpc-port=9528
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --host)
            HOST="$2"
            shift 2
            ;;
        --host=*)
            HOST="${1#*=}"
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
        --rpc-port)
            RPC_PORT="$2"
            shift 2
            ;;
        --rpc-port=*)
            RPC_PORT="${1#*=}"
            shift
            ;;
        --allow-public-rpc)
            EXPECT_RPC_CLOSED=0
            shift
            ;;
        --plan)
            PLAN=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "[ERROR] unknown option: $1" >&2
            usage
            exit 1
            ;;
    esac
done

positive_port() {
    [[ "$1" =~ ^[1-9][0-9]*$ ]] && [[ "$1" -le 65535 ]]
}

[[ -n "${HOST}" ]] || {
    echo "[ERROR] --host is required" >&2
    usage
    exit 1
}

positive_port "${P2P_PORT}" || {
    echo "[ERROR] --p2p-port must be 1..65535" >&2
    exit 1
}

positive_port "${RPC_PORT}" || {
    echo "[ERROR] --rpc-port must be 1..65535" >&2
    exit 1
}

print_plan() {
    cat <<EOF
[INFO] firewall preflight plan
  host: ${HOST}
  expected public P2P: ${P2P_PORT}/tcp reachable
  expected RPC exposure: ${RPC_PORT}/tcp $([[ ${EXPECT_RPC_CLOSED} -eq 1 ]] && printf 'closed from public networks' || printf 'publicly reachable by explicit exception')

Manual host-side checklist:
  - systemd unit uses --obtcmainnet and --listen=0.0.0.0:${P2P_PORT}
  - RPC is bound to 127.0.0.1:${RPC_PORT} or a private management interface
  - firewall allows inbound ${P2P_PORT}/tcp
  - firewall denies inbound ${RPC_PORT}/tcp from the public Internet
  - wallet RPC is not public
  - logs copied into evidence are redacted
EOF
}

check_nc() {
    local host="$1"
    local port="$2"
    nc -z -w 5 "${host}" "${port}" >/dev/null 2>&1
}

main() {
    print_plan

    if [[ ${PLAN} -eq 1 ]]; then
        echo "[OK] plan only; no network probes made"
        return 0
    fi

    local failures=0

    if check_nc "${HOST}" "${P2P_PORT}"; then
        echo "[PASS] ${HOST}:${P2P_PORT} is reachable for P2P"
    else
        echo "[FAIL] ${HOST}:${P2P_PORT} is not reachable for P2P" >&2
        failures=$((failures + 1))
    fi

    if check_nc "${HOST}" "${RPC_PORT}"; then
        if [[ ${EXPECT_RPC_CLOSED} -eq 1 ]]; then
            echo "[FAIL] ${HOST}:${RPC_PORT} is publicly reachable; RPC should be private" >&2
            failures=$((failures + 1))
        else
            echo "[PASS] ${HOST}:${RPC_PORT} is reachable by explicit exception"
        fi
    else
        if [[ ${EXPECT_RPC_CLOSED} -eq 1 ]]; then
            echo "[PASS] ${HOST}:${RPC_PORT} is not reachable from this network"
        else
            echo "[FAIL] ${HOST}:${RPC_PORT} is not reachable but --allow-public-rpc was set" >&2
            failures=$((failures + 1))
        fi
    fi

    if [[ ${failures} -ne 0 ]]; then
        exit 1
    fi

    echo "[OK] firewall preflight passed"
}

main
