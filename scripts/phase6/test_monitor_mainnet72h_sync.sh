#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$(mktemp -d)"
FAKE_BTCCTL="${WORK_DIR}/btcctl"

printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'method="${!#}"' \
    'case "${method}" in' \
    'getblockchaininfo) printf '\''{"blocks":100,"headers":100,"bestblockhash":"abc"}\n'\'' ;;' \
    'getpeerinfo|getchaintips) printf '\''[]\n'\'' ;;' \
    'getmempoolinfo|getmininginfo) printf '\''{}\n'\'' ;;' \
    'getexpiryindexstats)' \
    '  if [[ "${FAKE_EXPIRY_DISABLED:-0}" == 1 ]]; then' \
    '    printf '\''{"disabled":true}\n'\''' \
    '  else' \
    '    printf '\''{"disabled":false,"tip_height":100,"total_utxos":1}\n'\''' \
    '  fi' \
    '  ;;' \
    'getreapplan) printf '\''{"enabled":true,"active":false,"height":101,"picked":0,"tax_total":0,"est_weight":0}\n'\'' ;;' \
    '*) printf '\''unsupported method: %s\n'\'' "${method}" >&2; exit 1 ;;' \
    'esac' \
    >"${FAKE_BTCCTL}"
chmod 0755 "${FAKE_BTCCTL}"

run_monitor() {
    local run_id="$1"
    local activation_height="$2"
    local disabled="$3"
    local out_dir="${WORK_DIR}/${run_id}"

    FAKE_EXPIRY_DISABLED="${disabled}" \
        "${SCRIPT_DIR}/monitor_mainnet72h_sync.sh" \
        --network=obtcmainnet72h \
        --btcctl="${FAKE_BTCCTL}" \
        --rpcuser=test \
        --rpcpass=test \
        --notls \
        --outdir="${out_dir}" \
        --run-id="${run_id}" \
        --activation-height="${activation_height}" \
        --node='test-node|127.0.0.1:39528|validator' \
        >/dev/null

    find "${out_dir}/${run_id}/automation" -name summary.json -type f | head -1
}

before_summary="$(run_monitor before-activation 101 0)"
jq -e '
  .no_go == false and
  .nodes[0].expiry_disabled == false and
  .nodes[0].reap_enabled == true and
  .nodes[0].reap_active == false
' "${before_summary}" >/dev/null

disabled_summary="$(run_monitor disabled-at-activation 100 1)"
jq -e '
  .no_go == true and
  .expiry_disabled_nodes == ["test-node"] and
  .inactive_after_activation_nodes == ["test-node"]
' "${disabled_summary}" >/dev/null
