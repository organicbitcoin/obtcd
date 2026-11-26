#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

RUN_ID="mainnet72h-controller-test"
ARTIFACT_ROOT="${TMP_DIR}/artifacts"
RUN_DIR="${ARTIFACT_ROOT}/${RUN_ID}"
BIN_DIR="${TMP_DIR}/bin"
mkdir -p "${RUN_DIR}" "${BIN_DIR}" "${TMP_DIR}/config"

cat >"${BIN_DIR}/btcctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
while [[ "${1:-}" == --* ]]; do
    shift
done
command="${1:?missing RPC command}"
shift
case "${command}" in
    getblockcount)
        echo 956567
        ;;
    getblockhash)
        printf '%064d\n' "${1:?missing height}"
        ;;
    getblock)
        if [[ "${2:-}" == "2" ]]; then
            printf '{"hash":"%s","height":956566}\n' "${1}"
        else
            echo 00
        fi
        ;;
    getreapplan)
        echo '{"height":956568,"enabled":true,"active":true,"picked":256}'
        ;;
    setgenerate)
        echo null
        ;;
    *)
        echo "unexpected RPC command: ${command}" >&2
        exit 1
        ;;
esac
EOF

cat >"${BIN_DIR}/monitor" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "monitor captured"
EOF

cat >"${BIN_DIR}/export" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
outdir=""
run_id=""
for arg in "$@"; do
    case "${arg}" in
        --outdir=*) outdir="${arg#*=}" ;;
        --run-id=*) run_id="${arg#*=}" ;;
    esac
done
evidence_dir="${outdir}/${run_id}/reap-block-validation/test"
mkdir -p "${evidence_dir}"
echo '{"reap_blocks_seen":1,"reap_blocks_validated":0,"validation_failures":0,"no_go":false}' \
    >"${evidence_dir}/reap-block-validation-summary.json"
echo "REAP evidence exported"
EOF

cat >"${BIN_DIR}/systemctl" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

cat >"${BIN_DIR}/timeout" <<'EOF'
#!/usr/bin/env bash
shift
exec "$@"
EOF

chmod +x "${BIN_DIR}/btcctl" "${BIN_DIR}/monitor" "${BIN_DIR}/export" \
    "${BIN_DIR}/systemctl" "${BIN_DIR}/timeout"
touch "${TMP_DIR}/config/btcctl.conf"

jq -n \
    --arg run_id "${RUN_ID}" \
    '{
      run_id:$run_id,
      status:"complete",
      started_at:"2026-01-01T00:00:00Z",
      started_epoch:1,
      deadline_epoch:1,
      last_reap_scan_height:956565,
      next_snapshot_epoch:4102444800,
      next_upload_epoch:4102444800,
      next_upload_slot:0,
      finished_at:"2026-01-04T00:00:00Z"
    }' >"${RUN_DIR}/controller-state.json"

PATH="${BIN_DIR}:${PATH}" \
RUN_ID="${RUN_ID}" \
ARTIFACT_ROOT="${ARTIFACT_ROOT}" \
BTCCTL_BIN="${BIN_DIR}/btcctl" \
BTCCTL_CONFIG="${TMP_DIR}/config/btcctl.conf" \
MONITOR_BIN="${BIN_DIR}/monitor" \
EXPORT_BIN="${BIN_DIR}/export" \
REAP_SCAN_INTERVAL_SECONDS=1 \
SNAPSHOT_INTERVAL_SECONDS=1 \
EVIDENCE_UPLOAD_INTERVAL_SECONDS=1 \
CONTINUE_UNTIL_REAP=true \
POST_ACTIVATION_CONFIRMATIONS=1 \
REQUIRED_REAP_BLOCKS=1 \
    "${SCRIPT_DIR}/run_mainnet72h_observation.sh"

jq -e '
  .status == "complete" and
  .continued_until_reap == true and
  (.finished_at | type == "string")
' "${RUN_DIR}/controller-state.json" >/dev/null

jq -e '
  .passed == true and
  .tip_height == 956567 and
  .activation_height == 956566 and
  .reap_blocks_seen == 1 and
  .restart_consistent == true
' "${RUN_DIR}/system/completion-verdict.json" >/dev/null

grep -q '"kind":"observation_extended"' "${RUN_DIR}/controller-events.jsonl"
grep -q '"kind":"observation_complete"' "${RUN_DIR}/controller-events.jsonl"

echo "run_mainnet72h_observation continuation test passed"
