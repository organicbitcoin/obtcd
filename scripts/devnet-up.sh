#!/bin/bash

# OBTC DevNet control script.
# Starts a 2-node simnet and provides traffic/mempool simulation helpers.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BTCD_BINARY="./btcd"
BTCCTL_BINARY="./cmd/btcctl/btcctl"
DEVNETSIM_BINARY="./cmd/devnetsim/devnetsim"

DATA_DIR="$(pwd)/devnet-data"
NODE1_DIR="$DATA_DIR/node1"
NODE2_DIR="$DATA_DIR/node2"
SIMULATOR_DIR="$DATA_DIR/devnetsim"
PRIMARY_SIMULATOR_STATE="$SIMULATOR_DIR/state.json"
PEER_SIMULATOR_STATE="$SIMULATOR_DIR/peer-state.json"
NETWORK="${DEVNET_NETWORK:-obtcregtest}"
OBTC_BOOTSTRAP_HEIGHT=145

NODE1_RPC_PORT=18556
NODE1_P2P_PORT=18555
NODE2_RPC_PORT=18557
NODE2_P2P_PORT=18558

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

cleanup() {
    print_warning "Cleaning up DevNet data..."
    stop_nodes || true
    rm -rf "$DATA_DIR"
}

trap cleanup SIGINT SIGTERM

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

