#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

OBTC_DEMO_DIR="${OBTC_DEMO_DIR:-$REPO_ROOT/.obtc-demo-regtest}"
OBTC_RPC_USER="${OBTC_RPC_USER:-obtc}"
OBTC_RPC_PASS="${OBTC_RPC_PASS:-obtcpass}"
OBTC_RPC_PORT="${OBTC_RPC_PORT:-29528}"
OBTC_P2P_PORT="${OBTC_P2P_PORT:-29527}"
RESET="${RESET:-0}"
KEEP_NODE="${KEEP_NODE:-0}"

BIN_DIR="$OBTC_DEMO_DIR/bin"
DATA_DIR="$OBTC_DEMO_DIR/obtcd"
STATE_FILE="$OBTC_DEMO_DIR/devnetsim-state.json"
LOG_FILE="$OBTC_DEMO_DIR/obtcd.log"
PID_FILE="$OBTC_DEMO_DIR/obtcd.pid"

mkdir -p "$OBTC_DEMO_DIR" "$BIN_DIR"

if [[ "$RESET" == "1" ]]; then
  rm -rf "$DATA_DIR" "$STATE_FILE" "$LOG_FILE" "$PID_FILE"
  mkdir -p "$DATA_DIR"
fi

echo "==> Building local demo binaries"
(
  cd "$REPO_ROOT"
  go build -o "$BIN_DIR/btcd" .
  go build -o "$BIN_DIR/btcctl" ./cmd/btcctl
  go build -o "$BIN_DIR/devnetsim" ./cmd/devnetsim
  go build -o "$BIN_DIR/obtc-status" ./cmd/obtc-status
)

MINING_ADDR="$("$BIN_DIR/devnetsim" miningaddr \
  --network obtcregtest \
  --statefile "$STATE_FILE" \
  --seed-tag demo-miner)"

ctl() {
  "$BIN_DIR/btcctl" --obtcregtest \
    --rpcserver "127.0.0.1:${OBTC_RPC_PORT}" \
    --rpcuser "$OBTC_RPC_USER" \
    --rpcpass "$OBTC_RPC_PASS" \
    --notls "$@"
}

node_running() {
  [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" >/dev/null 2>&1
}

stop_node() {
  if [[ "$KEEP_NODE" == "1" ]]; then
    echo "==> KEEP_NODE=1; node left running with pid $(cat "$PID_FILE" 2>/dev/null || true)"
    echo "==> Data dir: $DATA_DIR"
    echo "==> Log file: $LOG_FILE"
    return
  fi
  if node_running; then
    echo "==> Stopping demo node"
    kill "$(cat "$PID_FILE")"
    wait "$(cat "$PID_FILE")" 2>/dev/null || true
  fi
}
trap stop_node EXIT

if ! node_running; then
  mkdir -p "$DATA_DIR"
  echo "==> Starting obtcd regtest node"
  "$BIN_DIR/btcd" \
    --obtcregtest \
    --datadir "$DATA_DIR" \
    --listen "127.0.0.1:${OBTC_P2P_PORT}" \
    --rpclisten "127.0.0.1:${OBTC_RPC_PORT}" \
    --rpcuser "$OBTC_RPC_USER" \
    --rpcpass "$OBTC_RPC_PASS" \
    --txindex \
    --expiryindex \
    --notls \
    --miningaddr "$MINING_ADDR" \
    --debuglevel info \
    >"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
else
  echo "==> Reusing running node pid $(cat "$PID_FILE")"
fi

echo "==> Waiting for RPC on 127.0.0.1:${OBTC_RPC_PORT}"
for _ in $(seq 1 120); do
  if ctl getblockcount >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
ctl getblockcount >/dev/null

mine_to_height() {
  local target="$1"
  local current
  current="$(ctl getblockcount)"
  if (( current < target )); then
    local blocks=$((target - current))
    echo "==> Mining ${blocks} block(s) to height ${target}"
    ctl generate "$blocks" >/dev/null
  fi
}

print_json_rpc() {
  local title="$1"
  shift
  echo
  echo "==> ${title}"
  ctl "$@"
}

echo "==> Demo network: obtcregtest"
echo "==> Mining address: $MINING_ADDR"
echo "==> Demo dir: $OBTC_DEMO_DIR"
echo "==> Regtest expiry window: 144 blocks"
echo "==> Regtest expiry/REAP enable height: 110"
echo "==> Regtest canonical REAP height: 112"
echo "==> Regtest replay protection height: 114"

mine_to_height 120

print_json_rpc "Chain info after activation" getblockchaininfo
print_json_rpc "Expiry index stats" getexpiryindexstats
print_json_rpc "Expiry commitment" getexpirycommitment
print_json_rpc "Near-expiry scan around current height" listexpiring 120 150 20
print_json_rpc "REAP plan before the first coinbase expiry" getreapplan

mine_to_height 144

print_json_rpc "Candidates expiring at next block height 145" listexpiring 145 145 20
print_json_rpc "REAP plan for next block" getreapplan

echo
echo "==> Mining one block to append the planned REAP transaction, if candidates are available"
ctl generate 1 >/dev/null
REAP_HEIGHT="$(ctl getblockcount)"
REAP_HASH="$(ctl getblockhash "$REAP_HEIGHT")"

print_json_rpc "Block at height ${REAP_HEIGHT}" getblock "$REAP_HASH" 2

echo
echo "==> Minimal status JSON"
OBTC_RPC_HOST=127.0.0.1 \
OBTC_RPC_PORT="$OBTC_RPC_PORT" \
OBTC_RPC_USER="$OBTC_RPC_USER" \
OBTC_RPC_PASS="$OBTC_RPC_PASS" \
OBTC_NETWORK=obtcregtest \
"$REPO_ROOT/scripts/status-obtc-demo.sh"

echo
echo "==> Demo complete"
echo "==> Re-run with RESET=1 for a clean chain."
echo "==> Re-run with KEEP_NODE=1 to leave RPC online for manual wallet or status commands."
