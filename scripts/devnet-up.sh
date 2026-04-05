#!/bin/bash

# OBTC DevNet control script.
# Starts a configurable local OBTC devnet and provides traffic/mempool helpers.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BTCD_BINARY="./btcd"
BTCCTL_BINARY="./cmd/btcctl/btcctl"
DEVNETSIM_BINARY="./cmd/devnetsim/devnetsim"

DATA_DIR="${DEVNET_DATA_DIR:-$(pwd)/devnet-data}"
SIMULATOR_DIR="$DATA_DIR/devnetsim"
PRIMARY_SIMULATOR_STATE="$SIMULATOR_DIR/state.json"
PEER_SIMULATOR_STATE="$SIMULATOR_DIR/peer-state.json"
MANIFEST_FILE="$DATA_DIR/manifest.json"

NETWORK="${DEVNET_NETWORK:-obtcregtest}"
DEVNET_NODE_COUNT="${DEVNET_NODE_COUNT:-3}"
DEVNET_MAX_NODES=5
DEVNET_MIN_NODES=2
DEVNET_RPC_BASE_PORT="${DEVNET_RPC_BASE_PORT:-18556}"
DEVNET_P2P_BASE_PORT="${DEVNET_P2P_BASE_PORT:-19555}"

PRIMARY_NODE_INDEX=1
PEER_NODE_INDEX=2
OBTC_BOOTSTRAP_HEIGHT=145

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

binary_needs_rebuild() {
    local binary="$1"
    local source_dir="$2"

    if [ ! -x "$binary" ]; then
        return 0
    fi

    if find "$source_dir" -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) \
        -newer "$binary" -print -quit | grep -q .; then
        return 0
    fi

    return 1
}

ensure_helper_binary() {
    local name="$1"
    local binary="$2"
    local source_dir="$3"

    if binary_needs_rebuild "$binary" "$source_dir"; then
        print_status "Building ${name}..."
        (cd "$source_dir" && go build -o "$(basename "$binary")")
    fi
}

cleanup() {
    print_warning "Cleaning up DevNet data..."
    stop_nodes || true
    remove_data_dir || true
}

trap cleanup SIGINT SIGTERM

validate_node_count() {
    if ! [[ "$DEVNET_NODE_COUNT" =~ ^[0-9]+$ ]]; then
        print_error "DEVNET_NODE_COUNT must be an integer"
        exit 1
    fi

    if [ "$DEVNET_NODE_COUNT" -lt "$DEVNET_MIN_NODES" ] ||
        [ "$DEVNET_NODE_COUNT" -gt "$DEVNET_MAX_NODES" ]; then
        print_error "DEVNET_NODE_COUNT must be between ${DEVNET_MIN_NODES} and ${DEVNET_MAX_NODES}"
        exit 1
    fi
}

is_obtc_network() {
    case "$NETWORK" in
        obtcregtest|obtctestnet|obtcmainnet)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

network_node_flag() {
    if is_obtc_network; then
        echo "--expiryindex"
    fi
}

node_name() {
    printf "node%s" "$1"
}

node_role() {
    case "$1" in
        1)
            echo "miner"
            ;;
        2)
            echo "peer"
            ;;
        *)
            echo "relay"
            ;;
    esac
}

node_dir() {
    printf "%s/%s" "$DATA_DIR" "$(node_name "$1")"
}

node_pid_file() {
    printf "%s/node.pid" "$(node_dir "$1")"
}

node_rpc_port() {
    echo $((DEVNET_RPC_BASE_PORT + $1 - 1))
}

node_p2p_port() {
    echo $((DEVNET_P2P_BASE_PORT + $1 - 1))
}

btcctl_node() {
    local node_index="$1"
    shift || true

    ensure_helper_binary "btcctl" "cmd/btcctl/btcctl" "cmd/btcctl"

    "$BTCCTL_BINARY" --notls --"$NETWORK" \
        --rpcuser=obtc --rpcpass=obtcpass \
        --rpcserver=127.0.0.1:$(node_rpc_port "$node_index") "$@"
}

btcctl_node1() {
    btcctl_node 1 "$@"
}

btcctl_node2() {
    btcctl_node 2 "$@"
}

