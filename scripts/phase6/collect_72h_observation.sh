#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SNAPSHOT_SCRIPT="${SCRIPT_DIR}/collect_validation_snapshot.sh"

NETWORK="${NETWORK:-obtcmainnet}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
RPC_SERVER="${RPC_SERVER:-}"
BTCCTL_BIN="${BTCCTL_BIN:-}"
OUT_FILE="${OUT_FILE:-}"
DURATION_HOURS=72
INTERVAL_HOURS=6
INTERVAL_SECONDS=""
SAMPLES=""
NOTLS=0
PLAN=0
NEW_FILE=0

usage() {
    cat <<EOF
Collect repeated OBTC node snapshots for a 72h mainnet-candidate observation window.

Usage:
  $0 --rpcuser <user> --rpcpass <pass> [options]

Options:
  --network <name>             OBTC network: obtctestnet or obtcmainnet (default: obtcmainnet)
  --network=<name>
  --rpcuser <user>             RPC username
  --rpcuser=<user>
  --rpcpass <pass>             RPC password
  --rpcpass=<pass>
  --rpcserver <host:port>      RPC endpoint
  --rpcserver=<host:port>
  --btcctl <path>              btcctl binary path
  --btcctl=<path>
  --out <file>                 markdown output file (default: /tmp/obtc-<network>-72h-observation.md)
  --out=<file>
  --duration-hours <n>         observation duration (default: 72)
  --duration-hours=<n>
  --interval-hours <n>         sample interval (default: 6)
  --interval-hours=<n>
  --interval-seconds <n>       override sleep interval, useful for rehearsal
  --interval-seconds=<n>
  --samples <n>                override sample count
  --samples=<n>
  --new-file                   replace output file before writing header
  --notls                      pass --notls to btcctl
  --plan                       print schedule without collecting RPC snapshots
  -h, --help                   show this help

Examples:
  $0 --plan
  $0 --network obtcmainnet --notls --rpcuser=u --rpcpass=p --new-file
  $0 --network obtcmainnet --notls --rpcuser=u --rpcpass=p --interval-seconds=10 --samples=3
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --network)
            NETWORK="$2"
            shift 2
            ;;
        --network=*)
            NETWORK="${1#*=}"
            shift
            ;;
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
        --duration-hours)
            DURATION_HOURS="$2"
            shift 2
            ;;
        --duration-hours=*)
            DURATION_HOURS="${1#*=}"
            shift
            ;;
        --interval-hours)
            INTERVAL_HOURS="$2"
            shift 2
            ;;
        --interval-hours=*)
            INTERVAL_HOURS="${1#*=}"
            shift
            ;;
        --interval-seconds)
            INTERVAL_SECONDS="$2"
            shift 2
            ;;
        --interval-seconds=*)
            INTERVAL_SECONDS="${1#*=}"
            shift
            ;;
        --samples)
            SAMPLES="$2"
            shift 2
            ;;
        --samples=*)
            SAMPLES="${1#*=}"
            shift
            ;;
        --new-file)
            NEW_FILE=1
            shift
            ;;
        --notls)
            NOTLS=1
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

case "${NETWORK}" in
    obtctestnet|obtcmainnet)
        ;;
    *)
        echo "[ERROR] --network must be obtctestnet or obtcmainnet" >&2
        exit 1
        ;;
esac

positive_int() {
    [[ "$1" =~ ^[1-9][0-9]*$ ]]
}

if ! positive_int "${DURATION_HOURS}"; then
    echo "[ERROR] --duration-hours must be a positive integer" >&2
    exit 1
fi

if ! positive_int "${INTERVAL_HOURS}"; then
    echo "[ERROR] --interval-hours must be a positive integer" >&2
    exit 1
fi

if [[ -n "${INTERVAL_SECONDS}" ]] && ! positive_int "${INTERVAL_SECONDS}"; then
    echo "[ERROR] --interval-seconds must be a positive integer" >&2
    exit 1
fi

if [[ -n "${SAMPLES}" ]] && ! positive_int "${SAMPLES}"; then
    echo "[ERROR] --samples must be a positive integer" >&2
    exit 1
fi

if [[ -z "${INTERVAL_SECONDS}" ]]; then
    INTERVAL_SECONDS=$((INTERVAL_HOURS * 3600))
fi

if [[ -z "${SAMPLES}" ]]; then
    SAMPLES=$((DURATION_HOURS / INTERVAL_HOURS + 1))
fi

if [[ -z "${OUT_FILE}" ]]; then
    OUT_FILE="/tmp/obtc-${NETWORK}-72h-observation.md"
fi

print_plan() {
    cat <<EOF
[INFO] observation plan
  network: ${NETWORK}
  output: ${OUT_FILE}
  duration hours: ${DURATION_HOURS}
  interval seconds: ${INTERVAL_SECONDS}
  samples: ${SAMPLES}
  RPC server: ${RPC_SERVER:-default}
  credentials: not recorded in output
EOF
}

write_header() {
    mkdir -p "$(dirname "${OUT_FILE}")"
    if [[ ${NEW_FILE} -eq 1 ]]; then
        : >"${OUT_FILE}"
    fi

    if [[ ! -s "${OUT_FILE}" ]]; then
        {
            echo "# OBTC ${NETWORK} Observation Window"
            echo
            echo "- Started UTC: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
            echo "- Duration hours: ${DURATION_HOURS}"
            echo "- Interval seconds: ${INTERVAL_SECONDS}"
            echo "- Planned samples: ${SAMPLES}"
            echo "- RPC server: ${RPC_SERVER:-default}"
            echo "- Credentials: not recorded"
            echo
        } >>"${OUT_FILE}"
    fi
}

collect_once() {
    local sample_index="$1"
    local elapsed_seconds=$((sample_index * INTERVAL_SECONDS))
    local elapsed_hours=$((elapsed_seconds / 3600))

    {
        echo
        echo "<!-- sample ${sample_index}/${SAMPLES}; elapsed approx T+${elapsed_hours}h; captured $(date -u +"%Y-%m-%dT%H:%M:%SZ") -->"
    } >>"${OUT_FILE}"

    local args=(
        --network "${NETWORK}"
        --rpcuser "${RPC_USER}"
        --rpcpass "${RPC_PASS}"
        --append "${OUT_FILE}"
    )
    if [[ -n "${RPC_SERVER}" ]]; then
        args+=(--rpcserver "${RPC_SERVER}")
    fi
    if [[ -n "${BTCCTL_BIN}" ]]; then
        args+=(--btcctl "${BTCCTL_BIN}")
    fi
    if [[ ${NOTLS} -eq 1 ]]; then
        args+=(--notls)
    fi

    "${SNAPSHOT_SCRIPT}" "${args[@]}"
}

main() {
    print_plan

    if [[ ${PLAN} -eq 1 ]]; then
        echo "[OK] plan only; no RPC calls made"
        return 0
    fi

    if [[ -z "${RPC_USER}" || -z "${RPC_PASS}" ]]; then
        echo "[ERROR] --rpcuser and --rpcpass are required unless --plan is used" >&2
        exit 1
    fi

    write_header

    local i
    for ((i = 0; i < SAMPLES; i++)); do
        echo "[INFO] collecting sample $((i + 1))/${SAMPLES}"
        collect_once "${i}"
        if [[ $((i + 1)) -lt ${SAMPLES} ]]; then
            echo "[INFO] sleeping ${INTERVAL_SECONDS}s before next sample"
            sleep "${INTERVAL_SECONDS}"
        fi
    done

    echo "[OK] observation snapshots written to ${OUT_FILE}"
}

main
