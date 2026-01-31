# OBTC UTXO Expiry Index Validation Tools

This directory contains comprehensive validation tools for testing the OBTC UTXO Expiry Index implementation across different Bitcoin networks (mainnet, testnet, regtest).

## 🎯 Purpose

These tools validate that the Week2 ExpiryIndex implementation works correctly by:

1. **Testing RPC Connectivity** - Ensuring basic RPC communication works
2. **Validating Index Availability** - Confirming the expiry index is enabled and functional
3. **Testing Core Functionality** - Verifying `listexpiring` and `getexpiryindexstats` commands
4. **Parameter Validation** - Testing edge cases and invalid parameters
5. **Performance Testing** - Benchmarking query performance across different scenarios
6. **Stress Testing** - Validating stability under load

## 📁 Files

- **`utxo_expiry_validator.go`** - Comprehensive validation tool with full test suite
- **`quick_validate.sh`** - Bash script for easy validation across networks
- **`config_examples.conf`** - Sample configurations for different networks
- **`README.md`** - This documentation

## 🚀 Quick Start

### 1. Prepare OBTCD

First, build and start OBTCD with the expiry index enabled:

```bash
# Build OBTCD
cd /path/to/obtcd
go build -o obtcd .

# Start with expiry index enabled (choose one):

# For OBTC regtest (local testing)
./obtcd --obtcregtest --expiryindex --rpcuser=test --rpcpass=test

# For OBTC testnet
./obtcd --obtctestnet --expiryindex --rpcuser=your_user --rpcpass=your_pass

# For OBTC mainnet (read-only validation)
./obtcd --obtcmainnet --expiryindex --rpcuser=your_user --rpcpass=your_pass
```

### 2. Wait for Sync and Index Build

Wait for OBTCD to sync and build the expiry index. You can monitor progress with:

```bash
# Check sync status
btcctl --obtcregtest --rpcuser=test --rpcpass=test getblockcount

# Check index status
btcctl --obtcregtest --rpcuser=test --rpcpass=test getexpiryindexstats
```

### 3. Run Validation

Use the quick validation script for easy testing:

```bash
# Basic OBTC regtest validation
./scripts/validation/quick_validate.sh obtcregtest --rpcuser=test --rpcpass=test

# OBTC testnet validation with stress testing
./scripts/validation/quick_validate.sh obtctestnet --rpcuser=user --rpcpass=pass --stress --verbose

# OBTC mainnet validation with performance benchmarking
./scripts/validation/quick_validate.sh obtcmainnet --rpcuser=user --rpcpass=pass --bench -o mainnet_results.json
```

## 🔧 Advanced Usage

### Direct Tool Usage

For more control, use the Go tool directly:

```bash
cd scripts/validation

# Basic validation
go run utxo_expiry_validator.go \
  -rpcuser=test -rpcpass=test \
  -network=obtcregtest -verbose

# Custom height range
go run utxo_expiry_validator.go \
  -rpcuser=test -rpcpass=test \
  -network=obtctestnet \
  -start=2800000 -end=2810000 \
  -max=1000 -output=results.json

# Comprehensive testing
go run utxo_expiry_validator.go \
  -rpcuser=test -rpcpass=test \
  -network=obtcregtest \
  -stress -stress-iterations=100 \
  -bench -verbose \
  -output=comprehensive_results.json
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-rpcuser` | RPC username | *Required* |
| `-rpcpass` | RPC password | *Required* |
| `-rpchost` | RPC host:port | Auto-detected |
| `-rpccert` | TLS certificate path | Empty (no TLS) |
| `-network` | Network (obtcregtest/obtctestnet/obtcmainnet or regtest/testnet/mainnet) | obtcregtest |
| `-start` | Start height | Current tip - 1000 |
| `-end` | End height | Current tip |
| `-max` | Max results per query | 100 |
| `-verbose` | Enable verbose output | false |
| `-output` | JSON output file | None |
| `-stress` | Enable stress testing | false |
| `-stress-iterations` | Stress test iterations | 10 |
| `-bench` | Enable performance benchmarking | false |

## 📊 Expected Results

### Successful Validation Output

```
🚀 Starting OBTC UTXO Expiry Index Validation
Network: obtcregtest
Height Range: 100 - 1000
Max Results: 100

🧪 Running test: connectivity... ✅ PASSED (15ms)
🧪 Running test: index_availability... ✅ PASSED (8ms)
   Index Stats: UTXOs=1234, Keys=56, TipHeight=1000
🧪 Running test: basic_query... ✅ PASSED (45ms)
   Found 78 expiring UTXOs between heights 100-1000
🧪 Running test: parameter_validation... ✅ PASSED (120ms)
🧪 Running test: pagination... ✅ PASSED (89ms)
   Paginated through 234 results
🧪 Running test: edge_cases... ✅ PASSED (25ms)
🧪 Running test: stress_test... ✅ PASSED (2.1s)
🧪 Running test: performance_benchmark... ✅ PASSED (1.8s)
   small_range: avg 45ms
   medium_range: avg 123ms  
   large_range: avg 456ms

📊 Validation Summary:
   Total Tests: 8
   Passed: 8 ✅
   Failed: 0 ❌
   Success Rate: 100.0%
   Report saved to: validation_report.json

✅ Validation completed successfully!
```

