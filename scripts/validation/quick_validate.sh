#!/bin/bash

# OBTC UTXO Expiry Index Quick Validation Script
# This script provides quick validation of the ExpiryIndex functionality
# across different networks with minimal setup.

set -e  # Exit on any error

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OBTCD_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
VALIDATION_TOOL="$SCRIPT_DIR/utxo_expiry_validator.go"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print functions
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE} OBTC UTXO Expiry Index Validation${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo
}

print_step() {
    echo -e "${YELLOW}🔄 $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# Help function
show_help() {
    cat << EOF
OBTC UTXO Expiry Index Quick Validation

USAGE:
    $0 [OPTIONS] NETWORK

NETWORKS:
    obtcregtest  - OBTC regression test network (default)
    obtctestnet  - OBTC testnet
    obtcmainnet  - OBTC mainnet (read-only validation)
    regtest      - Bitcoin regtest
    testnet      - Bitcoin testnet
    mainnet      - Bitcoin mainnet (read-only validation)

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose output
    -o, --output FILE   Save results to JSON file
    -s, --stress        Enable stress testing
    -b, --bench         Enable performance benchmarking
    --rpcuser USER      RPC username (required)
    --rpcpass PASS      RPC password (required)
    --rpchost HOST      RPC host:port (default: auto-detect)
    --start HEIGHT      Start height for testing
    --end HEIGHT        End height for testing
    --max RESULTS       Maximum results per query (default: 100)

EXAMPLES:
    # Quick OBTC regtest validation
    $0 obtcregtest --rpcuser=test --rpcpass=test

    # OBTC testnet validation with custom range
    $0 obtctestnet --rpcuser=user --rpcpass=pass --start=2800000 --end=2810000

    # Comprehensive OBTC mainnet validation
    $0 obtcmainnet --rpcuser=user --rpcpass=pass --stress --bench --verbose

    # Save results to file
    $0 obtcregtest --rpcuser=test --rpcpass=test -o validation_results.json

PREREQUISITES:
    1. Build OBTCD: go build -o obtcd .
    2. Start OBTCD with expiry index enabled:
       ./obtcd --obtcregtest --expiryindex --rpcuser=test --rpcpass=test
    3. Wait for initial sync and index build to complete

EOF
}

# Parse command line arguments
NETWORK=""
VERBOSE=false
OUTPUT_FILE=""
STRESS=false
BENCH=false
RPC_USER=""
RPC_PASS=""
RPC_HOST=""
START_HEIGHT=""
END_HEIGHT=""
MAX_RESULTS="100"

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -s|--stress)
            STRESS=true
            shift
            ;;
        -b|--bench)
            BENCH=true
            shift
            ;;
        --rpcuser)
            RPC_USER="$2"
            shift 2
            ;;
        --rpcuser=*)
            RPC_USER="${1#*=}"
            shift
            ;;
        --rpcpass)
            RPC_PASS="$2"
            shift 2
            ;;
        --rpcpass=*)
            RPC_PASS="${1#*=}"
            shift
            ;;
        --rpchost)
            RPC_HOST="$2"
            shift 2
            ;;
        --rpchost=*)
            RPC_HOST="${1#*=}"
            shift
            ;;
        --start)
            START_HEIGHT="$2"
            shift 2
            ;;
        --start=*)
            START_HEIGHT="${1#*=}"
            shift
            ;;
        --end)
            END_HEIGHT="$2"
            shift 2
            ;;
        --end=*)
            END_HEIGHT="${1#*=}"
            shift
            ;;
        --max)
            MAX_RESULTS="$2"
            shift 2
            ;;
        --max=*)
            MAX_RESULTS="${1#*=}"
            shift
            ;;
        obtcregtest|obtctestnet|obtcmainnet|regtest|testnet|mainnet)
            NETWORK="$1"
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Validate required parameters
if [[ -z "$NETWORK" ]]; then
    NETWORK="obtcregtest"
    print_info "No network specified, defaulting to obtcregtest"
fi

if [[ -z "$RPC_USER" || -z "$RPC_PASS" ]]; then
    print_error "RPC username and password are required"
    echo "Use --rpcuser and --rpcpass options"
    exit 1