run_devnetsim() {
    local command="$1"
    shift || true

    ensure_helper_binary "devnetsim" "cmd/devnetsim/devnetsim" "cmd/devnetsim"

    local args=(
        "$DEVNETSIM_BINARY" "$command"
        --network="$NETWORK"
        --statefile="$PRIMARY_SIMULATOR_STATE"
        --seed-tag=primary
        "--rpcserver=127.0.0.1:$(node_rpc_port "$PRIMARY_NODE_INDEX")"
        --rpcuser=obtc
        --rpcpass=obtcpass
    )

    if [ "$DEVNET_NODE_COUNT" -ge "$PEER_NODE_INDEX" ]; then
        args+=(
            --mirror-rpcserver="127.0.0.1:$(node_rpc_port "$PEER_NODE_INDEX")"
            --mirror-rpcuser=obtc
            --mirror-rpcpass=obtcpass
        )
    fi

    "${args[@]}" "$@"
}

run_devnetsim_peer() {
    local command="$1"
    shift || true

    ensure_helper_binary "devnetsim" "cmd/devnetsim/devnetsim" "cmd/devnetsim"

    local args=(
        "$DEVNETSIM_BINARY" "$command"
        --network="$NETWORK"
        --statefile="$PEER_SIMULATOR_STATE"
        --seed-tag=peer
        --rpcserver="127.0.0.1:$(node_rpc_port "$PEER_NODE_INDEX")"
        --rpcuser=obtc
        --rpcpass=obtcpass
        --mirror-rpcserver="127.0.0.1:$(node_rpc_port "$PRIMARY_NODE_INDEX")"
        --mirror-rpcuser=obtc
        --mirror-rpcpass=obtcpass
    )

    "${args[@]}" "$@"
}

get_mining_address() {
    ensure_helper_binary "devnetsim" "cmd/devnetsim/devnetsim" "cmd/devnetsim"

    "$DEVNETSIM_BINARY" miningaddr \
        --network="$NETWORK" \
        --statefile="$PRIMARY_SIMULATOR_STATE" \
        --seed-tag=primary
}

check_binaries() {
    print_status "Checking binaries..."

    if [ ! -f "$BTCD_BINARY" ]; then
        print_error "btcd binary not found. Please run: go build"
        exit 1
    fi

    if [ ! -d "cmd/btcctl" ]; then
        print_error "cmd/btcctl directory not found"
        exit 1
    fi

    if [ ! -d "cmd/devnetsim" ]; then
        print_error "cmd/devnetsim directory not found"
        exit 1
    fi

    print_success "Required source directories found"
}

build_tools() {
    print_status "Building btcctl..."
    (cd cmd/btcctl && go build -o btcctl)

    print_status "Building devnetsim..."
    (cd cmd/devnetsim && go build -o devnetsim)

    print_success "Helper tools built successfully"
}

write_manifest() {
    DEVNET_NODE_COUNT="$DEVNET_NODE_COUNT" \
    DEVNET_RPC_BASE_PORT="$DEVNET_RPC_BASE_PORT" \
    DEVNET_P2P_BASE_PORT="$DEVNET_P2P_BASE_PORT" \
    DATA_DIR="$DATA_DIR" \
    NETWORK="$NETWORK" \
    MANIFEST_FILE="$MANIFEST_FILE" \
    python3 <<'PY'
import json
import os
from datetime import datetime, timezone

count = int(os.environ["DEVNET_NODE_COUNT"])
rpc_base = int(os.environ["DEVNET_RPC_BASE_PORT"])
p2p_base = int(os.environ["DEVNET_P2P_BASE_PORT"])
data_dir = os.environ["DATA_DIR"]
network = os.environ["NETWORK"]
manifest_file = os.environ["MANIFEST_FILE"]

nodes = []
for idx in range(1, count + 1):
    if idx == 1:
        role = "miner"
    elif idx == 2:
        role = "peer"
    else:
        role = "relay"

    nodes.append({
        "index": idx,
        "name": f"node{idx}",
        "role": role,
        "rpc_server": f"127.0.0.1:{rpc_base + idx - 1}",
        "p2p_server": f"127.0.0.1:{p2p_base + idx - 1}",
        "data_dir": os.path.join(data_dir, f"node{idx}"),
    })

payload = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "network": network,
    "node_count": count,
    "data_dir": data_dir,
    "nodes": nodes,
}

with open(manifest_file, "w", encoding="utf-8") as f:
    json.dump(payload, f, indent=2)
    f.write("\n")
PY
}

