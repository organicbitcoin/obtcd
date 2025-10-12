#!/bin/bash

# OBTC DevNet Startup Script (Week 1)
# Starts 2-node simnet environment for development and testing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BTCD_BINARY="./btcd"
BTCCTL_BINARY="./cmd/btcctl/btcctl"
DATA_DIR="$(pwd)/devnet-data"
NODE1_DIR="$DATA_DIR/node1"
NODE2_DIR="$DATA_DIR/node2"
NETWORK="simnet"

# Ports
NODE1_RPC_PORT=18556
NODE1_P2P_PORT=18555
NODE2_RPC_PORT=18557
NODE2_P2P_PORT=18558

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    pkill -f "btcd.*simnet" || true
    rm -rf "$DATA_DIR"
    exit 0
}

# Handle Ctrl+C
trap cleanup SIGINT SIGTERM

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if binaries exist
check_binaries() {
    print_status "Checking binaries..."
    
    if [ ! -f "$BTCD_BINARY" ]; then
        print_error "btcd binary not found. Please run: go build"
        exit 1
    fi
    
    if [ ! -d "cmd/btcctl" ]; then
        print_error "btcctl directory not found"
        exit 1
    fi
    
    print_success "Binaries found"
}

# Build btcctl if needed
build_btcctl() {
    print_status "Building btcctl..."
    (cd cmd/btcctl && go build -o btcctl)
    print_success "btcctl built successfully"
}

# Setup data directories
setup_directories() {
    print_status "Setting up data directories..."
    
    rm -rf "$DATA_DIR"
    mkdir -p "$NODE1_DIR"
    mkdir -p "$NODE2_DIR"
    
    print_success "Data directories created"
}

# Start node 1 (mining node)
start_node1() {
    print_status "Starting Node 1 (miner)..."
    
    $BTCD_BINARY \
        --$NETWORK \
        --datadir="$NODE1_DIR" \
        --listen=127.0.0.1:$NODE1_P2P_PORT \
        --rpclisten=127.0.0.1:$NODE1_RPC_PORT \
        --rpcuser=obtc \
        --rpcpass=obtcpass \
        --miningaddr=SSge5cKdH2K3QKEumDJAX3EMvAgvZcmHZN \
        --generate \
        --logdir="$NODE1_DIR/logs" \
        --debuglevel=info &
    
    NODE1_PID=$!
    echo $NODE1_PID > "$NODE1_DIR/node.pid"
    
    print_success "Node 1 started (PID: $NODE1_PID)"
}

# Start node 2 (peer node)
start_node2() {
    print_status "Starting Node 2 (peer)..."
    
    # Wait a bit for node1 to fully start
    sleep 3
    
    $BTCD_BINARY \
        --$NETWORK \
        --datadir="$NODE2_DIR" \
        --listen=127.0.0.1:$NODE2_P2P_PORT \
        --rpclisten=127.0.0.1:$NODE2_RPC_PORT \
        --rpcuser=obtc \
        --rpcpass=obtcpass \
        --connect=127.0.0.1:$NODE1_P2P_PORT \
        --logdir="$NODE2_DIR/logs" \
        --debuglevel=info &
    
    NODE2_PID=$!
    echo $NODE2_PID > "$NODE2_DIR/node.pid"
    
    print_success "Node 2 started (PID: $NODE2_PID)"
}

# Wait for nodes to be ready
wait_for_nodes() {
    print_status "Waiting for nodes to be ready..."
    
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if btcctl_node1 getinfo >/dev/null 2>&1 && btcctl_node2 getinfo >/dev/null 2>&1; then
            print_success "Both nodes are ready!"
            return 0
        fi
        
        attempt=$((attempt + 1))
        echo -n "."
        sleep 1
    done
    
    print_error "Nodes failed to start within $max_attempts seconds"
    cleanup
    exit 1
}

# Helper functions for btcctl
btcctl_node1() {
    $BTCCTL_BINARY --$NETWORK --rpcuser=obtc --rpcpass=obtcpass --rpcserver=127.0.0.1:$NODE1_RPC_PORT "$@"
}

btcctl_node2() {
    $BTCCTL_BINARY --$NETWORK --rpcuser=obtc --rpcpass=obtcpass --rpcserver=127.0.0.1:$NODE2_RPC_PORT "$@"
}

# Show network status
show_status() {
    print_status "DevNet Status:"
    echo ""
    
    echo "Node 1 (Miner):"
    echo "  RPC: 127.0.0.1:$NODE1_RPC_PORT"
    echo "  P2P: 127.0.0.1:$NODE1_P2P_PORT"
    echo "  Info: $(btcctl_node1 getinfo | grep -E '(version|blocks|connections)' | tr '\n' ' ')"
    echo ""
    
    echo "Node 2 (Peer):"
    echo "  RPC: 127.0.0.1:$NODE2_RPC_PORT"
    echo "  P2P: 127.0.0.1:$NODE2_P2P_PORT"
    echo "  Info: $(btcctl_node2 getinfo | grep -E '(version|blocks|connections)' | tr '\n' ' ')"
    echo ""
}

