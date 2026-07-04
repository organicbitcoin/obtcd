#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEFAULT_BTCCTL="${REPO_ROOT}/cmd/btcctl/btcctl"

NETWORK="${NETWORK:-obtcmainnet72h}"
BTCCTL_BIN="${BTCCTL_BIN:-${DEFAULT_BTCCTL}}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
SOURCE_RPC="${SOURCE_RPC:-}"
VALIDATOR_RPC="${VALIDATOR_RPC:-}"
FROM_HEIGHT=""
TO_HEIGHT=""
OUT_DIR="${OUT_DIR:-/tmp/obtc-mainnet72h-reap-block-validation}"
RUN_ID="${RUN_ID:-}"
S3_URI="${S3_URI:-}"
AWS_PROFILE="${AWS_PROFILE:-obtc-testnet-deployer}"
NOTLS=0
UPLOAD=0
STRICT=0

usage() {
    cat <<EOF
Export and validate actual OBTC REAP block evidence.

Usage:
  $0 --rpcuser <user> --rpcpass <pass> --source-rpc <host:port> --from-height <h> --to-height <h> [options]

Options:
  --network <name>             OBTC network (default: obtcmainnet72h)
  --network=<name>
  --source-rpc <host:port>     source/miner node RPC endpoint
  --source-rpc=<host:port>
  --validator-rpc <host:port>  independent validator RPC endpoint
  --validator-rpc=<host:port>
  --from-height <height>       first height to scan
  --from-height=<height>
  --to-height <height>         last height to scan
  --to-height=<height>
  --rpcuser <user>             RPC username
  --rpcuser=<user>
  --rpcpass <pass>             RPC password
  --rpcpass=<pass>
  --btcctl <path>              btcctl binary path
  --btcctl=<path>
  --outdir <dir>               local evidence directory
  --outdir=<dir>
  --run-id <id>                rehearsal run ID
  --run-id=<id>
  --s3-uri <s3://bucket/prefix> upload destination
  --s3-uri=<s3://bucket/prefix>
  --aws-profile <profile>      AWS profile for upload (default: obtc-testnet-deployer)
  --aws-profile=<profile>
  --notls                      pass --notls to btcctl
  --upload                     upload evidence to S3
  --strict                     exit non-zero if any REAP block fails validator confirmation
  -h, --help                   show this help

The script detects REAP blocks by scanning raw block hex for the OP_RETURN
marker payload prefix "REAP:" (hex 524541503a). It proves independent node
confirmation by requiring the validator RPC to return the same hash at the same
height and to serve the same block.
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
        --source-rpc)
            SOURCE_RPC="$2"
            shift 2
            ;;
        --source-rpc=*)
            SOURCE_RPC="${1#*=}"
            shift
            ;;
        --validator-rpc)
            VALIDATOR_RPC="$2"
            shift 2
            ;;
        --validator-rpc=*)
            VALIDATOR_RPC="${1#*=}"
            shift
            ;;
        --from-height)
            FROM_HEIGHT="$2"
            shift 2
            ;;
        --from-height=*)
            FROM_HEIGHT="${1#*=}"
            shift
            ;;
        --to-height)
            TO_HEIGHT="$2"
            shift 2
            ;;
        --to-height=*)
            TO_HEIGHT="${1#*=}"
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
        --btcctl)
            BTCCTL_BIN="$2"
            shift 2
            ;;
        --btcctl=*)
            BTCCTL_BIN="${1#*=}"
            shift
            ;;
        --outdir)
            OUT_DIR="$2"
            shift 2
            ;;
        --outdir=*)
            OUT_DIR="${1#*=}"
            shift
            ;;
        --run-id)
            RUN_ID="$2"
            shift 2
            ;;
        --run-id=*)
            RUN_ID="${1#*=}"
            shift
            ;;
        --s3-uri)
            S3_URI="$2"
            shift 2
            ;;
        --s3-uri=*)
            S3_URI="${1#*=}"
            shift
            ;;
        --aws-profile)
            AWS_PROFILE="$2"
            shift 2
            ;;
        --aws-profile=*)
            AWS_PROFILE="${1#*=}"
            shift
            ;;
        --notls)
            NOTLS=1
            shift
            ;;
        --upload)
            UPLOAD=1
            shift
            ;;
        --strict)
            STRICT=1
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

positive_int() {
    [[ "$1" =~ ^[0-9]+$ ]]
}

if [[ -z "${SOURCE_RPC}" || -z "${RPC_USER}" || -z "${RPC_PASS}" ]]; then
    echo "[ERROR] --source-rpc, --rpcuser, and --rpcpass are required" >&2
    exit 1
