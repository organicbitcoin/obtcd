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
process_read_bytes=null
process_write_bytes=null
process_cancelled_write_bytes=null
if [[ "${pid}" =~ ^[1-9][0-9]*$ ]] && [[ -r "/proc/${pid}/status" ]]; then
    read -r cpu_percent memory_percent rss_kib elapsed_seconds < <(
        ps -p "${pid}" -o %cpu=,%mem=,rss=,etimes= | xargs
    )
    rss_bytes=$((rss_kib * 1024))
    if [[ -r "/proc/${pid}/io" ]]; then
        process_read_bytes="$(awk '$1=="read_bytes:" {print $2}' "/proc/${pid}/io")"
        process_write_bytes="$(awk '$1=="write_bytes:" {print $2}' "/proc/${pid}/io")"
        process_cancelled_write_bytes="$(awk '$1=="cancelled_write_bytes:" {print $2}' "/proc/${pid}/io")"
    fi
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
index_device="$(findmnt -n -o SOURCE --target "${INDEX_MOUNT}" 2>/dev/null || true)"
index_device="${index_device%%[*}"
index_device="${index_device##*/}"
index_device_stats=null
if [[ -n "${index_device}" ]]; then
    diskstats_line="$(awk -v device="${index_device}" '$3==device {print}' /proc/diskstats)"
    if [[ -n "${diskstats_line}" ]]; then
        index_device_stats="$(jq -cn --arg line "${diskstats_line}" '
          ($line|split(" ")|map(select(length>0))) as $v |
          {
            reads_completed:($v[3]|tonumber),
            sectors_read:($v[5]|tonumber),
            read_time_ms:($v[6]|tonumber),
            writes_completed:($v[7]|tonumber),
            sectors_written:($v[9]|tonumber),
            write_time_ms:($v[10]|tonumber),
            io_in_progress:($v[11]|tonumber),
            io_time_ms:($v[12]|tonumber),
            weighted_io_time_ms:($v[13]|tonumber)
          }')"
    fi
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
    --argjson process_read_bytes "${process_read_bytes}" \
    --argjson process_write_bytes "${process_write_bytes}" \
    --argjson process_cancelled_write_bytes "${process_cancelled_write_bytes}" \
    --argjson disk_total_bytes "${disk_total_bytes}" \
    --argjson disk_used_bytes "${disk_used_bytes}" \
    --argjson disk_available_bytes "${disk_available_bytes}" \
    --argjson index_disk_total_bytes "${index_disk_total_bytes}" \
    --argjson index_disk_used_bytes "${index_disk_used_bytes}" \
    --argjson index_disk_available_bytes "${index_disk_available_bytes}" \
    --arg index_device "${index_device}" \
    --argjson index_device_stats "${index_device_stats}" \
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
      process_read_bytes:$process_read_bytes,
      process_write_bytes:$process_write_bytes,
      process_cancelled_write_bytes:$process_cancelled_write_bytes,
      disk_total_bytes:$disk_total_bytes,
      disk_used_bytes:$disk_used_bytes,
      disk_available_bytes:$disk_available_bytes,
      index_disk_total_bytes:$index_disk_total_bytes,
      index_disk_used_bytes:$index_disk_used_bytes,
      index_disk_available_bytes:$index_disk_available_bytes,
      index_device:$index_device,
      index_device_stats:$index_device_stats,
      db_bytes:$db_bytes,
      metadata_bytes:$metadata_bytes,
      progress_line:$progress_line
    }' >>"${OUT_FILE}"