setup_directories() {
    print_status "Preparing DevNet directories..."
    rm -rf "$DATA_DIR"
    mkdir -p "$SIMULATOR_DIR"

    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        mkdir -p "$(node_dir "$node_index")"
    done

    write_manifest
    print_success "Directories ready"
}

pid_is_alive() {
    local pid="$1"
    kill -0 "$pid" 2>/dev/null
}

collect_devnet_pids() {
    (
        shopt -s nullglob

        local pid_file
        for pid_file in "$DATA_DIR"/node*/node.pid; do
            cat "$pid_file" 2>/dev/null || true
        done

        pgrep -f "btcd.*--datadir=${DATA_DIR}/node" 2>/dev/null || true
    ) | awk 'NF && !seen[$0]++'
}

collect_devnet_pids_array() {
    DEVNET_PIDS=()

    while IFS= read -r pid; do
        if [ -n "$pid" ]; then
            DEVNET_PIDS+=("$pid")
        fi
    done < <(collect_devnet_pids)
}

alive_pids_string() {
    local pid
    local alive=""
    for pid in "$@"; do
        if pid_is_alive "$pid"; then
            alive="${alive}${alive:+ }${pid}"
        fi
    done

    printf '%s\n' "$alive"
}

wait_for_pids_exit() {
    local max_attempts="${1:-10}"
    shift || true

    local attempt=0
    while [ "$attempt" -lt "$max_attempts" ]; do
        local any_alive=0
        local pid
        for pid in "$@"; do
            if pid_is_alive "$pid"; then
                any_alive=1
                break
            fi
        done

        if [ "$any_alive" -eq 0 ]; then
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 1
    done

    return 1
}

remove_data_dir() {
    if [ ! -d "$DATA_DIR" ]; then
        return 0
    fi

    local attempt=0
    local max_attempts=5
    while [ "$attempt" -lt "$max_attempts" ]; do
        if rm -rf "$DATA_DIR" 2>/dev/null; then
            break
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    if [ -d "$DATA_DIR" ]; then
        print_error "Failed to remove DevNet data dir: $DATA_DIR"
        return 1
    fi

    return 0
}

is_devnet_running() {
    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        local pid_file
        pid_file="$(node_pid_file "$node_index")"
        if [ ! -f "$pid_file" ]; then
            return 1
        fi

        local node_pid
        node_pid="$(cat "$pid_file")"
        if ! pid_is_alive "$node_pid"; then
            return 1
        fi
    done

    return 0
}

require_devnet_running() {
    if ! is_devnet_running; then
        print_error "DevNet is not running. Start it first with: $0 start"
        exit 1
    fi
}

start_node() {
    local node_index="$1"
    local role
    role="$(node_role "$node_index")"

    local node_dir_path
    node_dir_path="$(node_dir "$node_index")"

    local rpc_port
    rpc_port="$(node_rpc_port "$node_index")"

    local p2p_port
    p2p_port="$(node_p2p_port "$node_index")"

    local args=(
        "$BTCD_BINARY"
        --"$NETWORK"
        --notls
        --nobanning
        --datadir="$node_dir_path"
        --listen="127.0.0.1:$p2p_port"
        --rpclisten="127.0.0.1:$rpc_port"
        --rpcuser=obtc
        --rpcpass=obtcpass
        --logdir="$node_dir_path/logs"
        --debuglevel=info
    )

    local extra_flag
    extra_flag="$(network_node_flag)"
    if [ -n "$extra_flag" ]; then
        args+=("$extra_flag")
    fi

    if [ "$node_index" -eq "$PRIMARY_NODE_INDEX" ]; then
        local mining_addr
        mining_addr="$(get_mining_address)"
        print_status "Starting $(node_name "$node_index") (${role}) at ${mining_addr} ..."
        args+=(--miningaddr="$mining_addr")
    else
        print_status "Starting $(node_name "$node_index") (${role})..."
        args+=(--connect="127.0.0.1:$(node_p2p_port "$PRIMARY_NODE_INDEX")")
        sleep 1
    fi

    nohup "${args[@]}" >"$node_dir_path/console.log" 2>&1 < /dev/null &

    local node_pid=$!
    echo "$node_pid" > "$(node_pid_file "$node_index")"
    print_success "$(node_name "$node_index") started (PID: $node_pid)"
}