# Demo transaction
demo_transaction() {
    print_status "Waiting for initial blocks..."
    sleep 10
    
    print_status "Creating demo transaction..."
    
    # Generate some blocks first
    btcctl_node1 generate 101 >/dev/null
    sleep 2
    
    # Get a new address from node 2
    local node2_address=$(btcctl_node2 getnewaddress)
    print_status "Node 2 address: $node2_address"
    
    # Send 1 BTC from node 1 to node 2
    local txid=$(btcctl_node1 sendtoaddress "$node2_address" 1.0)
    print_success "Transaction sent! TXID: $txid"
    
    # Generate a block to confirm
    btcctl_node1 generate 1 >/dev/null
    sleep 2
    
    # Check transaction confirmation
    local confirmations=$(btcctl_node1 gettransaction "$txid" | grep -o '"confirmations":[0-9]*' | cut -d: -f2)
    print_success "Transaction confirmed with $confirmations confirmations"
    
    # Save transaction info
    echo "Demo Transaction Details:" > "$DATA_DIR/demo-transaction.txt"
    echo "TXID: $txid" >> "$DATA_DIR/demo-transaction.txt"
    echo "Amount: 1.0 BTC" >> "$DATA_DIR/demo-transaction.txt"
    echo "From: Node 1 (miner)" >> "$DATA_DIR/demo-transaction.txt"
    echo "To: Node 2 ($node2_address)" >> "$DATA_DIR/demo-transaction.txt"
    echo "Confirmations: $confirmations" >> "$DATA_DIR/demo-transaction.txt"
    echo "Timestamp: $(date)" >> "$DATA_DIR/demo-transaction.txt"
    
    print_success "Transaction details saved to $DATA_DIR/demo-transaction.txt"
}

# Show usage help
show_help() {
    echo "OBTC DevNet Control Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  start    Start 2-node DevNet"
    echo "  stop     Stop DevNet"
    echo "  status   Show network status"
    echo "  demo     Run demo transaction"
    echo "  logs     Show node logs"
    echo "  clean    Clean all data"
    echo "  help     Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 start     # Start the network"
    echo "  $0 demo      # Run a demo transaction"
    echo "  $0 status    # Check status"
    echo ""
}

# Show logs
show_logs() {
    if [ -f "$NODE1_DIR/logs/btcd.log" ]; then
        print_status "Node 1 recent logs:"
        tail -20 "$NODE1_DIR/logs/btcd.log"
        echo ""
    fi
    
    if [ -f "$NODE2_DIR/logs/btcd.log" ]; then
        print_status "Node 2 recent logs:"
        tail -20 "$NODE2_DIR/logs/btcd.log"
    fi
}

# Stop nodes
stop_nodes() {
    print_status "Stopping DevNet..."
    
    if [ -f "$NODE1_DIR/node.pid" ]; then
        local pid=$(cat "$NODE1_DIR/node.pid")
        kill $pid 2>/dev/null || true
        rm -f "$NODE1_DIR/node.pid"
    fi
    
    if [ -f "$NODE2_DIR/node.pid" ]; then
        local pid=$(cat "$NODE2_DIR/node.pid")
        kill $pid 2>/dev/null || true
        rm -f "$NODE2_DIR/node.pid"
    fi
    
    pkill -f "btcd.*simnet" || true
    print_success "DevNet stopped"
}

# Main function
main() {
    local command="${1:-start}"
    
    case "$command" in
        "start")
            print_status "Starting OBTC DevNet (2-node simnet)..."
            check_binaries
            build_btcctl
            setup_directories
            start_node1
            start_node2
            wait_for_nodes
            show_status
            
            print_success "DevNet is running!"
            print_status "Run '$0 demo' to test a transaction"
            print_status "Run '$0 stop' to stop the network"
            print_status "Press Ctrl+C to stop and cleanup"
            
            # Keep script running
            while true; do
                sleep 30
                if ! kill -0 $NODE1_PID 2>/dev/null || ! kill -0 $NODE2_PID 2>/dev/null; then
                    print_error "One or more nodes stopped unexpectedly"
                    cleanup
                fi
            done
            ;;
        "stop")
            stop_nodes
            ;;
        "status")
            if [ -f "$NODE1_DIR/node.pid" ] && [ -f "$NODE2_DIR/node.pid" ]; then
                show_status
            else
                print_warning "DevNet is not running"
            fi
            ;;
        "demo")
            if [ -f "$NODE1_DIR/node.pid" ] && [ -f "$NODE2_DIR/node.pid" ]; then
                demo_transaction
            else
                print_error "DevNet is not running. Start it first with: $0 start"
            fi
            ;;
        "logs")
            show_logs
            ;;
        "clean")
            stop_nodes
            rm -rf "$DATA_DIR"
            print_success "All data cleaned"
            ;;
        "help"|"--help"|"-h")
            show_help
            ;;
        *)
            print_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"