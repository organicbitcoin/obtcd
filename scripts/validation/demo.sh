#!/bin/bash

# OBTC Week2 Validation Demo Script
# This script demonstrates how to validate the ExpiryIndex implementation

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE} OBTC Week2 Validation Demo${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo -e "${YELLOW}This demo shows how to validate the OBTC UTXO Expiry Index implementation${NC}"
echo

# Check if we're in the right directory
if [[ ! -f "../../obtcd" && ! -f "../../main.go" ]]; then
    echo -e "${RED}❌ Error: Please run this script from the obtcd/scripts/validation directory${NC}"
    exit 1
fi

echo -e "${BLUE}📋 Demo Steps:${NC}"
echo "1. Build OBTCD with ExpiryIndex support"
echo "2. Start OBTCD in regtest mode with expiry index enabled"
echo "3. Generate some test blocks"
echo "4. Run validation tests"
echo "5. Show results"
echo

read -p "Press Enter to start the demo..."
echo

# Step 1: Build OBTCD
echo -e "${YELLOW}🔨 Step 1: Building OBTCD...${NC}"
cd ../../
if ! go build -o obtcd .; then
    echo "❌ Failed to build OBTCD"
    exit 1
fi
echo -e "${GREEN}✅ OBTCD built successfully${NC}"
echo

# Step 2: Check if OBTCD is already running
echo -e "${YELLOW}🔍 Step 2: Checking OBTCD status...${NC}"
if pgrep -f "obtcd.*regtest.*expiryindex" > /dev/null; then
    echo -e "${GREEN}✅ OBTCD is already running with expiry index${NC}"
    OBTCD_RUNNING=true
else
    echo -e "${BLUE}ℹ️  Starting OBTCD in regtest mode with expiry index...${NC}"
    echo "Command: ./obtcd --obtcregtest --expiryindex --rpcuser=test --rpcpass=test"
    echo
    echo "⚠️  Please start OBTCD manually in another terminal with:"
    echo "   cd $(pwd)"
    echo "   ./obtcd --obtcregtest --expiryindex --rpcuser=test --rpcpass=test"
    echo
    read -p "Press Enter when OBTCD is running..."
    OBTCD_RUNNING=false
fi
echo

# Step 3: Wait for RPC to be ready
echo -e "${YELLOW}⏳ Step 3: Waiting for RPC to be ready...${NC}"
for i in {1..30}; do
    if timeout 2 ./btcctl --obtcregtest --rpcuser=test --rpcpass=test getblockcount > /dev/null 2>&1; then
        echo -e "${GREEN}✅ RPC is ready${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [[ $i -eq 30 ]]; then
        echo
        echo "❌ RPC not ready after 30 seconds. Please check OBTCD status."
        exit 1
    fi
done
echo

# Step 4: Generate some blocks if needed
echo -e "${YELLOW}📦 Step 4: Ensuring we have blocks for testing...${NC}"
BLOCK_COUNT=$(./btcctl --obtcregtest --rpcuser=test --rpcpass=test getblockcount)
echo "Current block count: $BLOCK_COUNT"

if [[ $BLOCK_COUNT -lt 101 ]]; then
    echo "Generating blocks to have enough for testing..."
    BLOCKS_NEEDED=$((101 - BLOCK_COUNT))
    ./btcctl --obtcregtest --rpcuser=test --rpcpass=test generate $BLOCKS_NEEDED > /dev/null
    NEW_COUNT=$(./btcctl --obtcregtest --rpcuser=test --rpcpass=test getblockcount)
    echo -e "${GREEN}✅ Generated blocks. New count: $NEW_COUNT${NC}"
else
    echo -e "${GREEN}✅ Sufficient blocks available${NC}"
fi
echo

# Step 5: Run validation
echo -e "${YELLOW}🧪 Step 5: Running OBTC ExpiryIndex validation...${NC}"
cd scripts/validation/

echo "Building validation tool..."
if ! go build -o /tmp/obtc_validator utxo_expiry_validator.go; then
    echo "❌ Failed to build validation tool"
    exit 1
fi

echo
echo "Running validation tests..."
echo

if /tmp/obtc_validator -rpcuser=test -rpcpass=test -network=obtcregtest -verbose; then
    echo
    echo -e "${GREEN}🎉 Validation completed successfully!${NC}"
    echo
    echo -e "${BLUE}📊 What this means:${NC}"
    echo "✅ OBTCD is running with ExpiryIndex enabled"
    echo "✅ RPC commands 'listexpiring' and 'getexpiryindexstats' work"
    echo "✅ The expiry index is functioning correctly"
    echo "✅ Week2 implementation is working as expected"
else
    echo
    echo "❌ Validation failed!"
    echo
    echo -e "${YELLOW}💡 Troubleshooting tips:${NC}"
    echo "1. Ensure OBTCD is running with --expiryindex flag"
    echo "2. Check RPC credentials (should be test/test for this demo)"
    echo "3. Verify network is regtest"
    echo "4. Check OBTCD logs for any errors"
    exit 1
fi

# Cleanup
rm -f /tmp/obtc_validator

echo
echo -e "${BLUE}🎯 Demo completed!${NC}"
echo
echo -e "${YELLOW}Next steps:${NC}"
echo "• Try the validation on testnet: ./quick_validate.sh testnet --rpcuser=your_user --rpcpass=your_pass"
echo "• Run stress tests: /tmp/obtc_validator -rpcuser=test -rpcpass=test -stress -verbose"
echo "• Check the validation tools documentation in README.md"
echo
echo -e "${GREEN}Week2 ExpiryIndex implementation is validated and ready! ✨${NC}"
