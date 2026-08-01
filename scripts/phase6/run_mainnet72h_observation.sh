#!/usr/bin/env bash

set -euo pipefail

RUN_ID="${RUN_ID:-mainnet72h-reap-956542-$(date -u +'%Y%m%dT%H%M%SZ')}"
ARTIFACT_ROOT="${ARTIFACT_ROOT:-/mnt/obtc-data/artifacts}"
RUN_DIR="${RUN_DIR:-${ARTIFACT_ROOT}/${RUN_ID}}"
BTCCTL_BIN="${BTCCTL_BIN:-/mnt/obtc-data/bin/btcctl}"
BTCCTL_CONFIG="${BTCCTL_CONFIG:-/mnt/obtc-data/config/btcctl-obtcmainnet72h.conf}"
MONITOR_BIN="${MONITOR_BIN:-/mnt/obtc-data/bin/monitor-mainnet72h-sync}"
EXPORT_BIN="${EXPORT_BIN:-/mnt/obtc-data/bin/export-reap-block-evidence}"
NODE_SERVICE="${NODE_SERVICE:-obtc-mainnet72h.service}"
RPC_SERVER="${RPC_SERVER:-127.0.0.1:39528}"
FORK_HEIGHT="${FORK_HEIGHT:-956542}"
FORK_HASH="${FORK_HASH:-0000000000000000000200bad2d8d62a198f06b4390e7ca9be8f15581b42102e}"
FIRST_HEIGHT="${FIRST_HEIGHT:-956543}"
ACTIVATION_HEIGHT="${ACTIVATION_HEIGHT:-956566}"
OBSERVATION_SECONDS="${OBSERVATION_SECONDS:-259200}"
SNAPSHOT_INTERVAL_SECONDS="${SNAPSHOT_INTERVAL_SECONDS:-7200}"
REAP_SCAN_INTERVAL_SECONDS="${REAP_SCAN_INTERVAL_SECONDS:-600}"
PRESIGNED_UPLOADS_FILE="${PRESIGNED_UPLOADS_FILE:-}"

STATE_FILE="${RUN_DIR}/controller-state.json"
EVENTS_FILE="${RUN_DIR}/controller-events.jsonl"
BOUNDARY_DIR="${RUN_DIR}/boundaries"
SYSTEM_DIR="${RUN_DIR}/system"
UPLOAD_DIR="${RUN_DIR}/periodic-upload"

for required in "${BTCCTL_BIN}" "${MONITOR_BIN}" "${EXPORT_BIN}"; do
    if [[ ! -x "${required}" ]]; then
        echo "[ERROR] required executable is missing: ${required}" >&2
        exit 1
    fi
done
if [[ ! -r "${BTCCTL_CONFIG}" ]]; then
    echo "[ERROR] btcctl config is not readable: ${BTCCTL_CONFIG}" >&2
    exit 1
fi
if ! [[ "${OBSERVATION_SECONDS}" =~ ^[1-9][0-9]*$ &&
    "${SNAPSHOT_INTERVAL_SECONDS}" =~ ^[1-9][0-9]*$ &&
    "${REAP_SCAN_INTERVAL_SECONDS}" =~ ^[1-9][0-9]*$ ]]; then
    echo "[ERROR] observation intervals must be positive integers" >&2
    exit 1
fi

mkdir -p "${RUN_DIR}" "${BOUNDARY_DIR}" "${SYSTEM_DIR}" "${UPLOAD_DIR}"

rpc() {
    timeout 1800 "${BTCCTL_BIN}" --configfile="${BTCCTL_CONFIG}" "$@"
}

event() {
    local kind="$1"
    local detail="${2:-}"
    jq -cn \
        --arg captured_at "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --arg kind "${kind}" \
        --arg detail "${detail}" \
        '{captured_at:$captured_at,kind:$kind,detail:$detail}' >>"${EVENTS_FILE}"
}