wait_for_nodes() {
    print_status "Waiting for ${DEVNET_NODE_COUNT} node(s) to accept RPC..."

    local max_attempts=30
    local attempt=0

    while [ "$attempt" -lt "$max_attempts" ]; do
        local ready=true
        local node_index
        for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
            if ! btcctl_node "$node_index" getinfo >/dev/null 2>&1; then
                ready=false
                break
            fi
        done

        if [ "$ready" = true ]; then
            print_success "All ${DEVNET_NODE_COUNT} node(s) are ready"
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 1
    done

    print_error "Nodes failed to start within ${max_attempts}s"
    stop_nodes || true
    exit 1
}

wait_for_height_sync() {
    local target_height="${1:-$(btcctl_node "$PRIMARY_NODE_INDEX" getblockcount)}"
    local max_attempts="${2:-30}"
    local attempt=0

    while [ "$attempt" -lt "$max_attempts" ]; do
        local synced=true
        local node_index
        for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
            local node_height
            node_height="$(btcctl_node "$node_index" getblockcount 2>/dev/null || echo -1)"
            if [ "$node_height" -lt "$target_height" ]; then
                synced=false
                break
            fi
        done

        if [ "$synced" = true ]; then
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 1
    done

    print_error "Nodes failed to sync to height ${target_height}"
    return 1
}

json_get() {
    local field="$1"
    python3 -c 'import json,sys; v=json.load(sys.stdin).get(sys.argv[1]);
if isinstance(v, bool): print(str(v).lower())
elif v is None: print("")
else: print(v)' "$field"
}

show_obtc_status_node() {
    local node_index="$1"

    if ! is_obtc_network; then
        return 0
    fi

    echo "  OBTC ExpiryIndex: $(btcctl_node "$node_index" getexpiryindexstats)"
    echo "  OBTC ExpiryCommitment: $(btcctl_node "$node_index" getexpirycommitment)"
    echo "  OBTC REAP Plan: $(btcctl_node "$node_index" getreapplan)"
}

latest_block_has_reap_marker() {
    local block_hash="${1:-$(btcctl_node "$PRIMARY_NODE_INDEX" getbestblockhash)}"
    btcctl_node "$PRIMARY_NODE_INDEX" getblock "$block_hash" 2 | python3 -c 'import json,sys
block=json.load(sys.stdin)
txs=block.get("rawtx") or block.get("tx") or []
for tx in txs:
    if tx.get("version") != 3:
        continue
    for vout in tx.get("vout", []):
        asm=vout.get("scriptPubKey", {}).get("asm", "")
        parts=asm.split()
        if not parts or parts[0] != "OP_RETURN" or len(parts) < 2:
            continue
        try:
            payload=bytes.fromhex(parts[1]).decode("utf-8", "ignore")
        except Exception:
            payload=""
        if payload.startswith("REAP:"):
            print("true")
            sys.exit(0)
print("false")'
}

current_reap_picked() {
    if ! is_obtc_network; then
        echo 0
        return 0
    fi

    local reap_plan
    reap_plan="$(btcctl_node "$PRIMARY_NODE_INDEX" getreapplan)"
    local reap_picked
    reap_picked="$(printf '%s' "$reap_plan" | json_get picked)"
    if [ -z "$reap_picked" ]; then
        echo 0
        return 0
    fi

    echo "$reap_picked"
}

validate_obtc_node() {
    local label="$1"
    local node_index="$2"

    if ! is_obtc_network; then
        return 0
    fi

    local expiry_stats commitment reap_plan
    expiry_stats="$(btcctl_node "$node_index" getexpiryindexstats)"
    commitment="$(btcctl_node "$node_index" getexpirycommitment)"
    reap_plan="$(btcctl_node "$node_index" getreapplan)"

    local expiry_disabled expiry_tip commitment_enabled commitment_active reap_enabled reap_active reap_picked
    expiry_disabled="$(printf '%s' "$expiry_stats" | json_get disabled)"
    expiry_tip="$(printf '%s' "$expiry_stats" | json_get tip_height)"
    commitment_enabled="$(printf '%s' "$commitment" | json_get enabled)"
    commitment_active="$(printf '%s' "$commitment" | json_get active)"
    reap_enabled="$(printf '%s' "$reap_plan" | json_get enabled)"
    reap_active="$(printf '%s' "$reap_plan" | json_get active)"
    reap_picked="$(printf '%s' "$reap_plan" | json_get picked)"

    if [ "$expiry_disabled" != "false" ]; then
        print_error "$label expiry index is disabled"
        return 1
    fi
    if [ "$commitment_enabled" != "true" ] || [ "$commitment_active" != "true" ]; then
        print_error "$label expiry commitment is not active"
        return 1
    fi
    if [ "$reap_enabled" != "true" ] || [ "$reap_active" != "true" ]; then
        print_error "$label REAP plan is not active"
        return 1
    fi
    if [ -z "$expiry_tip" ] || [ "$expiry_tip" -lt "$OBTC_BOOTSTRAP_HEIGHT" ]; then
        print_error "$label expiry tip height is below OBTC bootstrap height"
        return 1
    fi
    if [ -z "$reap_picked" ] || [ "$reap_picked" -lt 0 ]; then
        print_error "$label REAP plan picked value is invalid"
        return 1
    fi

    return 0
}

