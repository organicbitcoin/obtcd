#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEFAULT_BTCCTL="${REPO_ROOT}/cmd/btcctl/btcctl"

NETWORK="${NETWORK:-obtcmainnet72h}"
BTCCTL_BIN="${BTCCTL_BIN:-${DEFAULT_BTCCTL}}"
BTCCTL_CONFIG="${BTCCTL_CONFIG:-}"
RPC_USER="${RPC_USER:-}"
RPC_PASS="${RPC_PASS:-}"
OUT_DIR="${OUT_DIR:-/tmp/obtc-mainnet72h-monitor}"
RUN_ID="${RUN_ID:-}"
S3_URI="${S3_URI:-}"
AWS_PROFILE="${AWS_PROFILE:-obtc-testnet-deployer}"
NOTLS=0
UPLOAD=0
STRICT=0
MAX_LAG_BLOCKS=2
ACTIVATION_HEIGHT=956566
NODES=()

usage() {
    cat <<EOF
Monitor OBTC mainnet72h synchronization, expiryindex, and REAP status.

Usage:
  $0 --rpcuser <user> --rpcpass <pass> --node <name|rpcserver|role> [options]

Options:
  --network <name>             OBTC network (default: obtcmainnet72h)
  --network=<name>
  --node <name|rpcserver|role> Node to check; repeatable
  --node=<name|rpcserver|role>
  --rpcuser <user>             RPC username shared by all nodes
  --rpcuser=<user>
  --rpcpass <pass>             RPC password shared by all nodes
  --rpcpass=<pass>
  --btcctl <path>              btcctl binary path
  --btcctl=<path>
  --btcctl-config <path>       btcctl config containing RPC credentials
  --btcctl-config=<path>
  --outdir <dir>               local evidence directory
  --outdir=<dir>
  --run-id <id>                rehearsal run ID
  --run-id=<id>
  --s3-uri <s3://bucket/prefix> upload destination
  --s3-uri=<s3://bucket/prefix>
  --aws-profile <profile>      AWS profile for upload (default: obtc-testnet-deployer)
  --aws-profile=<profile>
  --activation-height <height> REAP activation height (default: 956566)
  --activation-height=<height>
  --max-lag-blocks <n>         no-go lag threshold (default: 2)
  --max-lag-blocks=<n>
  --notls                      pass --notls to btcctl
  --upload                     upload this monitor snapshot to S3
  --strict                     exit non-zero on no-go conditions
  -h, --help                   show this help

Examples:
  $0 --notls --rpcuser=u --rpcpass=p \\
    --node='miner-1|127.0.0.1:39528|miner' \\
    --node='validator-1|10.0.1.12:39528|validator'
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
        --node)
            NODES+=("$2")
            shift 2
            ;;
        --node=*)
            NODES+=("${1#*=}")
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
        --btcctl-config)
            BTCCTL_CONFIG="$2"
            shift 2
            ;;
        --btcctl-config=*)
            BTCCTL_CONFIG="${1#*=}"
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
        --activation-height)
            ACTIVATION_HEIGHT="$2"
            shift 2
            ;;
        --activation-height=*)
            ACTIVATION_HEIGHT="${1#*=}"
            shift
            ;;
        --max-lag-blocks)
            MAX_LAG_BLOCKS="$2"
            shift 2
            ;;
        --max-lag-blocks=*)
            MAX_LAG_BLOCKS="${1#*=}"
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

case "${NETWORK}" in
    obtcmainnet72h|obtcmainnet|obtctestnet)
        ;;
    *)
        echo "[ERROR] unsupported --network: ${NETWORK}" >&2
        exit 1
        ;;
esac

