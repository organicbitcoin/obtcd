#!/usr/bin/env bash

set -euo pipefail

HEIGHT=""
EXPECTED_HASH=""
BITCOIN_CLI="${BITCOIN_CLI:-}"

usage() {
    cat <<EOF
Verify the OBTC mainnet 72h rehearsal BTC fork anchor.

Usage:
  $0 --height <height> --hash <block_hash> [--bitcoin-cli <path>]

The script checks mempool.space and Blockstream. If --bitcoin-cli is provided,
it also checks the local BTC source with: <bitcoin-cli> getblockhash <height>.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --height)
            HEIGHT="$2"
            shift 2
            ;;
        --height=*)
            HEIGHT="${1#*=}"
            shift
            ;;
        --hash)
            EXPECTED_HASH="$2"
            shift 2
            ;;
        --hash=*)
            EXPECTED_HASH="${1#*=}"
            shift
            ;;
        --bitcoin-cli)
            BITCOIN_CLI="$2"
            shift 2
            ;;
        --bitcoin-cli=*)
            BITCOIN_CLI="${1#*=}"
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

if [[ ! "${HEIGHT}" =~ ^[1-9][0-9]*$ ]]; then
    echo "[ERROR] --height must be a positive integer" >&2
    exit 1
fi

if [[ -z "${EXPECTED_HASH}" ]]; then
    echo "[ERROR] --hash is required" >&2
    exit 1
fi

check_hash() {
    local source="$1"
    local got="$2"
    if [[ "${got}" != "${EXPECTED_HASH}" ]]; then
        echo "[ERROR] ${source} hash mismatch at height ${HEIGHT}" >&2
        echo "        got:  ${got}" >&2
        echo "        want: ${EXPECTED_HASH}" >&2
        exit 1
    fi
    echo "[OK] ${source} height ${HEIGHT} hash ${got}"
}

mempool_hash="$(curl -fsSL "https://mempool.space/api/block-height/${HEIGHT}")"
check_hash "mempool.space" "${mempool_hash}"

blockstream_hash="$(curl -fsSL "https://blockstream.info/api/block-height/${HEIGHT}")"
check_hash "blockstream.info" "${blockstream_hash}"

if [[ -n "${BITCOIN_CLI}" ]]; then
    if [[ ! -x "${BITCOIN_CLI}" ]]; then
        echo "[ERROR] bitcoin-cli not executable: ${BITCOIN_CLI}" >&2
        exit 1
    fi
    local_hash="$("${BITCOIN_CLI}" getblockhash "${HEIGHT}")"
    check_hash "local bitcoin-cli" "${local_hash}"
else
    echo "[WARN] --bitcoin-cli not provided; local BTC source was not checked"
fi

echo "[OK] fork anchor verified"