validate_recent_obtc_block() {
    local require_marker="${1:-false}"

    if ! is_obtc_network; then
        return 0
    fi

    if [ "$require_marker" != "true" ]; then
        return 0
    fi

    local best_hash
    best_hash="$(btcctl_node "$PRIMARY_NODE_INDEX" getbestblockhash)"
    if [ "$(latest_block_has_reap_marker "$best_hash")" != "true" ]; then
        print_error "latest block ${best_hash} does not contain a REAP marker tx"
        return 1
    fi

    return 0
}

validate_obtc_state() {
    local context="${1:-state}"

    if ! is_obtc_network; then
        return 0
    fi

    print_status "Validating OBTC-specific state (${context})..."
    wait_for_height_sync >/dev/null

    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        validate_obtc_node "$(node_name "$node_index")" "$node_index"
    done

    print_success "OBTC-specific validation passed (${context})"
}

bootstrap_obtc_chain() {
    if ! is_obtc_network; then
        return 0
    fi

    local current_height
    current_height="$(btcctl_node "$PRIMARY_NODE_INDEX" getblockcount)"
    if [ "$current_height" -lt "$OBTC_BOOTSTRAP_HEIGHT" ]; then
        local blocks_to_mine=$((OBTC_BOOTSTRAP_HEIGHT - current_height))
        print_status "Mining ${blocks_to_mine} bootstrap block(s) to activate expiry/REAP/replay logic..."
        btcctl_node "$PRIMARY_NODE_INDEX" generate "$blocks_to_mine" >/dev/null
        wait_for_height_sync "$OBTC_BOOTSTRAP_HEIGHT"
    fi

    validate_obtc_state "bootstrap"
}

stop_nodes() {
    print_status "Stopping DevNet..."

    collect_devnet_pids_array
    if [ "${#DEVNET_PIDS[@]}" -eq 0 ]; then
        find "$DATA_DIR" -name node.pid -delete 2>/dev/null || true
        print_success "DevNet stopped"
        return 0
    fi

    kill "${DEVNET_PIDS[@]}" 2>/dev/null || true
    if ! wait_for_pids_exit 10 "${DEVNET_PIDS[@]}"; then
        local stubborn_pids
        stubborn_pids="$(alive_pids_string "${DEVNET_PIDS[@]}")"
        if [ -n "$stubborn_pids" ]; then
            print_warning "Force killing stubborn DevNet processes: ${stubborn_pids}"
            kill -9 $stubborn_pids 2>/dev/null || true
            wait_for_pids_exit 5 $stubborn_pids || true
        fi
    fi

    find "$DATA_DIR" -name node.pid -delete 2>/dev/null || true
    collect_devnet_pids_array
    if [ "${#DEVNET_PIDS[@]}" -gt 0 ]; then
        print_error "Some DevNet processes are still running: $(alive_pids_string "${DEVNET_PIDS[@]}")"
        return 1
    fi

    print_success "DevNet stopped"
}

show_status() {
    require_devnet_running

    print_status "DevNet Status"
    echo ""
    echo "Network: $NETWORK"
    echo "Nodes: $DEVNET_NODE_COUNT"
    echo ""

    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        echo "$(node_name "$node_index") ($(node_role "$node_index"))"
        echo "  RPC: 127.0.0.1:$(node_rpc_port "$node_index")"
        echo "  P2P: 127.0.0.1:$(node_p2p_port "$node_index")"
        echo "  Height: $(btcctl_node "$node_index" getblockcount)"
        echo "  Connections: $(btcctl_node "$node_index" getconnectioncount)"
        echo "  Mempool Info: $(btcctl_node "$node_index" getmempoolinfo)"
        if [ "$node_index" -eq "$PRIMARY_NODE_INDEX" ]; then
            echo "  Continuous Mining: $(btcctl_node "$node_index" getgenerate)"
        fi
        show_obtc_status_node "$node_index"
        echo ""
    done

    echo "Primary Simulator"
    run_devnetsim status
    echo ""
    echo "Peer Simulator"
    run_devnetsim_peer status
    echo ""

    if is_obtc_network; then
        validate_obtc_state "status"
    fi
}