upload_evidence_slot() {
    [[ -n "${PRESIGNED_UPLOADS_FILE}" ]] || return 0
    if [[ ! -r "${PRESIGNED_UPLOADS_FILE}" ]]; then
        event "evidence_upload_failed" "presigned upload file is not readable"
        return 1
    fi

    local slot url key marker cutoff list bundle receipt
    slot="$(jq -r '.next_upload_slot // 0' "${STATE_FILE}")"
    url="$(jq -r --arg slot "${slot}" '.slots[$slot].url // empty' \
        "${PRESIGNED_UPLOADS_FILE}")"
    key="$(jq -r --arg slot "${slot}" '.slots[$slot].key // empty' \
        "${PRESIGNED_UPLOADS_FILE}")"
    if [[ -z "${url}" || -z "${key}" ]]; then
        event "evidence_upload_failed" "no presigned URL for slot ${slot}"
        return 1
    fi

    marker="${UPLOAD_DIR}/last-success.marker"
    [[ -e "${marker}" ]] || touch -d '@0' "${marker}"
    cutoff="$(mktemp "${UPLOAD_DIR}/cutoff.XXXXXX")"
    list="$(mktemp "${UPLOAD_DIR}/files.XXXXXX")"
    bundle="${UPLOAD_DIR}/slot-$(printf '%03d' "${slot}").tar.gz"
    receipt="${UPLOAD_DIR}/slot-$(printf '%03d' "${slot}").receipt.json"

    (
        cd "${ARTIFACT_ROOT}"
        find "${RUN_ID}" -type f -newer "${marker}" \
            ! -path "${RUN_ID}/periodic-upload/*" -print0 >"${list}"
        tar --null --files-from="${list}" -czf "${bundle}"
    )
    sha256sum "${bundle}" >"${bundle}.sha256"

    if ! curl --fail --silent --show-error -X PUT \
        -H 'x-amz-server-side-encryption: AES256' \
        -H "x-amz-tagging: Project=OBTC&RunID=${RUN_ID}&DataClass=PrivateRehearsal" \
        --upload-file "${bundle}" "${url}"; then
        rm -f "${cutoff}" "${list}"
        event "evidence_upload_failed" "slot ${slot}"
        return 1
    fi

    jq -n \
        --argjson slot "${slot}" \
        --arg key "${key}" \
        --arg uploaded_at "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --arg sha256 "$(sha256sum "${bundle}" | awk '{print $1}')" \
        --argjson size_bytes "$(stat -c %s "${bundle}")" \
        '{slot:$slot,key:$key,uploaded_at:$uploaded_at,sha256:$sha256,size_bytes:$size_bytes}' \
        >"${receipt}"
    mv "${cutoff}" "${marker}"
    rm -f "${list}"

    local tmp
    tmp="$(mktemp)"
    jq --argjson next_slot "$((slot + 1))" '.next_upload_slot=$next_slot' \
        "${STATE_FILE}" >"${tmp}"
    mv "${tmp}" "${STATE_FILE}"
    event "evidence_uploaded" "slot ${slot}: ${key}"
}

wait_for_rpc() {
    while ! rpc getblockcount >/dev/null 2>&1; do
        if ! systemctl is-active --quiet "${NODE_SERVICE}"; then
            event "node_inactive" "waiting for RPC"
        fi
        sleep 30
    done
}

capture_monitor() {
    "${MONITOR_BIN}" \
        --network=obtcmainnet72h \
        --notls \
        --btcctl="${BTCCTL_BIN}" \
        --btcctl-config="${BTCCTL_CONFIG}" \
        --outdir="${ARTIFACT_ROOT}" \
        --run-id="${RUN_ID}" \
        --activation-height="${ACTIVATION_HEIGHT}" \
        --node="fullnode-miner-validator|${RPC_SERVER}|miner-validator" \
        >>"${SYSTEM_DIR}/monitor-reports-cn.md" 2>>"${SYSTEM_DIR}/monitor-errors.log" || {
            event "monitor_failed" "see monitor-errors.log"
            return 1
        }
}