fi

# Auto-detect RPC host if not specified
if [[ -z "$RPC_HOST" ]]; then
    case $NETWORK in
        obtcregtest)
            RPC_HOST="localhost:29528"
            ;;
        obtctestnet)
            RPC_HOST="localhost:19528"
            ;;
        obtcmainnet)
            RPC_HOST="localhost:9528"
            ;;
        regtest)
            RPC_HOST="localhost:18334"
            ;;
        testnet)
            RPC_HOST="localhost:18334"
            ;;
        mainnet)
            RPC_HOST="localhost:8334"
            ;;
    esac
fi

# Build validation tool arguments
TOOL_ARGS=(
    "-rpcuser=$RPC_USER"
    "-rpcpass=$RPC_PASS"
    "-rpchost=$RPC_HOST"
    "-network=$NETWORK"
    "-max=$MAX_RESULTS"
)

if [[ "$VERBOSE" == true ]]; then
    TOOL_ARGS+=("-verbose")
fi

if [[ -n "$OUTPUT_FILE" ]]; then
    TOOL_ARGS+=("-output=$OUTPUT_FILE")
fi

if [[ "$STRESS" == true ]]; then
    TOOL_ARGS+=("-stress")
fi

if [[ "$BENCH" == true ]]; then
    TOOL_ARGS+=("-bench")
fi

if [[ -n "$START_HEIGHT" ]]; then
    TOOL_ARGS+=("-start=$START_HEIGHT")
fi

if [[ -n "$END_HEIGHT" ]]; then
    TOOL_ARGS+=("-end=$END_HEIGHT")
fi

# Main execution
main() {
    print_header
    
    print_info "Network: $NETWORK"
    print_info "RPC Host: $RPC_HOST"
    print_info "Max Results: $MAX_RESULTS"
    if [[ -n "$OUTPUT_FILE" ]]; then
        print_info "Output File: $OUTPUT_FILE"
    fi
    echo

    # Check if obtcd is available
    print_step "Checking OBTCD availability..."
    cd "$OBTCD_DIR"
    
    if [[ ! -f "obtcd" ]]; then
        print_step "Building OBTCD..."
        if ! go build -o obtcd .; then
            print_error "Failed to build OBTCD"
            exit 1
        fi
        print_success "OBTCD built successfully"
    else
        print_success "OBTCD binary found"
    fi

    # Check if validation tool compiles
    print_step "Checking validation tool..."
    cd "$SCRIPT_DIR"
    
    if ! go build -o /tmp/obtc_validator "$VALIDATION_TOOL"; then
        print_error "Failed to build validation tool"
        exit 1
    fi
    print_success "Validation tool built successfully"

    # Run validation
    print_step "Starting validation tests..."
    echo
    
    if /tmp/obtc_validator "${TOOL_ARGS[@]}"; then
        echo
        print_success "Validation completed successfully!"
        
        if [[ -n "$OUTPUT_FILE" && -f "$OUTPUT_FILE" ]]; then
            print_info "Results saved to: $OUTPUT_FILE"
            
            # Show summary from JSON if jq is available
            if command -v jq &> /dev/null; then
                echo
                print_info "Quick Summary:"
                jq -r '.validation_summary | "  Tests: \(.total_tests), Passed: \(.passed), Failed: \(.failed), Success Rate: \((.passed/.total_tests*100)|floor)%"' "$OUTPUT_FILE"
            fi
        fi
    else
        print_error "Validation failed!"
        exit 1
    fi

    # Cleanup
    rm -f /tmp/obtc_validator
}

# Check for prerequisites
check_prerequisites() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi

    if [[ ! -d "$OBTCD_DIR" ]]; then
        print_error "OBTCD directory not found: $OBTCD_DIR"
        exit 1
    fi

    if [[ ! -f "$VALIDATION_TOOL" ]]; then
        print_error "Validation tool not found: $VALIDATION_TOOL"
        exit 1
    fi
}

# Run checks and main function
check_prerequisites
main

print_info "Validation script completed"