show_mempool() {
    require_devnet_running

    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        print_status "$(node_name "$node_index") mempool"
        btcctl_node "$node_index" getmempoolinfo
        echo ""
    done
}

validate_obtc_command() {
    require_devnet_running
    validate_obtc_state "manual"
}

replay_audit_command() {
    require_devnet_running
    ./scripts/validation/devnet_replay_audit.sh "$@"
}

mine_blocks() {
    require_devnet_running

    local blocks="${1:-1}"
    local pre_mine_reap_picked=0
    if is_obtc_network && [ "$blocks" -eq 1 ]; then
        pre_mine_reap_picked="$(current_reap_picked)"
    fi

    print_status "Mining ${blocks} block(s)..."
    btcctl_node "$PRIMARY_NODE_INDEX" generate "$blocks"
    wait_for_height_sync >/dev/null
    if is_obtc_network; then
        validate_obtc_state "mine ${blocks}"
        if [ "$blocks" -eq 1 ] && [ "${pre_mine_reap_picked:-0}" -gt 0 ]; then
            validate_recent_obtc_block true
        fi
    fi
    print_success "Generated ${blocks} block(s)"
}

set_miner() {
    require_devnet_running

    local action="${1:-off}"
    case "$action" in
        on)
            print_status "Enabling continuous CPU mining..."
            btcctl_node "$PRIMARY_NODE_INDEX" setgenerate true 1
            ;;
        off)
            print_status "Disabling continuous CPU mining..."
            btcctl_node "$PRIMARY_NODE_INDEX" setgenerate false
            ;;
        *)
            print_error "Unknown miner action: $action"
            exit 1
            ;;
    esac

    print_success "Continuous mining state: $(btcctl_node "$PRIMARY_NODE_INDEX" getgenerate)"
}

prepare_pool() {
    require_devnet_running

    local utxos="${1:-512}"
    local value="${2:-300000}"
    print_status "Preparing ${utxos} primary spendable UTXOs (value=${value} sat)..."
    run_devnetsim prepare --utxos "$utxos" --value "$value"
}

prepare_peer_pool() {
    require_devnet_running

    local utxos="${1:-256}"
    local value="${2:-300000}"
    print_status "Preparing ${utxos} peer spendable UTXOs (value=${value} sat)..."
    run_devnetsim prepare \
        --utxos "$utxos" \
        --value "$value" \
        --recipient-statefile "$PEER_SIMULATOR_STATE" \
        --recipient-seed-tag peer
    if is_obtc_network; then
        validate_obtc_state "prepare peer"
    fi
}

spam_pool() {
    require_devnet_running
    print_status "Injecting primary traffic into mempool..."
    run_devnetsim spam "$@"
}

spam_peer_pool() {
    require_devnet_running
    print_status "Injecting peer traffic into mempool..."
    run_devnetsim_peer spam "$@"
}

scenario_empty() {
    print_status "Scenario: empty block"
    mine_blocks 1
    show_status
}

scenario_sparse() {
    print_status "Scenario: sparse block with a few normal transactions"
    prepare_pool 32 300000
    spam_pool --count 5 --mode simple --value 120000
    mine_blocks 1
    show_status
}

scenario_dense() {
    print_status "Scenario: one block with many normal transactions"
    prepare_pool 4200 300000
    spam_pool --count 3200 --mode simple --value 120000
    mine_blocks 1
    show_status
}

scenario_backlog() {
    print_status "Scenario: overloaded mempool with backlog after mining"
    prepare_pool 6500 300000
    spam_pool --count 5000 --mode simple --value 120000
    mine_blocks 1
    show_status
}

scenario_feemarket() {
    print_status "Scenario: fee-banded mempool with backlog"
    prepare_pool 7000 300000
    spam_pool --count 4500 --mode feemarket --value 150000
    show_mempool
    mine_blocks 1
    show_status
}