if [[ ${#NODES[@]} -eq 0 ]]; then
    echo "[ERROR] at least one --node is required" >&2
    usage
    exit 1
fi

if [[ -z "${BTCCTL_CONFIG}" && ( -z "${RPC_USER}" || -z "${RPC_PASS}" ) ]]; then
    echo "[ERROR] --btcctl-config or both --rpcuser and --rpcpass are required" >&2
    exit 1
fi

if [[ -n "${BTCCTL_CONFIG}" && ! -r "${BTCCTL_CONFIG}" ]]; then
    echo "[ERROR] btcctl config is not readable: ${BTCCTL_CONFIG}" >&2
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

positive_int "${ACTIVATION_HEIGHT}" || {
    echo "[ERROR] --activation-height must be a non-negative integer" >&2
    exit 1
}

positive_int "${MAX_LAG_BLOCKS}" || {
    echo "[ERROR] --max-lag-blocks must be a non-negative integer" >&2
    exit 1
}

if [[ ${UPLOAD} -eq 1 && -z "${S3_URI}" ]]; then
    echo "[ERROR] --s3-uri is required with --upload" >&2
    exit 1
fi

ts="$(date -u +"%Y%m%dT%H%M%SZ")"
RUN_ID="${RUN_ID:-mainnet72h-reap-956542-${ts}}"
SNAPSHOT_DIR="${OUT_DIR%/}/${RUN_ID}/automation/${ts}"
RAW_DIR="${SNAPSHOT_DIR}/raw"
mkdir -p "${RAW_DIR}"

btcctl_args() {
    local rpcserver="$1"
    local args=("--${NETWORK}" "--rpcserver=${rpcserver}")
    if [[ -n "${BTCCTL_CONFIG}" ]]; then
        args=("--configfile=${BTCCTL_CONFIG}" "${args[@]}")
    else
        args+=("--rpcuser=${RPC_USER}" "--rpcpass=${RPC_PASS}")
    fi
    if [[ ${NOTLS} -eq 1 ]]; then
        args+=(--notls)
    fi
    printf '%s\n' "${args[@]}"
}

run_rpc() {
    local rpcserver="$1"
    local method="$2"
    local out_file="$3"
    local err_file="$4"
    shift 4
    local args=()
    while IFS= read -r arg; do
        args+=("${arg}")
    done < <(btcctl_args "${rpcserver}")

    local started_ms ended_ms elapsed_ms status
    started_ms="$(python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
    if "${BTCCTL_BIN}" "${args[@]}" "${method}" "$@" >"${out_file}" 2>"${err_file}"; then
        status="ok"
    else
        status="error"
    fi
    ended_ms="$(python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
    elapsed_ms=$((ended_ms - started_ms))
    printf '{"status":"%s","latency_ms":%d}\n' "${status}" "${elapsed_ms}"
}

json_string() {
    jq -Rn --arg v "$1" '$v'
}

node_json_files=()
for node in "${NODES[@]}"; do
    IFS='|' read -r node_name rpcserver role extra <<<"${node}"
    if [[ -z "${node_name}" || -z "${rpcserver}" ]]; then
        echo "[ERROR] --node must use name|rpcserver|role: ${node}" >&2
        exit 1
    fi
    role="${role:-observer}"

    safe_name="$(printf '%s' "${node_name}" | tr -c 'A-Za-z0-9_.-' '_')"
    node_dir="${RAW_DIR}/${safe_name}"
    mkdir -p "${node_dir}"

    method_rows=()
    for method in getblockchaininfo getpeerinfo getchaintips getmempoolinfo getmininginfo getexpiryindexstats getreapplan; do
        out_file="${node_dir}/${method}.json"
        err_file="${node_dir}/${method}.err"
        meta="$(run_rpc "${rpcserver}" "${method}" "${out_file}" "${err_file}")"
        method_rows+=("$(jq -cn \
            --arg method "${method}" \
            --slurpfile meta <(printf '%s\n' "${meta}") \
            --rawfile error "${err_file}" \
            '{method:$method,status:$meta[0].status,latency_ms:$meta[0].latency_ms,error:($error|rtrimstr("\n"))}')")
    done

    node_status="${SNAPSHOT_DIR}/${safe_name}.status.json"
    jq -n \
        --arg name "${node_name}" \
        --arg role "${role}" \
        --arg rpcserver "${rpcserver}" \
        --arg captured_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
        --slurpfile chain "${node_dir}/getblockchaininfo.json" \
        --slurpfile peers "${node_dir}/getpeerinfo.json" \
        --slurpfile expiry "${node_dir}/getexpiryindexstats.json" \
        --slurpfile reap "${node_dir}/getreapplan.json" \
        --argjson methods "[$(IFS=,; echo "${method_rows[*]}")]" \
        '{
          name:$name,
          role:$role,
          rpcserver:$rpcserver,
          captured_at:$captured_at,
          ok:($methods|all(.status=="ok")),
          height:($chain[0].blocks // null),
          headers:($chain[0].headers // null),
          best_hash:($chain[0].bestblockhash // null),
          peer_count:(if ($peers[0]|type)=="array" then ($peers[0]|length) else null end),
          expiry_tip_height:($expiry[0].tip_height // null),
          expiry_total_utxos:($expiry[0].total_utxos // null),
          expiry_disabled:($expiry[0].disabled // null),
          expiry_lag:(if ($chain[0].blocks? != null and $expiry[0].tip_height? != null) then (($chain[0].blocks|tonumber) - ($expiry[0].tip_height|tonumber)) else null end),
          reap_enabled:($reap[0].enabled // null),
          reap_active:($reap[0].active // null),
          reap_height:($reap[0].height // null),
          reap_picked:($reap[0].picked // null),
          reap_tax_total:($reap[0].tax_total // null),
          reap_est_weight:($reap[0].est_weight // null),
          rpc_methods:$methods
        }' >"${node_status}"
    node_json_files+=("${node_status}")
done

nodes_json="${SNAPSHOT_DIR}/nodes.json"
jq -s '.' "${node_json_files[@]}" >"${nodes_json}"

summary_json="${SNAPSHOT_DIR}/summary.json"
jq -n \
    --arg run_id "${RUN_ID}" \
    --arg network "${NETWORK}" \
    --arg captured_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --argjson activation_height "${ACTIVATION_HEIGHT}" \
    --argjson max_lag_blocks "${MAX_LAG_BLOCKS}" \
    --slurpfile nodes "${nodes_json}" '
    def valid_heights: [$nodes[0][] | select(.height != null) | .height];
    def max_height: (valid_heights | if length == 0 then null else max end);
    def min_height: (valid_heights | if length == 0 then null else min end);
    def lagging_nodes:
      if max_height == null then []
      else [$nodes[0][] | select(.height != null and ((max_height - .height) > $max_lag_blocks)) | .name]
      end;
    def same_height_mismatches:
      [
        $nodes[0]
        | group_by(.height)
        | .[]
        | select(.[0].height != null)
        | select(([.[].best_hash] | unique | length) > 1)
        | {height:.[0].height, hashes:([.[].best_hash] | unique), nodes:[.[].name]}
      ];
    def expiry_lagging:
      [$nodes[0][] | select(.expiry_lag != null and .expiry_lag > $max_lag_blocks) | .name];
    def inactive_after_activation:
      [$nodes[0][] | select(.height != null and .height >= $activation_height and .reap_active != true) | .name];
    def rpc_failed:
      [$nodes[0][] | select(.ok != true) | .name];
    {
      run_id:$run_id,
      network:$network,
      captured_at:$captured_at,
      activation_height:$activation_height,
      node_count:($nodes[0]|length),
      max_height:max_height,
      min_height:min_height,
      height_spread:(if max_height == null or min_height == null then null else max_height - min_height end),
      lagging_nodes:lagging_nodes,
      same_height_mismatches:same_height_mismatches,
      expiry_lagging_nodes:expiry_lagging,
      inactive_after_activation_nodes:inactive_after_activation,
      rpc_failed_nodes:rpc_failed,
      no_go:((lagging_nodes|length) > 0 or (same_height_mismatches|length) > 0 or (expiry_lagging|length) > 0 or (inactive_after_activation|length) > 0 or (rpc_failed|length) > 0),
      nodes:$nodes[0]
    }' >"${summary_json}"

report_md="${SNAPSHOT_DIR}/status-report-cn.md"
jq -r '
  "# OBTC mainnet72h 同步监控\n",
  "- Run ID: `\(.run_id)`",
  "- Network: `\(.network)`",
  "- Captured UTC: `\(.captured_at)`",
  "- 节点数: \(.node_count)",
  "- 最高高度: \(.max_height // "unknown")",
  "- 最低高度: \(.min_height // "unknown")",
  "- 高度差: \(.height_spread // "unknown")",
  "- No-go: \(.no_go)",
  "",
  "## 节点状态",
  (.nodes[] | "- \(.name) [\(.role)] height=\(.height // "unknown") peers=\(.peer_count // "unknown") expiry_lag=\(.expiry_lag // "unknown") reap_active=\(.reap_active // "unknown") picked=\(.reap_picked // "unknown") tax=\(.reap_tax_total // "unknown")"),
  "",
  "## 告警",
  (if (.rpc_failed_nodes|length) > 0 then "- RPC 失败节点: \(.rpc_failed_nodes|join(", "))" else "- RPC 失败节点: none" end),
  (if (.lagging_nodes|length) > 0 then "- 高度落后节点: \(.lagging_nodes|join(", "))" else "- 高度落后节点: none" end),
  (if (.expiry_lagging_nodes|length) > 0 then "- ExpiryIndex 落后节点: \(.expiry_lagging_nodes|join(", "))" else "- ExpiryIndex 落后节点: none" end),
  (if (.inactive_after_activation_nodes|length) > 0 then "- Activation 后 REAP inactive 节点: \(.inactive_after_activation_nodes|join(", "))" else "- Activation 后 REAP inactive 节点: none" end),
  (if (.same_height_mismatches|length) > 0 then "- 同高度 hash 分歧: \(.same_height_mismatches|tojson)" else "- 同高度 hash 分歧: none" end)
' "${summary_json}" >"${report_md}"

sha_file="${SNAPSHOT_DIR}/SHA256SUMS"
(
    cd "${SNAPSHOT_DIR}"
    find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum
) >"${sha_file}"

if [[ ${UPLOAD} -eq 1 ]]; then
    aws s3 sync "${SNAPSHOT_DIR}/" "${S3_URI%/}/automation/${ts}/" \
        --profile "${AWS_PROFILE}" \
        --sse AES256 \
        --metadata "run-id=${RUN_ID},data-class=PrivateRehearsal" \
        --no-progress
fi

cat "${report_md}"

if [[ "$(jq -r '.no_go' "${summary_json}")" == "true" && ${STRICT} -eq 1 ]]; then
    exit 1
fi
