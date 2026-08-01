#!/usr/bin/env bash

set -euo pipefail

SERVICE="${SERVICE:-obtc-mainnet72h.service}"
DATA_MOUNT="${DATA_MOUNT:-/mnt/obtc-data}"
INDEX_MOUNT="${INDEX_MOUNT:-/mnt/obtc-expiry-temp}"
DB_DIR="${DB_DIR:-${DATA_MOUNT}/btc-mainnet/mainnet/blocks_ffldb}"
OUT_FILE="${OUT_FILE:-${DATA_MOUNT}/artifacts/expiry-rebuild-monitor.jsonl}"

mkdir -p "$(dirname "${OUT_FILE}")"

captured_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
service_state="$(systemctl is-active "${SERVICE}" 2>/dev/null || true)"
pid="$(systemctl show -p MainPID --value "${SERVICE}" 2>/dev/null || true)"
pid="${pid:-0}"

cpu_percent=null
memory_percent=null
rss_bytes=null
elapsed_seconds=null
if [[ "${pid}" =~ ^[1-9][0-9]*$ ]] && [[ -r "/proc/${pid}/status" ]]; then
    read -r cpu_percent memory_percent rss_kib elapsed_seconds < <(
        ps -p "${pid}" -o %cpu=,%mem=,rss=,etimes= | xargs
    )
    rss_bytes=$((rss_kib * 1024))
fi

read -r disk_total_bytes disk_used_bytes disk_available_bytes < <(
    df -B1 --output=size,used,avail "${DATA_MOUNT}" | tail -1 | xargs
)
index_disk_total_bytes=null
index_disk_used_bytes=null
index_disk_available_bytes=null
if mountpoint -q "${INDEX_MOUNT}"; then
    read -r index_disk_total_bytes index_disk_used_bytes index_disk_available_bytes < <(
        df -B1 --output=size,used,avail "${INDEX_MOUNT}" | tail -1 | xargs
    )
fi
# LevelDB compaction can remove files while du is walking the directory.  Keep
# the sample and mark the size unavailable instead of failing the collector.
db_bytes="$(du -sb "${DB_DIR}" 2>/dev/null | awk '{print $1}' || true)"
metadata_bytes="$(du -sb "${DB_DIR}/metadata" 2>/dev/null | awk '{print $1}' || true)"
db_bytes="${db_bytes:-null}"
metadata_bytes="${metadata_bytes:-null}"
progress_line="$(journalctl -u "${SERVICE}" --since '10 minutes ago' --no-pager \
    | grep 'ExpiryIndex: Cleared\|ExpiryIndex: Processed\|Fast rebuild completed' | tail -1 || true)"
if mountpoint -q "${DB_DIR}/metadata"; then
    metadata_copy_state="cutover_complete"
else
    metadata_copy_state="$(systemctl is-active obtc-expiry-metadata-copy.service 2>/dev/null || true)"
fi

jq -cn \
    --arg captured_at "${captured_at}" \
    --arg service "${SERVICE}" \
    --arg service_state "${service_state}" \
    --arg metadata_copy_state "${metadata_copy_state}" \
    --arg progress_line "${progress_line}" \
    --argjson pid "${pid}" \
    --argjson cpu_percent "${cpu_percent}" \
    --argjson memory_percent "${memory_percent}" \
    --argjson rss_bytes "${rss_bytes}" \
    --argjson elapsed_seconds "${elapsed_seconds}" \
    --argjson disk_total_bytes "${disk_total_bytes}" \
    --argjson disk_used_bytes "${disk_used_bytes}" \
    --argjson disk_available_bytes "${disk_available_bytes}" \
    --argjson index_disk_total_bytes "${index_disk_total_bytes}" \
    --argjson index_disk_used_bytes "${index_disk_used_bytes}" \
    --argjson index_disk_available_bytes "${index_disk_available_bytes}" \
    --argjson db_bytes "${db_bytes}" \
    --argjson metadata_bytes "${metadata_bytes}" \
    '{
      captured_at:$captured_at,
      service:$service,
      service_state:$service_state,
      metadata_copy_state:$metadata_copy_state,
      pid:$pid,
      cpu_percent:$cpu_percent,
      memory_percent:$memory_percent,
      rss_bytes:$rss_bytes,
      elapsed_seconds:$elapsed_seconds,
      disk_total_bytes:$disk_total_bytes,
      disk_used_bytes:$disk_used_bytes,
      disk_available_bytes:$disk_available_bytes,
      index_disk_total_bytes:$index_disk_total_bytes,
      index_disk_used_bytes:$index_disk_used_bytes,
      index_disk_available_bytes:$index_disk_available_bytes,
      db_bytes:$db_bytes,
      metadata_bytes:$metadata_bytes,
      progress_line:$progress_line
    }' >>"${OUT_FILE}"