scenario_conflict() {
    print_status "Scenario: conflicting double-spend attempts"
    prepare_pool 160 300000
    spam_pool --count 60 --mode conflict
    show_mempool
    mine_blocks 1
    show_status
}

scenario_consolidation() {
    print_status "Scenario: multi-input consolidation transactions"
    prepare_pool 256 300000
    spam_pool --count 40 --mode consolidate
    mine_blocks 1
    show_status
}

scenario_multisource() {
    print_status "Scenario: multi-wallet traffic from both nodes"
    prepare_pool 256 300000
    prepare_peer_pool 192 240000

    print_status "Phase 1: primary fee-market burst"
    spam_pool --count 60 --mode feemarket --value 150000

    print_status "Phase 2: peer mixed traffic"
    spam_peer_pool --count 48 --mode mixed --value 110000

    print_status "Phase 3: peer chains and primary consolidation"
    spam_peer_pool --count 20 --mode chain --value 90000
    spam_pool --count 8 --mode consolidate

    show_mempool
    mine_blocks 1
    show_status
}

scenario_steady() {
    print_status "Scenario: steady multi-block traffic"
    prepare_pool 1500 300000

    local round
    for round in 1 2 3 4 5; do
        print_status "Round ${round}: paced fee-market traffic then mine"
        spam_pool --count 180 --mode feemarket --value 150000 --pace-ms 5
        mine_blocks 1
    done

    show_status
}

scenario_dynamic() {
    print_status "Scenario: dynamic traffic mix"

    print_status "Phase 1: empty block"
    mine_blocks 1

    print_status "Phase 2: sparse block"
    prepare_pool 48 300000
    spam_pool --count 6 --mode simple --value 120000
    mine_blocks 1

    print_status "Phase 3: medium mixed traffic"
    prepare_pool 320 300000
    spam_pool --count 180 --mode mixed --value 150000
    mine_blocks 1

    print_status "Phase 4: fee market backlog"
    prepare_pool 2500 300000
    spam_pool --count 1600 --mode feemarket --value 150000
    mine_blocks 1

    print_status "Phase 5: conflicting spends"
    prepare_pool 96 300000
    spam_pool --count 24 --mode conflict
    mine_blocks 1

    print_status "Phase 6: dependent transaction chains left in mempool"
    prepare_pool 24 300000
    spam_pool --count 1200 --mode chain --value 120000

    show_status
}

show_logs() {
    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        local log_file
        log_file="$(node_dir "$node_index")/logs/btcd.log"
        if [ -f "$log_file" ]; then
            print_status "$(node_name "$node_index") recent logs"
            tail -20 "$log_file"
            echo ""
        fi
    done
}

start_devnet_fresh() {
    print_status "Starting OBTC DevNet (${DEVNET_NODE_COUNT}-node ${NETWORK})..."
    check_binaries
    build_tools
    setup_directories

    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        start_node "$node_index"
    done

    wait_for_nodes
    bootstrap_obtc_chain
    show_status
    print_success "DevNet is running"
    print_status "Use '$0 scenario dynamic' to generate mixed traffic"
}

resume_devnet() {
    if is_devnet_running; then
        print_warning "DevNet is already running"
        show_status
        exit 0
    fi

    if [ ! -d "$DATA_DIR" ] || [ ! -f "$PRIMARY_SIMULATOR_STATE" ]; then
        print_error "No existing DevNet data found. Start fresh with: $0 start"
        exit 1
    fi

    print_status "Resuming OBTC DevNet from existing data..."
    check_binaries
    build_tools
    mkdir -p "$SIMULATOR_DIR"

    local node_index
    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        mkdir -p "$(node_dir "$node_index")"
    done

    write_manifest

    for node_index in $(seq 1 "$DEVNET_NODE_COUNT"); do
        start_node "$node_index"
    done

    wait_for_nodes
    bootstrap_obtc_chain
    show_status
    print_success "DevNet resumed"
}

