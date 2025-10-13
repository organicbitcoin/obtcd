#!/bin/bash

# OBTC CI Validation Script
# Simulates GitHub Actions CI pipeline locally

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[CI-TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[FAILED]${NC} $1"
}

print_section() {
    echo ""
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW} $1${NC}"
    echo -e "${YELLOW}========================================${NC}"
}

# Test 1: Build
test_build() {
    print_section "Testing Build"
    print_status "Running go build..."
    
    if go build; then
        print_success "Build test passed"
    else
        print_error "Build test failed"
        return 1
    fi
}

# Test 2: Unit Tests
test_unit() {
    print_section "Testing Unit Tests"
    print_status "Running go test..."
    
    if go test ./...; then
        print_success "Unit tests passed"
    else
        print_error "Unit tests failed"
        return 1
    fi
}

# Test 3: Race Detection
test_race() {
    print_section "Testing Race Conditions"
    print_status "Running go test -race..."
    
    if go test -race -short ./...; then
        print_success "Race condition tests passed"
    else
        print_error "Race condition tests failed"
        return 1
    fi
}

# Test 4: OBTC Specific Tests
test_obtc() {
    print_section "Testing OBTC Integration"
    
    print_status "Running OBTC network tests..."
    if go test ./chaincfg ./wire -v -run "OBTC"; then
        print_success "OBTC network tests passed"
    else
        print_error "OBTC network tests failed"
        return 1
    fi
    
    print_status "Running OBTC fork height tests..."
    if go test ./chaincfg -v -run "Fork"; then
        print_success "OBTC fork height tests passed"
    else
        print_error "OBTC fork height tests failed"
        return 1
    fi
    
    print_status "Validating OBTC parameters..."
    cat > validate_obtc_temp.go << 'EOF'
package main
import (
  "fmt"
  "github.com/btcsuite/btcd/chaincfg"
)
func main() {
  if !chaincfg.IsOBTC(&chaincfg.ObtcMainNetParams) {
    panic("OBTC MainNet not properly registered")
  }
  if !chaincfg.IsOBTC(&chaincfg.ObtcTestNetParams) {
    panic("OBTC TestNet not properly registered")
  }
  if !chaincfg.IsOBTC(&chaincfg.ObtcRegTestParams) {
    panic("OBTC RegTest not properly registered")
  }
  
  mainnetFork := chaincfg.GetOBTCForkHeight(&chaincfg.ObtcMainNetParams)
  testnetFork := chaincfg.GetOBTCForkHeight(&chaincfg.ObtcTestNetParams)
  regtestFork := chaincfg.GetOBTCForkHeight(&chaincfg.ObtcRegTestParams)
  
  if mainnetFork <= 0 || testnetFork <= 0 || regtestFork <= 0 {
    panic("Invalid fork heights detected")
  }
  
  fmt.Println("✅ All OBTC parameters validated successfully")
  fmt.Printf("Fork heights - MainNet: %d, TestNet: %d, RegTest: %d\n", 
    mainnetFork, testnetFork, regtestFork)
}
EOF
    
    if go run validate_obtc_temp.go; then
        print_success "OBTC parameter validation passed"
    else
        print_error "OBTC parameter validation failed"
        return 1
    fi
    
    rm -f validate_obtc_temp.go
}

# Test 5: Scripts Validation
test_scripts() {
    print_section "Testing DevNet Scripts"
    
    if [ ! -f "scripts/devnet-up.sh" ] || [ ! -f "scripts/rebase-upstream.sh" ]; then
        print_error "Required scripts not found"
        return 1
    fi
    
    print_status "Checking script permissions..."
    chmod +x scripts/devnet-up.sh scripts/rebase-upstream.sh
    
    print_status "Testing devnet-up.sh help..."
    if ./scripts/devnet-up.sh help > /dev/null; then
        print_success "devnet-up.sh help works"
    else
        print_error "devnet-up.sh help failed"
        return 1
    fi
    
    print_status "Testing rebase-upstream.sh help..."
    if ./scripts/rebase-upstream.sh --help > /dev/null; then
        print_success "rebase-upstream.sh help works"
    else
        print_error "rebase-upstream.sh help failed"
        return 1
    fi
    
    print_success "All scripts validated successfully"
}

# Test 6: Code Quality
test_quality() {
    print_section "Testing Code Quality"
    
    print_status "Running go vet..."
    if go vet ./...; then
        print_success "go vet passed"
    else
        print_error "go vet failed"
        return 1
    fi
    
    print_status "Checking go fmt..."
    if [ "$(gofmt -s -l . | wc -l)" -gt 0 ]; then
        print_error "Code is not properly formatted. Run 'gofmt -s -w .'"
        gofmt -s -l .
        return 1
    else
        print_success "Code formatting is correct"
    fi
}

# Main function
main() {
    print_section "OBTC CI Validation Pipeline"
    echo "Simulating GitHub Actions CI locally..."
    echo ""
    
    local failed_tests=()
    
    # Run all tests
    test_build || failed_tests+=("Build")
    test_unit || failed_tests+=("Unit Tests")
    test_race || failed_tests+=("Race Detection")
    test_obtc || failed_tests+=("OBTC Integration")
    test_scripts || failed_tests+=("Scripts Validation")
    test_quality || failed_tests+=("Code Quality")
    
    # Summary
    print_section "CI Validation Results"
    
    if [ ${#failed_tests[@]} -eq 0 ]; then
        print_success "🎉 All CI tests passed! Your changes are ready for GitHub Actions."
        print_status "You can safely push to GitHub - the CI pipeline should succeed."
        echo ""
        print_status "To push your changes:"
        print_status "  git add ."
        print_status "  git commit -m 'your commit message'"
        print_status "  git push origin your-branch-name"
        return 0
    else
        print_error "❌ Some CI tests failed:"
        for test in "${failed_tests[@]}"; do
            echo "  - $test"
        done
        echo ""
        print_status "Please fix the failing tests before pushing to GitHub."
        return 1
    fi
}

# Handle command line arguments
case "${1:-}" in
    "--help"|"-h")
        echo "OBTC CI Validation Script"
        echo ""
        echo "Usage: $0 [options]"
        echo ""
        echo "This script simulates the GitHub Actions CI pipeline locally"
        echo "to ensure your changes will pass CI before pushing."
        echo ""
        echo "Options:"
        echo "  --help    Show this help message"
        echo ""
        echo "Tests performed:"
        echo "  - Build compilation"
        echo "  - Unit tests"
        echo "  - Race condition detection"
        echo "  - OBTC-specific integration tests"
        echo "  - Script validation"
        echo "  - Code quality (vet, fmt)"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        print_error "Unknown option: $1"
        echo "Run '$0 --help' for usage information."
        exit 1
        ;;
esac