capture_boundary() {
    local height="$1"
    local label="$2"
    local receipt="${BOUNDARY_DIR}/${height}-${label}.receipt.json"
    [[ -f "${receipt}" ]] && return 0

    local tip hash confirmed_hash prefix
    tip="$(rpc getblockcount)"
    (( tip >= height )) || return 0
    hash="$(rpc getblockhash "${height}")"
    confirmed_hash="$(rpc getblockhash "${height}")"
    [[ "${hash}" == "${confirmed_hash}" ]]

    prefix="${BOUNDARY_DIR}/${height}-${label}-${hash}"
    rpc getblock "${hash}" 0 >"${prefix}.raw.hex"
    rpc getblock "${hash}" 2 >"${prefix}.verbose.json"
    jq -n \
        --argjson height "${height}" \
        --arg label "${label}" \
        --arg hash "${hash}" \
        --arg captured_at "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        '{height:$height,label:$label,hash:$hash,captured_at:$captured_at,node_served_block:true}' \
        >"${receipt}"
    event "boundary_captured" "${height}:${label}:${hash}"
}

archive_new_reap_blocks() {
    local tip last_height from_height
    tip="$(rpc getblockcount)"
    last_height="$(jq -r '.last_reap_scan_height' "${STATE_FILE}")"
    from_height=$((last_height + 1))
    if (( from_height < ACTIVATION_HEIGHT )); then
        from_height="${ACTIVATION_HEIGHT}"
    fi
    (( tip >= from_height )) || return 0

    "${EXPORT_BIN}" \
        --network=obtcmainnet72h \
        --notls \
        --btcctl="${BTCCTL_BIN}" \
        --btcctl-config="${BTCCTL_CONFIG}" \
        --source-rpc="${RPC_SERVER}" \
        --from-height="${from_height}" \
        --to-height="${tip}" \
        --outdir="${ARTIFACT_ROOT}" \
        --run-id="${RUN_ID}" \
        >>"${SYSTEM_DIR}/reap-export-reports-cn.md" \
        2>>"${SYSTEM_DIR}/reap-export-errors.log"

    local tmp
    tmp="$(mktemp)"
    jq --argjson height "${tip}" '.last_reap_scan_height=$height' \
        "${STATE_FILE}" >"${tmp}"
    mv "${tmp}" "${STATE_FILE}"
    event "reap_range_archived" "${from_height}-${tip}"
}

initialize_observation() {
    local height hash expiry_file reap_file started_epoch deadline_epoch tmp
    height="$(rpc getblockcount)"
    hash="$(rpc getblockhash "${FORK_HEIGHT}")"
    if [[ "${height}" != "${FORK_HEIGHT}" || "${hash}" != "${FORK_HASH}" ]]; then
        event "anchor_mismatch" "height=${height} hash=${hash}"
        echo "[ERROR] rehearsal anchor mismatch" >&2
        exit 1
    fi

    capture_boundary "${FORK_HEIGHT}" "fork-anchor"
    capture_monitor
    expiry_file="$(find "${RUN_DIR}/automation" -name getexpiryindexstats.json -type f | sort | tail -1)"
    reap_file="$(find "${RUN_DIR}/automation" -name getreapplan.json -type f | sort | tail -1)"
    if [[ -z "${expiry_file}" || "$(jq -r '.tip_height' "${expiry_file}")" != "${FORK_HEIGHT}" ]]; then
        event "expiry_tip_mismatch" "expected ${FORK_HEIGHT}"
        echo "[ERROR] expiry index is not at the fork anchor" >&2
        exit 1
    fi
    if [[ -n "${reap_file}" && "$(jq -r '.active' "${reap_file}")" == "true" ]]; then
        event "reap_active_too_early" "fork anchor"
        echo "[ERROR] REAP is active before activation height" >&2
        exit 1
    fi

    started_epoch="$(date -u +%s)"
    deadline_epoch=$((started_epoch + OBSERVATION_SECONDS))
    tmp="$(mktemp)"
    jq -n \
        --arg run_id "${RUN_ID}" \
        --arg started_at "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --argjson started_epoch "${started_epoch}" \
        --argjson deadline_epoch "${deadline_epoch}" \
        --argjson last_reap_scan_height "$((ACTIVATION_HEIGHT - 1))" \
        --argjson next_snapshot_epoch "${started_epoch}" \
        '{
          run_id:$run_id,
          status:"running",
          started_at:$started_at,
          started_epoch:$started_epoch,
          deadline_epoch:$deadline_epoch,
          last_reap_scan_height:$last_reap_scan_height,
          next_snapshot_epoch:$next_snapshot_epoch,
          next_upload_epoch:$started_epoch,
          next_upload_slot:0
        }' >"${tmp}"
    mv "${tmp}" "${STATE_FILE}"

    rpc setgenerate true 1
    event "mining_started" "one CPU worker"
}

