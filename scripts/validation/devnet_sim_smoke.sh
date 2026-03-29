#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

BTCCTL="./cmd/btcctl/btcctl"
DEVNET_SCRIPT="./scripts/devnet-up.sh"

btcctl_node1() {
    "$BTCCTL" --obtcregtest --notls \
        --rpcuser=obtc --rpcpass=obtcpass \
        --rpcserver=127.0.0.1:18556 "$@"
}

btcctl_node2() {
    "$BTCCTL" --obtcregtest --notls \
        --rpcuser=obtc --rpcpass=obtcpass \
        --rpcserver=127.0.0.1:18557 "$@"
}

cleanup() {
    "$DEVNET_SCRIPT" stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

json_field() {
    local field="$1"
    python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field"
}

count_block_txs() {
    local block_hash="$1"
    python3 - "$block_hash" <<'PY'
import json,subprocess,sys
block_hash = sys.argv[1]
out = subprocess.check_output([
    './cmd/btcctl/btcctl',
    '--obtcregtest','--notls',
    '--rpcuser=obtc','--rpcpass=obtcpass',
    '--rpcserver=127.0.0.1:18556',
    'getblock', block_hash, '1',
], text=True)
block = json.loads(out)
print(len(block['tx']))
PY
}

has_multi_input_tx() {
    python3 <<'PY'
import json, subprocess
mempool = subprocess.check_output([
    './cmd/btcctl/btcctl',
    '--obtcregtest','--notls',
    '--rpcuser=obtc','--rpcpass=obtcpass',
    '--rpcserver=127.0.0.1:18556',
    'getrawmempool',
], text=True)
found = False
for txid in json.loads(mempool):
    raw = subprocess.check_output([
        './cmd/btcctl/btcctl',
        '--obtcregtest','--notls',
        '--rpcuser=obtc','--rpcpass=obtcpass',
        '--rpcserver=127.0.0.1:18556',
        'getrawtransaction', txid, '1',
    ], text=True)
    tx = json.loads(raw)
    if len(tx['vin']) > 1:
        found = True
        break
print('true' if found else 'false')
PY
}

block_has_reap_marker() {
    local block_hash="$1"
    python3 - "$block_hash" <<'PY'
import json, subprocess, sys
block_hash = sys.argv[1]
block = json.loads(subprocess.check_output([
    './cmd/btcctl/btcctl',
    '--obtcregtest','--notls',
    '--rpcuser=obtc','--rpcpass=obtcpass',
    '--rpcserver=127.0.0.1:18556',
    'getblock', block_hash, '2',
], text=True))
for tx in (block.get('rawtx') or block.get('tx') or []):
    if tx.get('version') != 3:
        continue
    for vout in tx.get('vout', []):
        asm = vout.get('scriptPubKey', {}).get('asm', '')
        parts = asm.split()
        if not parts or parts[0] != 'OP_RETURN' or len(parts) < 2:
            continue
        try:
            payload = bytes.fromhex(parts[1]).decode('utf-8', 'ignore')
        except Exception:
            payload = ''
        if payload.startswith('REAP:'):
            print('true')
            sys.exit(0)
print('false')
PY
}

echo "[smoke] starting OBTC devnet"
"$DEVNET_SCRIPT" start >/tmp/devnet-start.out
cat /tmp/devnet-start.out

echo "[smoke] explicit OBTC validation"
"$DEVNET_SCRIPT" validate-obtc >/tmp/devnet-validate.out
cat /tmp/devnet-validate.out

echo "[smoke] confirm OBTC RPCs are active"
expiry_disabled="$(btcctl_node1 getexpiryindexstats | json_field disabled)"
commitment_active="$(btcctl_node1 getexpirycommitment | json_field active)"
reap_active="$(btcctl_node1 getreapplan | json_field active)"
reap_picked="$(btcctl_node1 getreapplan | json_field picked)"
if [ "$expiry_disabled" != "False" ] && [ "$expiry_disabled" != "false" ]; then
    echo "expected expiryindex to be enabled" >&2
    exit 1
fi
if [ "$commitment_active" != "True" ] && [ "$commitment_active" != "true" ]; then
    echo "expected expiry commitment to be active" >&2
    exit 1
fi
if [ "$reap_active" != "True" ] && [ "$reap_active" != "true" ]; then
    echo "expected REAP plan to be active" >&2
    exit 1
fi
if [ "$reap_picked" -le 0 ]; then
    echo "expected REAP plan to pick at least one expiring input" >&2
    exit 1
fi

echo "[smoke] prepare primary utxo pool"
"$DEVNET_SCRIPT" prepare 64 300000 >/tmp/devnet-prepare.out
cat /tmp/devnet-prepare.out

echo "[smoke] prepare peer utxo pool"
"$DEVNET_SCRIPT" prepare-peer 24 220000 >/tmp/devnet-prepare-peer.out
cat /tmp/devnet-prepare-peer.out

echo "[smoke] inject primary feemarket traffic"
"$DEVNET_SCRIPT" spam --count 12 --mode feemarket --value 150000 >/tmp/devnet-feemarket.out
cat /tmp/devnet-feemarket.out

echo "[smoke] inject peer mixed traffic"
"$DEVNET_SCRIPT" spam-peer --count 8 --mode mixed --value 110000 >/tmp/devnet-peer-mixed.out
cat /tmp/devnet-peer-mixed.out

sleep 1
mempool_size="$(btcctl_node1 getmempoolinfo | json_field size)"
node2_mempool_size="$(btcctl_node2 getmempoolinfo | json_field size)"
if [ "$mempool_size" -lt 20 ]; then
    echo "expected at least 20 mempool txs after multi-source traffic, got $mempool_size" >&2
    exit 1
fi
if [ "$node2_mempool_size" -lt 20 ]; then
    echo "expected peer node to observe relayed mempool traffic, got $node2_mempool_size" >&2
    exit 1
fi

echo "[smoke] inject conflict traffic"
"$DEVNET_SCRIPT" spam --count 4 --mode conflict >/tmp/devnet-conflict.out
cat /tmp/devnet-conflict.out
if ! grep -q '^rejected=4$' /tmp/devnet-conflict.out; then
    echo "expected 4 rejected double-spend attempts" >&2
    exit 1
fi

echo "[smoke] inject consolidation traffic"
"$DEVNET_SCRIPT" spam --count 2 --mode consolidate >/tmp/devnet-consolidate.out
cat /tmp/devnet-consolidate.out
if [ "$(has_multi_input_tx)" != "true" ]; then
    echo "expected at least one multi-input transaction in mempool" >&2
    exit 1
fi

echo "[smoke] mine one block and verify it is non-empty + contains REAP"
"$DEVNET_SCRIPT" mine 1 >/tmp/devnet-mine.out
cat /tmp/devnet-mine.out
best_hash="$(btcctl_node1 getbestblockhash)"
block_txs="$(count_block_txs "$best_hash")"
if [ "$block_txs" -le 1 ]; then
    echo "expected mined block to contain non-coinbase transactions, got $block_txs txs" >&2
    exit 1
fi
if [ "$(block_has_reap_marker "$best_hash")" != "true" ]; then
    echo "expected mined block to contain a REAP marker transaction" >&2
    exit 1
fi

echo "[smoke] restart devnet and verify peer wallet persistence + OBTC validation"
"$DEVNET_SCRIPT" stop >/tmp/devnet-stop.out
cat /tmp/devnet-stop.out
"$DEVNET_SCRIPT" restart >/tmp/devnet-restart.out
cat /tmp/devnet-restart.out
"$DEVNET_SCRIPT" validate-obtc >/tmp/devnet-validate-restart.out
cat /tmp/devnet-validate-restart.out
"$DEVNET_SCRIPT" spam-peer --count 3 --mode simple --value 90000 >/tmp/devnet-peer-after-restart.out
cat /tmp/devnet-peer-after-restart.out
if ! grep -q '^accepted=3$' /tmp/devnet-peer-after-restart.out; then
    echo "expected peer wallet to continue spending after restart" >&2
    exit 1
fi

echo "[smoke] PASS mempool_size=${mempool_size} node2_mempool_size=${node2_mempool_size} mined_block_txs=${block_txs} reap_picked=${reap_picked}"