### JSON Report Structure

```json
{
  "validation_summary": {
    "total_tests": 8,
    "passed": 8,
    "failed": 0,
  "network": "obtcregtest",
    "timestamp": "2024-11-02T10:30:45Z"
  },
  "test_results": [
    {
      "test_name": "connectivity",
  "network": "obtcregtest",
      "success": true,
      "execution_time": "15ms",
      "timestamp": "2024-11-02T10:30:45Z"
    }
  ],
  "configuration": {
    "network": "regtest",
    "rpc_host": "localhost:18334",
    "start_height": 100,
    "end_height": 1000,
    "max_results": 100
  }
}
```

## 🔍 Test Descriptions

### Core Tests

1. **Connectivity Test**
   - Verifies RPC connection to OBTCD
   - Tests basic `getblockcount` command
   - Ensures credentials are correct

2. **Index Availability Test**
   - Calls `getexpiryindexstats` command
   - Verifies expiry index is enabled and functional
   - Reports index statistics

3. **Basic Query Test**
   - Tests `listexpiring` command with basic parameters
   - Validates response structure
   - Reports result count

4. **Parameter Validation Test**
   - Tests various parameter combinations
   - Validates error handling for invalid inputs
   - Ensures proper parameter constraints

5. **Pagination Test**
   - Tests pagination functionality
   - Verifies `next_height` + `next_outpoint` cursor works correctly
   - Validates large result set handling

6. **Edge Cases Test**
   - Tests empty result sets
   - Tests with nil/null parameters
   - Validates boundary conditions

### Optional Tests

7. **Stress Test** (with `-stress` flag)
   - Performs multiple rapid queries
   - Tests system stability under load
   - Validates concurrent request handling

8. **Performance Benchmark** (with `-bench` flag)
   - Measures query performance across different scenarios
   - Tests small, medium, and large height ranges
   - Reports average response times

## ⚠️ Troubleshooting

### Common Issues

#### "RPC connectivity test failed"
- **Cause**: OBTCD not running or wrong RPC settings
- **Solution**: 
  - Check if OBTCD is running: `ps aux | grep obtcd`
  - Verify RPC credentials match OBTCD configuration
  - Ensure correct host:port for network

#### "expiry index not available"
- **Cause**: ExpiryIndex not enabled or command not recognized
- **Solution**:
  - Ensure OBTCD started with `--expiryindex` flag
  - Verify you're using OBTCD with Week2 changes
  - Check OBTCD version supports expiry index

#### "expiry index is disabled"
- **Cause**: Index still building or disabled due to error
- **Solution**:
  - Wait for initial index build to complete
  - Check OBTCD logs for indexing progress
  - Ensure sufficient disk space for index

#### Performance Issues
- **Cause**: Large height ranges or slow hardware
- **Solution**:
  - Reduce height range for testing
  - Lower `max_results` parameter
  - Use pagination for large datasets

#### Parameter Validation Failures
- **Cause**: Invalid height ranges or network-specific constraints
- **Solution**:
  - Ensure start_height < end_height
  - Use realistic height values for network
  - Check max_results is within limits (1-10000)

### Network-Specific Notes

#### Regtest
- Fast sync and index build
- Ideal for development testing
- Can generate custom test scenarios

#### Testnet
- Longer sync time
- Real Bitcoin network conditions
- Good for integration testing

#### Mainnet
- **READ-ONLY VALIDATION ONLY**
- Long sync time (hours/days)
- Production network conditions
- Use only for final validation

## 📋 Testing Checklist

Before considering Week2 complete, ensure:

- [ ] All core tests pass on regtest
- [ ] All core tests pass on testnet
- [ ] Basic validation passes on mainnet
- [ ] Performance benchmarks show reasonable response times
- [ ] Stress testing shows stability under load
- [ ] Edge cases are handled properly
- [ ] Error conditions are handled gracefully
- [ ] JSON output format is correct
- [ ] Pagination works for large result sets
- [ ] Index statistics are reasonable

## 🛠️ Development Notes

### Extending the Tests

To add new test cases:

1. Add a new test function in `utxo_expiry_validator.go`
2. Register it in the `RunAllTests()` method
3. Update this README with test description

### Performance Baselines

Expected performance baselines (regtest):
- Small range (100 blocks): < 50ms
- Medium range (1000 blocks): < 200ms  
- Large range (5000 blocks): < 1000ms

### Memory Usage

The validation tool should use minimal memory:
- Basic validation: < 50MB
- Stress testing: < 100MB
- Large result pagination: < 200MB

## 🎯 Success Criteria

Week2 ExpiryIndex implementation is considered successful if:

1. **All core tests pass** on all three networks
2. **Performance is acceptable** (< 1s for reasonable queries)
3. **Memory usage is stable** during stress testing
4. **Error handling is robust** for edge cases
5. **API is consistent** with btcd patterns
6. **Documentation is complete** and accurate

This validation suite provides comprehensive coverage to ensure the ExpiryIndex implementation meets production quality standards.