network_node_args() {
    if is_obtc_network; then
        echo "--expiryindex"
    fi
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

setup_directories() {
    print_status "Preparing DevNet directories..."
    rm -rf "$DATA_DIR"
    mkdir -p "$NODE1_DIR" "$NODE2_DIR" "$SIMULATOR_DIR"
    print_success "Directories ready"
}

btcctl_node1() {
    "$BTCCTL_BINARY" --notls --"$NETWORK" \
        --rpcuser=obtc --rpcpass=obtcpass \
        --rpcserver=127.0.0.1:$NODE1_RPC_PORT "$@"
}

btcctl_node2() {
    "$BTCCTL_BINARY" --notls --"$NETWORK" \
        --rpcuser=obtc --rpcpass=obtcpass \
        --rpcserver=127.0.0.1:$NODE2_RPC_PORT "$@"
}

run_devnetsim() {
    local command="$1"
    shift || true

    "$DEVNETSIM_BINARY" "$command" \
        --network="$NETWORK" \
        --statefile="$PRIMARY_SIMULATOR_STATE" \
        --seed-tag=primary \
        --rpcserver=127.0.0.1:$NODE1_RPC_PORT \
        --rpcuser=obtc \
        --rpcpass=obtcpass \
        --mirror-rpcserver=127.0.0.1:$NODE2_RPC_PORT \
        --mirror-rpcuser=obtc \
        --mirror-rpcpass=obtcpass \
        "$@"
}

run_devnetsim_peer() {
    local command="$1"
    shift || true

    "$DEVNETSIM_BINARY" "$command" \
        --network="$NETWORK" \
        --statefile="$PEER_SIMULATOR_STATE" \
        --seed-tag=peer \
        --rpcserver=127.0.0.1:$NODE2_RPC_PORT \
        --rpcuser=obtc \
        --rpcpass=obtcpass \
        --mirror-rpcserver=127.0.0.1:$NODE1_RPC_PORT \
        --mirror-rpcuser=obtc \
        --mirror-rpcpass=obtcpass \
        "$@"
}

get_mining_address() {
    "$DEVNETSIM_BINARY" miningaddr \
        --network="$NETWORK" \
        --statefile="$PRIMARY_SIMULATOR_STATE" \
        --seed-tag=primary
}

pid_is_alive() {
    local pid="$1"
    kill -0 "$pid" 2>/dev/null
}

is_devnet_running() {
    if [ ! -f "$NODE1_DIR/node.pid" ] || [ ! -f "$NODE2_DIR/node.pid" ]; then
        return 1
    fi

    local node1_pid node2_pid
    node1_pid=$(cat "$NODE1_DIR/node.pid")
    node2_pid=$(cat "$NODE2_DIR/node.pid")

    pid_is_alive "$node1_pid" && pid_is_alive "$node2_pid"
}

require_devnet_running() {
    if ! is_devnet_running; then
        print_error "DevNet is not running. Start it first with: $0 start"
        exit 1
    fi
}

start_node1() {
    local mining_addr
    mining_addr=$(get_mining_address)

    print_status "Starting Node 1 (manual miner) at $mining_addr ..."
    "$BTCD_BINARY" \
        --"$NETWORK" \
        $(network_node_args) \
        --notls \
        --nobanning \
        --datadir="$NODE1_DIR" \
        --listen=127.0.0.1:$NODE1_P2P_PORT \
        --rpclisten=127.0.0.1:$NODE1_RPC_PORT \
        --rpcuser=obtc \
        --rpcpass=obtcpass \
        --miningaddr="$mining_addr" \
        --logdir="$NODE1_DIR/logs" \
        --debuglevel=info \
        >"$NODE1_DIR/console.log" 2>&1 < /dev/null &

    NODE1_PID=$!
    echo "$NODE1_PID" > "$NODE1_DIR/node.pid"
    print_success "Node 1 started (PID: $NODE1_PID)"
}

start_node2() {
    print_status "Starting Node 2 (peer)..."
    sleep 2

    "$BTCD_BINARY" \
        --"$NETWORK" \
        $(network_node_args) \
        --notls \
        --nobanning \
        --datadir="$NODE2_DIR" \
        --listen=127.0.0.1:$NODE2_P2P_PORT \
        --rpclisten=127.0.0.1:$NODE2_RPC_PORT \
        --rpcuser=obtc \
        --rpcpass=obtcpass \
        --connect=127.0.0.1:$NODE1_P2P_PORT \
        --logdir="$NODE2_DIR/logs" \
        --debuglevel=info \
        >"$NODE2_DIR/console.log" 2>&1 < /dev/null &

    NODE2_PID=$!
    echo "$NODE2_PID" > "$NODE2_DIR/node.pid"
    print_success "Node 2 started (PID: $NODE2_PID)"
}

wait_for_nodes() {
    print_status "Waiting for both nodes to accept RPC..."

    local max_attempts=30
    local attempt=0

    while [ "$attempt" -lt "$max_attempts" ]; do
        if btcctl_node1 getinfo >/dev/null 2>&1 && btcctl_node2 getinfo >/dev/null 2>&1; then
            print_success "Both nodes are ready"
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
    local target_height="${1:-$(btcctl_node1 getblockcount)}"
    local max_attempts="${2:-30}"
    local attempt=0

    while [ "$attempt" -lt "$max_attempts" ]; do
        local node1_height node2_height
        node1_height="$(btcctl_node1 getblockcount 2>/dev/null || echo -1)"
        node2_height="$(btcctl_node2 getblockcount 2>/dev/null || echo -1)"

        if [ "$node1_height" -ge "$target_height" ] && [ "$node2_height" -ge "$target_height" ]; then
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
    local title="$1"
    local rpc="$2"

    if ! is_obtc_network; then
        return 0
    fi

    echo "  OBTC ExpiryIndex: $($rpc getexpiryindexstats)"
    echo "  OBTC ExpiryCommitment: $($rpc getexpirycommitment)"
    echo "  OBTC REAP Plan: $($rpc getreapplan)"
}

latest_block_has_reap_marker() {
    local block_hash="${1:-$(btcctl_node1 getbestblockhash)}"
    btcctl_node1 getblock "$block_hash" 2 | python3 -c 'import json,sys
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

validate_obtc_node() {
    local label="$1"
    local rpc="$2"

    if ! is_obtc_network; then
        return 0
    fi

    local expiry_stats commitment reap_plan
    expiry_stats="$($rpc getexpiryindexstats)"
    commitment="$($rpc getexpirycommitment)"
    reap_plan="$($rpc getreapplan)"

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
    if [ -z "$reap_picked" ] || [ "$reap_picked" -le 0 ]; then
        print_error "$label REAP plan picked no inputs"
        return 1
    fi

    return 0
}

validate_recent_obtc_block() {
    if ! is_obtc_network; then
        return 0
    fi

    local best_hash
    best_hash="$(btcctl_node1 getbestblockhash)"
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
    validate_obtc_node "node1" btcctl_node1
    validate_obtc_node "node2" btcctl_node2
    validate_recent_obtc_block
    print_success "OBTC-specific validation passed (${context})"
}

bootstrap_obtc_chain() {
    if ! is_obtc_network; then
        return 0
    fi

    local current_height
    current_height="$(btcctl_node1 getblockcount)"
    if [ "$current_height" -lt "$OBTC_BOOTSTRAP_HEIGHT" ]; then
        local blocks_to_mine=$((OBTC_BOOTSTRAP_HEIGHT - current_height))
        print_status "Mining ${blocks_to_mine} bootstrap block(s) to activate expiry/REAP/replay logic..."
        btcctl_node1 generate "$blocks_to_mine" >/dev/null
        wait_for_height_sync "$OBTC_BOOTSTRAP_HEIGHT"
    fi

    validate_obtc_state "bootstrap"
}

stop_nodes() {
    print_status "Stopping DevNet..."

    if [ -f "$NODE1_DIR/node.pid" ]; then
        local pid
        pid=$(cat "$NODE1_DIR/node.pid")
        kill "$pid" 2>/dev/null || true
        rm -f "$NODE1_DIR/node.pid"
    fi

    if [ -f "$NODE2_DIR/node.pid" ]; then
        local pid
        pid=$(cat "$NODE2_DIR/node.pid")
        kill "$pid" 2>/dev/null || true
        rm -f "$NODE2_DIR/node.pid"
    fi

    pkill -f "btcd.*${NETWORK}" 2>/dev/null || true
    print_success "DevNet stopped"
}

show_status() {
    require_devnet_running

    print_status "DevNet Status"
    echo ""
    echo "Network: $NETWORK"
    echo ""
    echo "Node 1 (Miner)"
    echo "  RPC: 127.0.0.1:$NODE1_RPC_PORT"
    echo "  P2P: 127.0.0.1:$NODE1_P2P_PORT"
    echo "  Height: $(btcctl_node1 getblockcount)"
    echo "  Connections: $(btcctl_node1 getconnectioncount)"
    echo "  Continuous Mining: $(btcctl_node1 getgenerate)"
    echo "  Mempool Info: $(btcctl_node1 getmempoolinfo)"
    show_obtc_status_node "node1" btcctl_node1
    echo ""
    echo "Node 2 (Peer)"
    echo "  RPC: 127.0.0.1:$NODE2_RPC_PORT"
    echo "  P2P: 127.0.0.1:$NODE2_P2P_PORT"
    echo "  Height: $(btcctl_node2 getblockcount)"
    echo "  Connections: $(btcctl_node2 getconnectioncount)"
    echo "  Mempool Info: $(btcctl_node2 getmempoolinfo)"
    show_obtc_status_node "node2" btcctl_node2
    echo ""
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

    print_status "Node 1 mempool"
    btcctl_node1 getmempoolinfo
    echo ""

    print_status "Node 2 mempool"
    btcctl_node2 getmempoolinfo
    echo ""
}

validate_obtc_command() {
    require_devnet_running
    validate_obtc_state "manual"
}

mine_blocks() {
    require_devnet_running

    local blocks="${1:-1}"
    print_status "Mining ${blocks} block(s)..."
    btcctl_node1 generate "$blocks"
    wait_for_height_sync >/dev/null
    if is_obtc_network; then
        validate_obtc_state "mine ${blocks}"
    fi
    print_success "Generated ${blocks} block(s)"
}

set_miner() {
    require_devnet_running

    local action="${1:-off}"
    case "$action" in
        on)
            print_status "Enabling continuous CPU mining..."
            btcctl_node1 setgenerate true 1
            ;;
        off)
            print_status "Disabling continuous CPU mining..."
            btcctl_node1 setgenerate false
            ;;
        *)
            print_error "Unknown miner action: $action"
            exit 1
            ;;
    esac

    print_success "Continuous mining state: $(btcctl_node1 getgenerate)"
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
    if [ -f "$NODE1_DIR/logs/btcd.log" ]; then
        print_status "Node 1 recent logs"
        tail -20 "$NODE1_DIR/logs/btcd.log"
        echo ""
    fi

    if [ -f "$NODE2_DIR/logs/btcd.log" ]; then
        print_status "Node 2 recent logs"
        tail -20 "$NODE2_DIR/logs/btcd.log"
    fi
}