show_help() {
    echo "OBTC DevNet control script"
    echo ""
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Environment:"
    echo "  DEVNET_NODE_COUNT        Number of nodes to launch (default: 3, min: 2, max: 5)"
    echo "  DEVNET_NETWORK           Network name (default: obtcregtest)"
    echo ""
    echo "Commands:"
    echo "  start                     Start a fresh local DevNet (manual mining by default)"
    echo "  restart                   Resume DevNet from existing node + simulator data"
    echo "  stop                      Stop DevNet processes"
    echo "  status                    Show node, wallet and mempool status"
    echo "  mine [n]                  Mine n blocks on node1"
    echo "  miner <on|off>            Toggle continuous CPU mining on node1"
    echo "  mempool                   Show mempool info on every node"
    echo "  audit-replay [flags]      Replay-audit blocks on node1 via scripts/validation/devnet_replay_audit.sh"
    echo "  prepare [utxos] [value]   Pre-build primary spendable UTXOs"
    echo "  prepare-peer [u] [value]  Fund a second deterministic peer wallet"
    echo "  spam [devnetsim flags]    Inject transactions from the primary wallet"
    echo "  spam-peer [flags]         Inject transactions from the peer wallet via node2"
    echo "  scenario <name>           Run a canned scenario"
    echo "  demo                      Alias for 'scenario dynamic'"
    echo "  logs                      Show recent node logs"
    echo "  clean                     Stop DevNet and remove all data"
    echo "  help                      Show this help"
    echo ""
    echo "Scenarios:"
    echo "  empty         Mine an empty block"
    echo "  sparse        Mine a block with only a few transactions"
    echo "  dense         Mine a block with many transactions"
    echo "  backlog       Leave a large backlog in the mempool after mining"
    echo "  feemarket     Simulate fee-banded traffic under block pressure"
    echo "  conflict      Simulate conflicting double-spend attempts"
    echo "  consolidation Simulate heavier wallet consolidation transactions"
    echo "  multisource   Simulate two independent wallets sending via both nodes"
    echo "  steady        Feed paced traffic across multiple blocks"
    echo "  dynamic       Alternate empty/sparse/fee/conflict/chain traffic"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  DEVNET_NODE_COUNT=5 $0 start"
    echo "  $0 restart"
    echo "  $0 mine 10"
    echo "  $0 prepare 4000 300000"
    echo "  $0 prepare-peer 256 240000"
    echo "  $0 spam --count 500 --mode feemarket --value 150000 --pace-ms 10"
    echo "  $0 spam-peer --count 120 --mode mixed --value 110000"
    echo "  $0 scenario multisource"
    echo "  $0 scenario steady"
    echo "  $0 scenario dynamic"
    echo ""
}

main() {
    validate_node_count

    local command="${1:-help}"
    shift || true

    case "$command" in
        start)
            if is_devnet_running; then
                print_warning "DevNet is already running"
                show_status
                exit 0
            fi
            start_devnet_fresh
            ;;

        restart)
            resume_devnet
            ;;

        stop)
            stop_nodes
            ;;

        status)
            show_status
            ;;

        mine)
            mine_blocks "${1:-1}"
            ;;

        miner)
            set_miner "${1:-off}"
            ;;

        mempool)
            show_mempool
            ;;

        validate-obtc)
            validate_obtc_command
            ;;

        audit-replay)
            replay_audit_command "$@"
            ;;

        prepare)
            prepare_pool "${1:-512}" "${2:-300000}"
            ;;

        prepare-peer)
            prepare_peer_pool "${1:-256}" "${2:-300000}"
            ;;

        spam)
            spam_pool "$@"
            ;;

        spam-peer)
            spam_peer_pool "$@"
            ;;

        scenario)
            case "${1:-}" in
                empty)
                    scenario_empty
                    ;;
                sparse)
                    scenario_sparse
                    ;;
                dense)
                    scenario_dense
                    ;;
                backlog)
                    scenario_backlog
                    ;;
                feemarket)
                    scenario_feemarket
                    ;;
                conflict)
                    scenario_conflict
                    ;;
                consolidation)
                    scenario_consolidation
                    ;;
                multisource)
                    scenario_multisource
                    ;;
                steady)
                    scenario_steady
                    ;;
                dynamic)
                    scenario_dynamic
                    ;;
                *)
                    print_error "Unknown scenario: ${1:-<missing>}"
                    show_help
                    exit 1
                    ;;
            esac
            ;;

        demo)
            scenario_dynamic
            ;;

        logs)
            show_logs
            ;;

        clean)
            stop_nodes || true
            if remove_data_dir; then
                print_success "All DevNet data removed"
            else
                exit 1
            fi
            ;;

        help|--help|-h)
            show_help
            ;;

        *)
            print_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
