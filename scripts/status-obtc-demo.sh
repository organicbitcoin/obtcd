#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

OBTC_RPC_HOST="${OBTC_RPC_HOST:-127.0.0.1}"
OBTC_RPC_PORT="${OBTC_RPC_PORT:-29528}"
OBTC_RPC_URL="${OBTC_RPC_URL:-http://${OBTC_RPC_HOST}:${OBTC_RPC_PORT}/}"
OBTC_RPC_USER="${OBTC_RPC_USER:-obtc}"
OBTC_RPC_PASS="${OBTC_RPC_PASS:-obtcpass}"
OBTC_RPC_TIMEOUT="${OBTC_RPC_TIMEOUT:-5}"
OBTC_LAST_REAP_SCAN="${OBTC_LAST_REAP_SCAN:-300}"
OBTC_NETWORK="${OBTC_NETWORK:-obtcregtest}"

BUILD_COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || printf unknown)"

export OBTC_RPC_URL OBTC_RPC_USER OBTC_RPC_PASS OBTC_RPC_TIMEOUT
export OBTC_LAST_REAP_SCAN OBTC_NETWORK BUILD_COMMIT
export OBTC_WALLET_RPC_URL="${OBTC_WALLET_RPC_URL:-}"
export OBTC_WALLET_RPC_USER="${OBTC_WALLET_RPC_USER:-}"
export OBTC_WALLET_RPC_PASS="${OBTC_WALLET_RPC_PASS:-}"

python3 - <<'PY'
import base64
import json
import os
import sys
import urllib.error
import urllib.request


RPC_URL = os.environ["OBTC_RPC_URL"]
RPC_USER = os.environ["OBTC_RPC_USER"]
RPC_PASS = os.environ["OBTC_RPC_PASS"]
TIMEOUT = float(os.environ["OBTC_RPC_TIMEOUT"])
LAST_REAP_SCAN = int(os.environ["OBTC_LAST_REAP_SCAN"])


def rpc(method, params=None, url=RPC_URL, user=RPC_USER, password=RPC_PASS):
    payload = {
        "jsonrpc": "1.0",
        "id": method,
        "method": method,
        "params": [] if params is None else params,
    }
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    token = base64.b64encode(f"{user}:{password}".encode("utf-8")).decode("ascii")
    req.add_header("Authorization", f"Basic {token}")
    with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
        decoded = json.loads(resp.read().decode("utf-8"))
    if decoded.get("error"):
        raise RuntimeError(f"{method}: {decoded['error']}")
    return decoded.get("result")


def optional(method, params=None):
    try:
        return {"ok": True, "result": rpc(method, params)}
    except Exception as exc:
        return {"ok": False, "error": str(exc)}


def marker_payload(script_hex):
    try:
        script = bytes.fromhex(script_hex or "")
    except ValueError:
        return None
    if len(script) < 2 or script[0] != 0x6A:
        return None
    op = script[1]
    idx = 2
    if op <= 75:
        size = op
    elif op == 0x4C and len(script) >= 3:
        size = script[2]
        idx = 3
    elif op == 0x4D and len(script) >= 4:
        size = script[2] | (script[3] << 8)
        idx = 4
    else:
        return None
    payload = script[idx : idx + size]
    try:
        return payload.decode("utf-8")
    except UnicodeDecodeError:
        return None


def scan_last_reap(blocks):
    if blocks <= 0:
        return None
    start = max(0, blocks - LAST_REAP_SCAN + 1)
    for height in range(blocks, start - 1, -1):
        try:
            block_hash = rpc("getblockhash", [height])
            block = rpc("getblock", [block_hash, 2])
        except Exception:
            continue
        txs = block.get("tx")
        if txs is None:
            txs = block.get("rawtx") or []
        for tx in txs[1:]:
            if tx.get("version") != 3:
                continue
            vout = tx.get("vout") or []
            if not vout:
                continue
            marker = marker_payload(vout[-1].get("scriptPubKey", {}).get("hex"))
            if marker and marker.startswith("REAP:"):
                return {
                    "height": height,
                    "block_hash": block_hash,
                    "txid": tx.get("txid"),
                    "vin_count": len(tx.get("vin") or []),
                    "vout_count": len(vout),
                    "marker": marker,
                }
    return None


def wallet_status():
    url = os.environ.get("OBTC_WALLET_RPC_URL")
    user = os.environ.get("OBTC_WALLET_RPC_USER")
    password = os.environ.get("OBTC_WALLET_RPC_PASS")
    if not url or not user or not password:
        return {
            "available": False,
            "reason": "wallet RPC environment not set",
            "auto_renew_status": "unknown",
        }

    def call(method, params=None):
        return rpc(method, params, url=url, user=user, password=password)

    result = {"available": True, "auto_renew_status": "not_exposed_by_legacy_rpc"}
    for method, params, key in [
        ("getwalletinfo", [], "wallet_info"),
        ("getbalance", [], "balance"),
        ("obtc.getexpiry", [20], "expiry"),
    ]:
        try:
            result[key] = call(method, params)
        except Exception as exc:
            result[key] = {"error": str(exc)}
    return result


try:
    chain = rpc("getblockchaininfo")
    peers = optional("getpeerinfo")
    expiry_stats = optional("getexpiryindexstats")
    commitment = optional("getexpirycommitment")
    reap_plan = optional("getreapplan")
    network_info = optional("getnetworkinfo")
    mining_info = optional("getmininginfo")
    mempool_info = optional("getmempoolinfo")

    peer_result = peers.get("result") if peers.get("ok") else []
    peer_count = len(peer_result or [])
    stats_result = expiry_stats.get("result") if expiry_stats.get("ok") else {}
    commitment_result = commitment.get("result") if commitment.get("ok") else {}
    reap_result = reap_plan.get("result") if reap_plan.get("ok") else {}

    out = {
        "build_commit_hash": os.environ["BUILD_COMMIT"],
        "network_requested": os.environ["OBTC_NETWORK"],
        "rpc_url": RPC_URL,
        "chain": chain.get("chain"),
        "current_height": chain.get("blocks"),
        "best_block_hash": chain.get("bestblockhash"),
        "peer_count": peer_count,
        "mempool": mempool_info,
        "expiry_indexed_tip": stats_result.get("tip_height"),
        "expiry_commitment_root": commitment_result.get("root"),
        "expiry_commitment": commitment,
        "reap_plan": reap_plan,
        "expired_candidate_count_if_mined_next": reap_result.get("picked"),
        "last_reap": scan_last_reap(int(chain.get("blocks") or 0)),
        "wallet": wallet_status(),
        "network_params_summary": stats_result.get("network_params"),
        "node_network_info": network_info,
        "mining_info": mining_info,
        "warnings": [],
    }

    if not expiry_stats.get("ok"):
        out["warnings"].append("getexpiryindexstats failed")
    elif stats_result.get("disabled"):
        out["warnings"].append("expiry index is disabled; start node with --expiryindex")
    if not commitment.get("ok"):
        out["warnings"].append("getexpirycommitment failed")
    if not reap_plan.get("ok"):
        out["warnings"].append("getreapplan failed")
    if out["last_reap"] is None:
        out["warnings"].append(
            f"no REAP transaction found in last {LAST_REAP_SCAN} blocks"
        )

    print(json.dumps(out, indent=2, sort_keys=True))
except urllib.error.URLError as exc:
    print(f"status-obtc-demo: RPC connection failed: {exc}", file=sys.stderr)
    sys.exit(1)
except Exception as exc:
    print(f"status-obtc-demo: {exc}", file=sys.stderr)
    sys.exit(1)
PY
