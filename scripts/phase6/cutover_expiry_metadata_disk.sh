#!/usr/bin/env bash

set -euo pipefail

NODE_SERVICE="${NODE_SERVICE:-obtc-mainnet72h.service}"
SOURCE_DIR="${SOURCE_DIR:-/mnt/obtc-data/btc-mainnet/mainnet/blocks_ffldb/metadata}"
INDEX_MOUNT="${INDEX_MOUNT:-/mnt/obtc-expiry-temp}"
TARGET_DIR="${TARGET_DIR:-${INDEX_MOUNT}/metadata}"
RECEIPT_FILE="${RECEIPT_FILE:-${INDEX_MOUNT}/metadata-cutover-receipt.json}"

if [[ "${EUID}" -ne 0 ]]; then
    echo "[ERROR] expiry metadata cutover must run as root" >&2
    exit 1
fi
if ! mountpoint -q "${INDEX_MOUNT}"; then
    echo "[ERROR] index disk is not mounted: ${INDEX_MOUNT}" >&2
    exit 1
fi
if [[ ! -d "${SOURCE_DIR}" || ! -d "${TARGET_DIR}" ]]; then
    echo "[ERROR] source or target metadata directory is missing" >&2
    exit 1
fi

started_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
systemctl stop "${NODE_SERVICE}"
if systemctl is-active --quiet "${NODE_SERVICE}"; then
    echo "[ERROR] node did not stop cleanly" >&2
    exit 1
fi

# The first pass is performed while the node runs.  This final stopped-node
# pass makes the LevelDB copy consistent before the bind mount is activated.
rsync -a --delete "${SOURCE_DIR}/" "${TARGET_DIR}/"
sync

if ! mountpoint -q "${SOURCE_DIR}"; then
    mount --bind "${TARGET_DIR}" "${SOURCE_DIR}"
fi
if ! mountpoint -q "${SOURCE_DIR}"; then
    echo "[ERROR] metadata bind mount was not established" >&2
    exit 1
fi

source_device="$(findmnt -n -o SOURCE --target "${SOURCE_DIR}")"
target_device="$(findmnt -n -o SOURCE --target "${TARGET_DIR}")"
source_device_id="$(findmnt -n -o MAJ:MIN --target "${SOURCE_DIR}")"
target_device_id="$(findmnt -n -o MAJ:MIN --target "${TARGET_DIR}")"
if [[ "${source_device_id}" != "${target_device_id}" ]]; then
    echo "[ERROR] metadata source and target resolve to different devices" >&2
    exit 1
fi

finished_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
jq -n \
    --arg started_at "${started_at}" \
    --arg finished_at "${finished_at}" \
    --arg node_service "${NODE_SERVICE}" \
    --arg source_dir "${SOURCE_DIR}" \
    --arg target_dir "${TARGET_DIR}" \
    --arg source_device "${source_device}" \
    --arg target_device "${target_device}" \
    --arg device_id "${source_device_id}" \
    '{
      started_at:$started_at,
      finished_at:$finished_at,
      node_service:$node_service,
      source_dir:$source_dir,
      target_dir:$target_dir,
      source_device:$source_device,
      target_device:$target_device,
      device_id:$device_id,
      final_rsync_complete:true,
      bind_mount_verified:true
    }' >"${RECEIPT_FILE}"

systemctl start "${NODE_SERVICE}"
echo "[OK] expiry metadata cutover complete"