finalize_observation() {
    local before_height before_hash after_height after_hash tmp
    rpc setgenerate false 0 || true
    archive_new_reap_blocks || true
    capture_monitor || true

    before_height="$(rpc getblockcount)"
    before_hash="$(rpc getblockhash "${before_height}")"
    systemctl restart "${NODE_SERVICE}"
    wait_for_rpc
    after_height="$(rpc getblockcount)"
    after_hash="$(rpc getblockhash "${after_height}")"
    jq -n \
        --argjson before_height "${before_height}" \
        --arg before_hash "${before_hash}" \
        --argjson after_height "${after_height}" \
        --arg after_hash "${after_hash}" \
        --arg captured_at "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        '{
          captured_at:$captured_at,
          before:{height:$before_height,hash:$before_hash},
          after:{height:$after_height,hash:$after_hash},
          consistent:($before_height==$after_height and $before_hash==$after_hash)
        }' >"${SYSTEM_DIR}/restart-validation.json"

    tmp="$(mktemp)"
    jq --arg finished_at "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        '.status="complete" | .finished_at=$finished_at' "${STATE_FILE}" >"${tmp}"
    mv "${tmp}" "${STATE_FILE}"
    (
        cd "${RUN_DIR}"
        find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum
    ) >"${RUN_DIR}/SHA256SUMS"
    event "observation_complete" "tip=${after_height}:${after_hash}"
    upload_evidence_slot || true
}

wait_for_rpc
if [[ ! -f "${STATE_FILE}" ]]; then
    initialize_observation
else
    event "controller_resumed" "existing state"
    if [[ "$(jq -r '.status' "${STATE_FILE}")" == "complete" ]]; then
        exit 0
    fi
    rpc setgenerate true 1
fi

while (( $(date -u +%s) < $(jq -r '.deadline_epoch' "${STATE_FILE}") )); do
    if rpc getblockcount >/dev/null 2>&1; then
        capture_boundary "${FIRST_HEIGHT}" "first-obtc"
        capture_boundary "$((ACTIVATION_HEIGHT - 1))" "activation-minus-one"
        capture_boundary "${ACTIVATION_HEIGHT}" "activation"
        capture_boundary "$((ACTIVATION_HEIGHT + 1))" "activation-plus-one"

        now="$(date -u +%s)"
        next_snapshot="$(jq -r '.next_snapshot_epoch' "${STATE_FILE}")"
        if (( now >= next_snapshot )); then
            capture_monitor || true
            tmp="$(mktemp)"
            jq --argjson next "$((now + SNAPSHOT_INTERVAL_SECONDS))" \
                '.next_snapshot_epoch=$next' "${STATE_FILE}" >"${tmp}"
            mv "${tmp}" "${STATE_FILE}"
        fi
        archive_new_reap_blocks || event "reap_archive_failed" "retrying later"

        next_upload="$(jq -r '.next_upload_epoch' "${STATE_FILE}")"
        if (( now >= next_upload )); then
            if upload_evidence_slot; then
                tmp="$(mktemp)"
                jq --argjson next "$((now + SNAPSHOT_INTERVAL_SECONDS))" \
                    '.next_upload_epoch=$next' "${STATE_FILE}" >"${tmp}"
                mv "${tmp}" "${STATE_FILE}"
            fi
        fi
    else
        event "rpc_unavailable" "controller loop"
    fi
    sleep "${REAP_SCAN_INTERVAL_SECONDS}"
done

finalize_observation
