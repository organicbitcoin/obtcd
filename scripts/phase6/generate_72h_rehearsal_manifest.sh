#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

RUN_ID=""
FORK_HEIGHT="956542"
FORK_HASH="0000000000000000000200bad2d8d62a198f06b4390e7ca9be8f15581b42102e"
ACTIVATION_HEIGHT=""
START_UTC=""
END_UTC=""
RAW_ARTIFACT_URI=""
CONTROL_PLANE_DIR="${CONTROL_PLANE_DIR:-/Users/pengyu/src/obtc-control-plane}"
OUT_FILE=""
NODES=()

usage() {
    cat <<EOF
Generate a redacted manifest for a private OBTC mainnet 72h REAP rehearsal.

Usage:
  $0 --run-id <id> --raw-artifact-uri <s3://...> --out <manifest.json> [options]

Options:
  --fork-height <height>       BTC fork anchor height (default: 956542)
  --fork-hash <hash>           BTC fork anchor hash
  --activation-height <height> Activation height (default: fork height + 24)
  --start-utc <timestamp>      Observation start timestamp
  --end-utc <timestamp>        Observation end timestamp
  --node <value>               Redacted node descriptor; may repeat
  --control-plane-dir <path>   control-plane repo path

Node descriptors are free-form redacted strings, for example:
  seed-1|observer-miner|aws-eu-north-1|p2p-private|rpc-private
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --run-id)
            RUN_ID="$2"
            shift 2
            ;;
        --run-id=*)
            RUN_ID="${1#*=}"
            shift
            ;;
        --fork-height)
            FORK_HEIGHT="$2"
            shift 2
            ;;
        --fork-height=*)
            FORK_HEIGHT="${1#*=}"
            shift
            ;;
        --fork-hash)
            FORK_HASH="$2"
            shift 2
            ;;
        --fork-hash=*)
            FORK_HASH="${1#*=}"
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
        --start-utc)
            START_UTC="$2"
            shift 2
            ;;
        --start-utc=*)
            START_UTC="${1#*=}"
            shift
            ;;
        --end-utc)
            END_UTC="$2"
            shift 2
            ;;
        --end-utc=*)
            END_UTC="${1#*=}"
            shift
            ;;
        --raw-artifact-uri)
            RAW_ARTIFACT_URI="$2"
            shift 2
            ;;
        --raw-artifact-uri=*)
            RAW_ARTIFACT_URI="${1#*=}"
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
        --control-plane-dir)
            CONTROL_PLANE_DIR="$2"
            shift 2
            ;;
        --control-plane-dir=*)
            CONTROL_PLANE_DIR="${1#*=}"
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

if [[ -z "${RUN_ID}" ]]; then
    echo "[ERROR] --run-id is required" >&2
    exit 1
fi

if [[ -z "${RAW_ARTIFACT_URI}" ]]; then
    echo "[ERROR] --raw-artifact-uri is required" >&2
    exit 1
fi

if [[ -z "${OUT_FILE}" ]]; then
    echo "[ERROR] --out is required" >&2
    exit 1
fi

if [[ -z "${ACTIVATION_HEIGHT}" ]]; then
    ACTIVATION_HEIGHT=$((FORK_HEIGHT + 24))
fi

obtcd_commit="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
obtcd_branch="$(git -C "${REPO_ROOT}" branch --show-current || true)"
if [[ -d "${CONTROL_PLANE_DIR}/.git" ]]; then
    control_plane_commit="$(git -C "${CONTROL_PLANE_DIR}" rev-parse HEAD)"
    control_plane_branch="$(git -C "${CONTROL_PLANE_DIR}" branch --show-current || true)"
else
    control_plane_commit=""
    control_plane_branch=""
fi

mkdir -p "$(dirname "${OUT_FILE}")"

python3 - <<'PY' \
    "${OUT_FILE}" "${RUN_ID}" "${FORK_HEIGHT}" "${FORK_HASH}" \
    "${ACTIVATION_HEIGHT}" "${START_UTC}" "${END_UTC}" \
    "${RAW_ARTIFACT_URI}" "${obtcd_commit}" "${obtcd_branch}" \
    "${control_plane_commit}" "${control_plane_branch}" "${NODES[@]}"
import json
import sys
from datetime import datetime, timezone

(
    out_file,
    run_id,
    fork_height,
    fork_hash,
    activation_height,
    start_utc,
    end_utc,
    raw_artifact_uri,
    obtcd_commit,
    obtcd_branch,
    control_plane_commit,
    control_plane_branch,
    *nodes,
) = sys.argv[1:]

manifest = {
    "schema": "obtc-mainnet-72h-reap-rehearsal-manifest-v1",
    "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "run_id": run_id,
    "network": "obtcmainnet72h",
    "privacy": "redacted; no RPC passwords, private keys, raw txid/vout dumps, or secret endpoints",
    "fork": {
        "height": int(fork_height),
        "hash": fork_hash,
        "first_obtc_height": int(fork_height) + 1,
        "activation_height": int(activation_height),
        "replay_protection_height": int(fork_height) + 1,
    },
    "params": {
        "window_blocks": 362880,
        "reap_max_inputs": 256,
        "reap_dust_max_inputs": 1024,
        "reap_max_weight": 400000,
        "reap_tax_numerator": 30,
        "reap_tax_denominator": 100,
        "reap_dust_threshold_sat": 720,
    },
    "observation": {
        "start_utc": start_utc,
        "end_utc": end_utc,
        "schedule": "hourly snapshots for 72h plus activation-boundary snapshots",
        "nodes": nodes,
    },
    "artifacts": {
        "raw_private_uri": raw_artifact_uri,
        "retention": "private S3 raw artifacts plus control-plane manifest/report/checksums",
        "expected_tags": {
            "Project": "OBTC",
            "RunID": run_id,
            "DataClass": "PrivateRehearsal",
        },
    },
    "commits": {
        "obtcd": {"commit": obtcd_commit, "branch": obtcd_branch},
        "obtc_control_plane": {
            "commit": control_plane_commit,
            "branch": control_plane_branch,
        },
    },
    "limitations": [
        "This is a private rehearsal network and does not close the formal mainnet-candidate 72h evidence gate by itself.",
        "Operators must verify the BTC fork anchor against local BTC data before launch.",
    ],
}

with open(out_file, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2, sort_keys=True)
    f.write("\n")
PY

echo "[OK] redacted manifest written to ${OUT_FILE}"
