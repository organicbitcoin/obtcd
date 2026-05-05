// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// UTXO expiry index validation tool.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
)

type Config struct {
	RPCHost          string
	RPCUser          string
	RPCPass          string
	RPCCert          string
	Network          string
	Verbose          bool
	MaxResults       int
	OutputFile       string
	Stress           bool
	StressIterations int
	Bench            bool
	StartHeight      int32
	EndHeight        int32
	StartSet         bool
	EndSet           bool
}

type ValidationResult struct {
	TestName      string                 `json:"test_name"`
	Network       string                 `json:"network"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	ExecutionTime string                 `json:"execution_time"`
	Timestamp     string                 `json:"timestamp"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

type ValidationSummary struct {
	TotalTests  int     `json:"total_tests"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
	Network     string  `json:"network"`
	Timestamp   string  `json:"timestamp"`
}

type ValidationReport struct {
	Summary       ValidationSummary      `json:"validation_summary"`
	TestResults   []ValidationResult     `json:"test_results"`
	Configuration map[string]interface{} `json:"configuration"`
}

type testContext struct {
	client      *rpcclient.Client
	config      *Config
	tipHeight   int32
	expiryStats *btcjson.ExpiryIndexStatsResult
}

type testCase struct {
	name string
	fn   func(*testContext) (map[string]interface{}, error)
}

func main() {
	config := parseFlags()

	if err := validateConfig(config); err != nil {
		fatalf("Configuration error: %v", err)
	}

	client, err := createRPCClient(config)
	if err != nil {
		fatalf("Failed to create RPC client: %v", err)
	}
	defer client.Shutdown()

	tipHeight, err := getBlockCount(client)
	if err != nil {
		fatalf("Failed to query block count: %v", err)
	}

	ctx := &testContext{
		client:    client,
		config:    config,
		tipHeight: tipHeight,
	}

	results := runTests(ctx)

	printResults(results)

	if config.OutputFile != "" {
		report := buildReport(ctx, results)
		if err := writeReport(config.OutputFile, report); err != nil {
			fatalf("Failed to write report: %v", err)
		}
		if config.Verbose {
			fmt.Printf("\nReport saved to: %s\n", config.OutputFile)
		}
	}
}

func parseFlags() *Config {
	config := &Config{}
	var startHeight int
	var endHeight int

	flag.StringVar(&config.RPCHost, "rpchost", "", "RPC server host:port (auto-detected if empty)")
	flag.StringVar(&config.RPCUser, "rpcuser", "", "RPC username (required)")
	flag.StringVar(&config.RPCPass, "rpcpass", "", "RPC password (required)")
	flag.StringVar(&config.RPCCert, "rpccert", "", "RPC TLS certificate path (optional)")
	flag.StringVar(&config.Network, "network", "obtcregtest", "Network (obtcregtest/obtctestnet/obtcmainnet or regtest/testnet/mainnet)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose output")
	flag.IntVar(&config.MaxResults, "max", 100, "Maximum results per query")
	flag.StringVar(&config.OutputFile, "output", "", "Write JSON report to file")
	flag.BoolVar(&config.Stress, "stress", false, "Enable stress testing")
	flag.IntVar(&config.StressIterations, "stress-iterations", 10, "Stress test iterations")
	flag.BoolVar(&config.Bench, "bench", false, "Enable performance benchmarking")
	flag.IntVar(&startHeight, "start", -1, "Start height for testing")
	flag.IntVar(&endHeight, "end", -1, "End height for testing")

	flag.Parse()

	if startHeight >= 0 {
		config.StartHeight = int32(startHeight)
		config.StartSet = true
	}
	if endHeight >= 0 {
		config.EndHeight = int32(endHeight)
		config.EndSet = true
	}

	if config.RPCHost == "" {
		switch config.Network {
		case "obtcregtest":
			config.RPCHost = "localhost:29528"
		case "obtctestnet":
			config.RPCHost = "localhost:19528"
		case "obtcmainnet":
			config.RPCHost = "localhost:9528"
		case "regtest":
			config.RPCHost = "localhost:18334"
		case "testnet":
			config.RPCHost = "localhost:18334"
		case "mainnet":
			config.RPCHost = "localhost:8334"
		default:
			config.RPCHost = "localhost:18334"
		}
	}

	return config
}

func validateConfig(config *Config) error {
	if config.RPCUser == "" || config.RPCPass == "" {
		return errors.New("RPC username and password are required")
	}

	switch config.Network {
	case "mainnet", "testnet", "regtest", "obtcmainnet", "obtctestnet", "obtcregtest":
		// ok
	default:
		return fmt.Errorf("invalid network: %s", config.Network)
	}

	if config.MaxResults <= 0 {
		return fmt.Errorf("max results must be positive")
	}

	if config.StressIterations <= 0 {
		return fmt.Errorf("stress iterations must be positive")
	}

	return nil
}

func createRPCClient(config *Config) (*rpcclient.Client, error) {
	rpcConfig := &rpcclient.ConnConfig{
		Host:         config.RPCHost,
		User:         config.RPCUser,
		Pass:         config.RPCPass,
		HTTPPostMode: true,
		DisableTLS:   true,
	}

	if config.RPCCert != "" {
		cert, err := os.ReadFile(filepath.Clean(config.RPCCert))
		if err != nil {
			return nil, fmt.Errorf("unable to read RPC cert: %w", err)
		}
		rpcConfig.DisableTLS = false
		rpcConfig.Certificates = cert
	}

	return rpcclient.New(rpcConfig, nil)
}

func runTests(ctx *testContext) []ValidationResult {
	tests := []testCase{
		{name: "connectivity", fn: testConnectivity},
		{name: "index_availability", fn: testIndexAvailability},
		{name: "basic_query", fn: testBasicQuery},
		{name: "parameter_validation", fn: testParameterValidation},
		{name: "pagination", fn: testPagination},
		{name: "edge_cases", fn: testEdgeCases},
	}

	if ctx.config.Stress {
		tests = append(tests, testCase{name: "stress_test", fn: testStress})
	}
	if ctx.config.Bench {
		tests = append(tests, testCase{name: "performance_benchmark", fn: testBenchmark})
	}

	results := make([]ValidationResult, 0, len(tests))

	for _, test := range tests {
		fmt.Printf("🧪 Running test: %s...", test.name)
		start := time.Now()
		details, err := test.fn(ctx)
		duration := time.Since(start)

		result := ValidationResult{
			TestName:      test.name,
			Network:       ctx.config.Network,
			Success:       err == nil,
			ExecutionTime: duration.String(),
			Timestamp:     time.Now().Format(time.RFC3339),
			Details:       details,
		}

		if err != nil {
			result.ErrorMessage = err.Error()
			fmt.Printf(" ❌ FAILED (%s)\n", duration)
			if ctx.config.Verbose {
				fmt.Printf("   Error: %v\n", err)
			}
		} else {
			fmt.Printf(" ✅ PASSED (%s)\n", duration)
			if ctx.config.Verbose && len(details) > 0 {
				printDetails(details)
			}
		}

		results = append(results, result)
	}

	return results
}

func testConnectivity(ctx *testContext) (map[string]interface{}, error) {
	_, err := ctx.client.GetBlockCount()
	if err != nil {
		return nil, fmt.Errorf("RPC connectivity test failed: %v", err)
	}
	return nil, nil
}

func testIndexAvailability(ctx *testContext) (map[string]interface{}, error) {
	stats, err := getExpiryStats(ctx.client)
	if err != nil {
		return nil, fmt.Errorf("expiry index not available: %v", err)
	}
	ctx.expiryStats = stats

	if stats.Disabled {
		return nil, fmt.Errorf("expiry index is disabled")
	}

	details := map[string]interface{}{
		"total_utxos":       stats.TotalUTXOs,
		"total_expiry_keys": stats.TotalExpiryKeys,
		"tip_height":        stats.TipHeight,
	}
	if stats.NetworkParams != nil {
		details["window_blocks"] = stats.NetworkParams.WindowBlocks
		details["list_batch_limit"] = stats.NetworkParams.ListBatchLimit
		details["start_scan_height"] = stats.NetworkParams.StartScanHeight
		details["enable_at_height"] = stats.NetworkParams.EnableAtHeight
	}

	return details, nil
}

func testBasicQuery(ctx *testContext) (map[string]interface{}, error) {
	startHeight, endHeight := resolveRange(ctx)
	maxResults := ctx.config.MaxResults

	result, err := callListExpiring(ctx.client, &startHeight, &endHeight, &maxResults)
	if err != nil {
		return nil, fmt.Errorf("listexpiring query failed: %v", err)
	}

	if result.StartHeight != startHeight || result.EndHeight != endHeight {
		return nil, fmt.Errorf("unexpected response range: got %d-%d", result.StartHeight, result.EndHeight)
	}
	if result.TotalResults != len(result.ExpiringUTXOs) {
		return nil, fmt.Errorf("result count mismatch: total_results=%d len=%d", result.TotalResults, len(result.ExpiringUTXOs))
	}

	details := map[string]interface{}{
		"start_height":  startHeight,
		"end_height":    endHeight,
		"total_results": result.TotalResults,
	}

	return details, nil
}

func testParameterValidation(ctx *testContext) (map[string]interface{}, error) {
	var failures []string

	startNegative := int32(-1)
	endZero := int32(0)
	maxOne := 1
	if _, err := callListExpiring(ctx.client, &startNegative, &endZero, &maxOne); err == nil {
		failures = append(failures, "expected error for negative start height")
	}

	startTen := int32(10)
	endNine := int32(9)
	if _, err := callListExpiring(ctx.client, &startTen, &endNine, &maxOne); err == nil {
		failures = append(failures, "expected error for end < start")
	}

	maxZero := 0
	if _, err := callListExpiring(ctx.client, &startTen, &startTen, &maxZero); err == nil {
		failures = append(failures, "expected error for max_results <= 0")
	}

	details := map[string]interface{}{
		"checks": 3,
		"failed": len(failures),
	}

	if len(failures) > 0 {
		return details, fmt.Errorf("parameter validation failed: %v", failures)
	}

	return details, nil
}

func testPagination(ctx *testContext) (map[string]interface{}, error) {
	startHeight, endHeight := resolveRange(ctx)
	maxResults := 1

	result, err := callListExpiring(ctx.client, &startHeight, &endHeight, &maxResults)
	if err != nil {
		return nil, fmt.Errorf("pagination initial query failed: %v", err)
	}

	details := map[string]interface{}{
		"start_height":  result.StartHeight,
		"end_height":    result.EndHeight,
		"total_results": result.TotalResults,
	}

	if result.TotalResults == 0 {
		details["note"] = "no results to paginate"
		return details, nil
	}

	if result.TotalResults < maxResults {
		details["note"] = "insufficient results for pagination"
		return details, nil
	}

	if result.NextHeight == nil {
		return details, fmt.Errorf("expected next_height for pagination but got nil")
	}

	nextStart := *result.NextHeight
	horizon := endHeight - startHeight
	if horizon < 1 {
		horizon = 1
	}
	nextEnd := nextStart + horizon
	maxNext := ctx.config.MaxResults

	nextResult, err := callListExpiring(ctx.client, &nextStart, &nextEnd, &maxNext)
	if err != nil {
		return details, fmt.Errorf("pagination follow-up query failed: %v", err)
	}

	if nextResult.StartHeight != nextStart {
		return details, fmt.Errorf("pagination next start height mismatch: got %d want %d", nextResult.StartHeight, nextStart)
	}

	details["next_height"] = nextStart
	details["next_total_results"] = nextResult.TotalResults

	return details, nil
}

func testEdgeCases(ctx *testContext) (map[string]interface{}, error) {
	// Case 1: Nil parameters (use defaults)
	if _, err := callListExpiring(ctx.client, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("listexpiring default parameters failed: %v", err)
	}

	// Case 2: Far future range should return empty set
	offset := int32(1000000)
	if ctx.expiryStats != nil && ctx.expiryStats.NetworkParams != nil {
		window := ctx.expiryStats.NetworkParams.WindowBlocks
		if window > 0 {
			offset = int32(window) + 100
		}
	}

	futureStart := ctx.tipHeight + offset
	futureEnd := futureStart
	maxResults := ctx.config.MaxResults
	futureResult, err := callListExpiring(ctx.client, &futureStart, &futureEnd, &maxResults)
	if err != nil {
		return nil, fmt.Errorf("listexpiring future range failed: %v", err)
	}

	details := map[string]interface{}{
		"default_call":   "ok",
		"future_start":   futureStart,
		"future_end":     futureEnd,
		"future_results": futureResult.TotalResults,
	}

	if futureResult.TotalResults != 0 {
		details["note"] = "future range returned non-empty results"
	}

	return details, nil
}

func testStress(ctx *testContext) (map[string]interface{}, error) {
	iterations := ctx.config.StressIterations
	maxResults := ctx.config.MaxResults

	var totalDuration time.Duration
	failures := 0

	for i := 0; i < iterations; i++ {
		startHeight, endHeight := stressRange(ctx, int32(i))
		start := time.Now()
		_, err := callListExpiring(ctx.client, &startHeight, &endHeight, &maxResults)
		dur := time.Since(start)
		totalDuration += dur
		if err != nil {
			failures++
		}
	}

	details := map[string]interface{}{
		"iterations":       iterations,
		"failures":         failures,
		"average_duration": (totalDuration / time.Duration(iterations)).String(),
		"total_duration":   totalDuration.String(),
	}

	if failures > 0 {
		return details, fmt.Errorf("stress test failed: %d/%d iterations", failures, iterations)
	}

	return details, nil
}

func testBenchmark(ctx *testContext) (map[string]interface{}, error) {
	runs := 3
	maxResults := ctx.config.MaxResults

	smallAvg, err := benchmarkRange(ctx, 10, runs, maxResults)
	if err != nil {
		return nil, fmt.Errorf("small range benchmark failed: %v", err)
	}
	mediumAvg, err := benchmarkRange(ctx, 100, runs, maxResults)
	if err != nil {
		return nil, fmt.Errorf("medium range benchmark failed: %v", err)
	}
	largeAvg, err := benchmarkRange(ctx, 1000, runs, maxResults)
	if err != nil {
		return nil, fmt.Errorf("large range benchmark failed: %v", err)
	}

	details := map[string]interface{}{
		"runs":         runs,
		"small_range":  smallAvg.String(),
		"medium_range": mediumAvg.String(),
		"large_range":  largeAvg.String(),
	}

	return details, nil
}

func benchmarkRange(ctx *testContext, window int32, runs int, maxResults int) (time.Duration, error) {
	var total time.Duration
	for i := 0; i < runs; i++ {
		startHeight, endHeight := benchmarkWindow(ctx, window, int32(i))
		start := time.Now()
		_, err := callListExpiring(ctx.client, &startHeight, &endHeight, &maxResults)
		dur := time.Since(start)
		if err != nil {
			return 0, err
		}
		total += dur
	}
	return total / time.Duration(runs), nil
}

func benchmarkWindow(ctx *testContext, window int32, offset int32) (int32, int32) {
	start := ctx.tipHeight - window - offset
	if start < 0 {
		start = 0
	}
	end := start + window
	return start, end
}

func stressRange(ctx *testContext, i int32) (int32, int32) {
	window := int32(25 + (i % 25))
	start := ctx.tipHeight - window - (i * 3)
	if start < 0 {
		start = 0
	}
	end := start + window
	return start, end
}

func resolveRange(ctx *testContext) (int32, int32) {
	if ctx.config.StartSet && ctx.config.EndSet {
		return ctx.config.StartHeight, ctx.config.EndHeight
	}
	if ctx.config.StartSet && !ctx.config.EndSet {
		end := ctx.config.StartHeight + 1000
		if end > ctx.tipHeight {
			end = ctx.tipHeight
		}
		return ctx.config.StartHeight, end
	}
	if !ctx.config.StartSet && ctx.config.EndSet {
		start := ctx.config.EndHeight - 1000
		if start < 0 {
			start = 0
		}
		return start, ctx.config.EndHeight
	}

	start := ctx.tipHeight - 1000
	if start < 0 {
		start = 0
	}
	return start, ctx.tipHeight
}

func getBlockCount(client *rpcclient.Client) (int32, error) {
	count, err := client.GetBlockCount()
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func getExpiryStats(client *rpcclient.Client) (*btcjson.ExpiryIndexStatsResult, error) {
	result, err := client.RawRequest("getexpiryindexstats", nil)
	if err != nil {
		return nil, err
	}

	var stats btcjson.ExpiryIndexStatsResult
	if err := json.Unmarshal(result, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func callListExpiring(client *rpcclient.Client, start *int32, end *int32, max *int) (*btcjson.ListExpiringResult, error) {
	params := buildListExpiringParams(start, end, max)
	result, err := client.RawRequest("listexpiring", params)
	if err != nil {
		return nil, err
	}

	var list btcjson.ListExpiringResult
	if err := json.Unmarshal(result, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func buildListExpiringParams(start *int32, end *int32, max *int) []json.RawMessage {
	if start == nil && end == nil && max == nil {
		return nil
	}

	params := make([]json.RawMessage, 0, 3)
	if start != nil {
		params = append(params, mustMarshal(*start))
	} else if end != nil || max != nil {
		params = append(params, json.RawMessage("null"))
	}

	if end != nil {
		params = append(params, mustMarshal(*end))
	} else if max != nil {
		params = append(params, json.RawMessage("null"))
	}

	if max != nil {
		params = append(params, mustMarshal(*max))
	}

	return params
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}

func printResults(results []ValidationResult) {
	fmt.Printf("\n📊 Validation Summary:\n")
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("   Total Tests: %d\n", len(results))
	fmt.Printf("   Passed: %d ✅\n", passed)
	fmt.Printf("   Failed: %d ❌\n", failed)
	if len(results) > 0 {
		successRate := float64(passed) / float64(len(results)) * 100
		fmt.Printf("   Success Rate: %.1f%%\n", successRate)
	}

	if failed == 0 {
		fmt.Printf("\n✅ Validation completed successfully!\n")
	} else {
		fmt.Printf("\n❌ Some tests failed. Check OBTCD configuration and ensure --expiryindex is enabled.\n")
		os.Exit(1)
	}
}

func buildReport(ctx *testContext, results []ValidationResult) ValidationReport {
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
	}

	startHeight, endHeight := resolveRange(ctx)

	summary := ValidationSummary{
		TotalTests:  len(results),
		Passed:      passed,
		Failed:      failed,
		SuccessRate: successRate(passed, len(results)),
		Network:     ctx.config.Network,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	configuration := map[string]interface{}{
		"network":      ctx.config.Network,
		"rpc_host":     ctx.config.RPCHost,
		"start_height": startHeight,
		"end_height":   endHeight,
		"max_results":  ctx.config.MaxResults,
	}

	return ValidationReport{
		Summary:       summary,
		TestResults:   results,
		Configuration: configuration,
	}
}

func writeReport(path string, report ValidationReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func successRate(passed int, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(passed) / float64(total) * 100
}

func printDetails(details map[string]interface{}) {
	for key, value := range details {
		fmt.Printf("   %s: %v\n", key, value)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