fi

positive_int "${FROM_HEIGHT}" || {
    echo "[ERROR] --from-height is required and must be a non-negative integer" >&2
    exit 1
}

positive_int "${TO_HEIGHT}" || {
    echo "[ERROR] --to-height is required and must be a non-negative integer" >&2
    exit 1
}

if (( TO_HEIGHT < FROM_HEIGHT )); then
    echo "[ERROR] --to-height must be >= --from-height" >&2
    exit 1
fi

if [[ ! -x "${BTCCTL_BIN}" ]]; then
    echo "[ERROR] btcctl not found or not executable: ${BTCCTL_BIN}" >&2
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "[ERROR] jq is required" >&2
    exit 1
fi

if [[ ${UPLOAD} -eq 1 && -z "${S3_URI}" ]]; then
    echo "[ERROR] --s3-uri is required with --upload" >&2
    exit 1
fi

ts="$(date -u +"%Y%m%dT%H%M%SZ")"
RUN_ID="${RUN_ID:-mainnet72h-reap-956542-${ts}}"
EVIDENCE_DIR="${OUT_DIR%/}/${RUN_ID}/reap-block-validation/${FROM_HEIGHT}-${TO_HEIGHT}-${ts}"
BLOCK_DIR="${EVIDENCE_DIR}/blocks"
mkdir -p "${BLOCK_DIR}"

btcctl_base_args() {
    local rpcserver="$1"
    local args=(
        "--${NETWORK}"
        "--rpcuser=${RPC_USER}"
        "--rpcpass=${RPC_PASS}"
        "--rpcserver=${rpcserver}"
    )
    if [[ ${NOTLS} -eq 1 ]]; then
        args+=(--notls)
    fi
    printf '%s\n' "${args[@]}"
}

rpc() {
    local rpcserver="$1"
    shift
    local args=()
    while IFS= read -r arg; do
        args+=("${arg}")
    done < <(btcctl_base_args "${rpcserver}")
    "${BTCCTL_BIN}" "${args[@]}" "$@"
}

results_jsonl="${EVIDENCE_DIR}/reap-block-validation-results.jsonl"
seen_jsonl="${EVIDENCE_DIR}/reap-blocks-seen.jsonl"
: >"${results_jsonl}"
: >"${seen_jsonl}"

reap_blocks_seen=0
reap_blocks_validated=0
validation_failures=0
first_reap_height=""
last_reap_height=""

for ((height = FROM_HEIGHT; height <= TO_HEIGHT; height++)); do
    block_hash="$(rpc "${SOURCE_RPC}" getblockhash "${height}")"
    raw_hex="$(rpc "${SOURCE_RPC}" getblock "${block_hash}" 0)"
    if [[ "${raw_hex}" != *"524541503a"* ]]; then
        continue
    fi

    reap_blocks_seen=$((reap_blocks_seen + 1))
    [[ -n "${first_reap_height}" ]] || first_reap_height="${height}"
    last_reap_height="${height}"

    block_prefix="${BLOCK_DIR}/${height}-${block_hash}"
    printf '%s\n' "${raw_hex}" >"${block_prefix}.raw.hex"
    rpc "${SOURCE_RPC}" getblock "${block_hash}" 2 >"${block_prefix}.verbose.json"
    rpc "${SOURCE_RPC}" getblock "${block_hash}" 1 >"${block_prefix}.summary.json"

    marker_payloads="$(python3 - <<'PY' "${raw_hex}"