start_devnet_fresh() {
    print_status "Starting OBTC DevNet (2-node ${NETWORK})..."
    check_binaries
    build_tools
    setup_directories
    start_node1
    start_node2
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
    mkdir -p "$NODE1_DIR" "$NODE2_DIR" "$SIMULATOR_DIR"
    start_node1
    start_node2
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
    echo "Commands:"
    echo "  start                     Start a fresh 2-node DevNet (manual mining by default)"
    echo "  restart                   Resume DevNet from existing node + simulator data"
    echo "  stop                      Stop DevNet processes"
    echo "  status                    Show node, wallet and mempool status"
    echo "  mine [n]                  Mine n blocks on Node 1"
    echo "  miner <on|off>            Toggle continuous CPU mining on Node 1"
    echo "  mempool                   Show mempool info on both nodes"
    echo "  prepare [utxos] [value]   Pre-build primary spendable UTXOs"
    echo "  prepare-peer [u] [value]  Fund a second deterministic peer wallet"
    echo "  spam [devnetsim flags]    Inject transactions from the primary wallet"
    echo "  spam-peer [flags]         Inject transactions from the peer wallet via Node 2"
    echo "  scenario <name>           Run a canned scenario"
    echo "  demo                      Alias for 'scenario dynamic'"
    echo "  logs                      Show recent node logs"
    echo "  clean                     Stop DevNet and remove all data"
    echo "  help                      Show this help"
    echo ""
    echo "Scenarios:"
    echo "  empty     Mine an empty block"
    echo "  sparse         Mine a block with only a few transactions"
    echo "  dense          Mine a block with many transactions"
    echo "  backlog        Leave a large backlog in the mempool after mining"
    echo "  feemarket      Simulate fee-banded traffic under block pressure"
    echo "  conflict       Simulate conflicting double-spend attempts"
    echo "  consolidation  Simulate heavier wallet consolidation transactions"
    echo "  multisource    Simulate two independent wallets sending via both nodes"
    echo "  steady         Feed paced traffic across multiple blocks"
    echo "  dynamic        Alternate empty/sparse/fee/conflict/chain traffic"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 restart"
    echo "  $0 mine 101"
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
            rm -rf "$DATA_DIR"
            print_success "All DevNet data removed"
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