import binascii
import re
import sys
raw = sys.argv[1].strip()
payloads = []
for match in re.finditer("524541503a", raw):
    start = max(0, match.start() - 80)
    end = min(len(raw), match.end() + 160)
    snippet = raw[start:end]
    try:
        decoded = binascii.unhexlify(snippet).decode("utf-8", "ignore")
    except Exception:
        decoded = ""
    payloads.append({"hex_offset": match.start() // 2, "context_hex": snippet, "context_ascii": decoded})
import json
print(json.dumps(payloads, separators=(",", ":")))
PY
)"

    validator_status="not_configured"
    validator_hash=""
    validator_error=""
    if [[ -n "${VALIDATOR_RPC}" ]]; then
        if validator_hash="$(rpc "${VALIDATOR_RPC}" getblockhash "${height}" 2>"${block_prefix}.validator.err")" &&
            [[ "${validator_hash}" == "${block_hash}" ]] &&
            rpc "${VALIDATOR_RPC}" getblock "${block_hash}" 0 >"${block_prefix}.validator.raw.hex" 2>>"${block_prefix}.validator.err"; then
            validator_status="accepted_same_hash"
            reap_blocks_validated=$((reap_blocks_validated + 1))
        else
            validator_status="failed"
            validation_failures=$((validation_failures + 1))
            validator_error="$(cat "${block_prefix}.validator.err" 2>/dev/null || true)"
        fi
    fi

    jq -cn \
        --argjson height "${height}" \
        --arg block_hash "${block_hash}" \
        --arg source_rpc "${SOURCE_RPC}" \
        --arg validator_rpc "${VALIDATOR_RPC}" \
        --arg validator_status "${validator_status}" \
        --arg validator_hash "${validator_hash}" \
        --arg validator_error "${validator_error}" \
        --argjson marker_payloads "${marker_payloads}" \
        '{
          height:$height,
          block_hash:$block_hash,
          source_rpc:$source_rpc,
          validator_rpc:$validator_rpc,
          validator_status:$validator_status,
          validator_hash:$validator_hash,
          validator_error:$validator_error,
          marker_payloads:$marker_payloads
        }' >>"${results_jsonl}"

    jq -cn \
        --argjson height "${height}" \
        --arg block_hash "${block_hash}" \
        --arg raw_block_file "${block_prefix}.raw.hex" \
        --arg verbose_block_file "${block_prefix}.verbose.json" \
        --argjson marker_payloads "${marker_payloads}" \
        '{height:$height,block_hash:$block_hash,raw_block_file:$raw_block_file,verbose_block_file:$verbose_block_file,marker_payloads:$marker_payloads}' >>"${seen_jsonl}"
done

gzip -n -f "${results_jsonl}"
gzip -n -f "${seen_jsonl}"

summary_json="${EVIDENCE_DIR}/reap-block-validation-summary.json"
jq -n \
    --arg run_id "${RUN_ID}" \
    --arg network "${NETWORK}" \
    --arg source_rpc "${SOURCE_RPC}" \
    --arg validator_rpc "${VALIDATOR_RPC}" \
    --arg captured_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --argjson from_height "${FROM_HEIGHT}" \
    --argjson to_height "${TO_HEIGHT}" \
    --argjson reap_blocks_seen "${reap_blocks_seen}" \
    --argjson reap_blocks_validated "${reap_blocks_validated}" \
    --argjson validation_failures "${validation_failures}" \
    --arg first_reap_block_height "${first_reap_height}" \
    --arg last_reap_block_height "${last_reap_height}" \
    '{
      run_id:$run_id,
      network:$network,
      source_rpc:$source_rpc,
      validator_rpc:$validator_rpc,
      captured_at:$captured_at,
      from_height:$from_height,
      to_height:$to_height,
      reap_blocks_seen:$reap_blocks_seen,
      reap_blocks_validated:$reap_blocks_validated,
      validation_failures:$validation_failures,
      first_reap_block_height:(if $first_reap_block_height == "" then null else ($first_reap_block_height|tonumber) end),
      last_reap_block_height:(if $last_reap_block_height == "" then null else ($last_reap_block_height|tonumber) end),
      no_go:($validation_failures > 0)
    }' >"${summary_json}"

report_md="${EVIDENCE_DIR}/reap-block-validation-report-cn.md"
jq -r '
  "# OBTC REAP block validation\n",
  "- Run ID: `\(.run_id)`",
  "- Network: `\(.network)`",
  "- Height range: \(.from_height)-\(.to_height)",
  "- REAP blocks seen: \(.reap_blocks_seen)",
  "- REAP blocks independently confirmed: \(.reap_blocks_validated)",
  "- Validation failures: \(.validation_failures)",
  "- First REAP block: \(.first_reap_block_height // "none")",
  "- Last REAP block: \(.last_reap_block_height // "none")",
  "- No-go: \(.no_go)"
' "${summary_json}" >"${report_md}"

sha_file="${EVIDENCE_DIR}/SHA256SUMS"
(
    cd "${EVIDENCE_DIR}"
    find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum
) >"${sha_file}"

if [[ ${UPLOAD} -eq 1 ]]; then
    aws s3 sync "${EVIDENCE_DIR}/" "${S3_URI%/}/reap-block-validation/${FROM_HEIGHT}-${TO_HEIGHT}-${ts}/" \
        --profile "${AWS_PROFILE}" \
        --sse AES256 \
        --metadata "run-id=${RUN_ID},data-class=PrivateRehearsal" \
        --no-progress
fi

cat "${report_md}"

if [[ ${validation_failures} -gt 0 && ${STRICT} -eq 1 ]]; then
    exit 1
fi
