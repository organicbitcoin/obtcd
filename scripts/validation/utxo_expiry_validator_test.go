// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

// ---------------------------------------------------------------------------
// validateConfig
// ---------------------------------------------------------------------------

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  &Config{RPCUser: "u", RPCPass: "p", Network: "obtcregtest", MaxResults: 100, StressIterations: 10},
			wantErr: false,
		},
		{
			name:    "missing user",
			config:  &Config{RPCUser: "", RPCPass: "p", Network: "obtcregtest", MaxResults: 100, StressIterations: 10},
			wantErr: true,
			errMsg:  "required",
		},
		{
			name:    "missing pass",
			config:  &Config{RPCUser: "u", RPCPass: "", Network: "obtcregtest", MaxResults: 100, StressIterations: 10},
			wantErr: true,
			errMsg:  "required",
		},
		{
			name:    "invalid network",
			config:  &Config{RPCUser: "u", RPCPass: "p", Network: "badnet", MaxResults: 100, StressIterations: 10},
			wantErr: true,
			errMsg:  "invalid network",
		},
		{
			name:    "zero max results",
			config:  &Config{RPCUser: "u", RPCPass: "p", Network: "mainnet", MaxResults: 0, StressIterations: 10},
			wantErr: true,
			errMsg:  "max results",
		},
		{
			name:    "negative max results",
			config:  &Config{RPCUser: "u", RPCPass: "p", Network: "mainnet", MaxResults: -1, StressIterations: 10},
			wantErr: true,
			errMsg:  "max results",
		},
		{
			name:    "zero stress iterations",
			config:  &Config{RPCUser: "u", RPCPass: "p", Network: "testnet", MaxResults: 100, StressIterations: 0},
			wantErr: true,
			errMsg:  "stress iterations",
		},
	}

	validNetworks := []string{"mainnet", "testnet", "regtest", "obtcmainnet", "obtctestnet", "obtcregtest"}
	for _, net := range validNetworks {
		tests = append(tests, struct {
			name    string
			config  *Config
			wantErr bool
			errMsg  string
		}{
			name:    "valid network " + net,
			config:  &Config{RPCUser: "u", RPCPass: "p", Network: net, MaxResults: 100, StressIterations: 10},
			wantErr: false,
		})
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.config)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildListExpiringParams
// ---------------------------------------------------------------------------

func TestBuildListExpiringParams(t *testing.T) {
	t.Run("all nil", func(t *testing.T) {
		params := buildListExpiringParams(nil, nil, nil)
		if params != nil {
			t.Fatalf("expected nil params, got %v", params)
		}
	})

	t.Run("start only", func(t *testing.T) {
		s := int32(100)
		params := buildListExpiringParams(&s, nil, nil)
		if len(params) != 1 {
			t.Fatalf("expected 1 param, got %d", len(params))
		}
		var v int32
		if err := json.Unmarshal(params[0], &v); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if v != 100 {
			t.Fatalf("expected 100, got %d", v)
		}
	})

	t.Run("start and end", func(t *testing.T) {
		s := int32(100)
		e := int32(200)
		params := buildListExpiringParams(&s, &e, nil)
		if len(params) != 2 {
			t.Fatalf("expected 2 params, got %d", len(params))
		}
	})

	t.Run("all set", func(t *testing.T) {
		s := int32(10)
		e := int32(20)
		m := 50
		params := buildListExpiringParams(&s, &e, &m)
		if len(params) != 3 {
			t.Fatalf("expected 3 params, got %d", len(params))
		}
	})

	t.Run("end only adds null start", func(t *testing.T) {
		e := int32(200)
		params := buildListExpiringParams(nil, &e, nil)
		if len(params) != 2 {
			t.Fatalf("expected 2 params, got %d", len(params))
		}
		if string(params[0]) != "null" {
			t.Fatalf("expected null for start, got %s", string(params[0]))
		}
	})

	t.Run("max only adds null start and end", func(t *testing.T) {
		m := 50
		params := buildListExpiringParams(nil, nil, &m)
		if len(params) != 3 {
			t.Fatalf("expected 3 params, got %d", len(params))
		}
		if string(params[0]) != "null" {
			t.Fatalf("expected null for start, got %s", string(params[0]))
		}
		if string(params[1]) != "null" {
			t.Fatalf("expected null for end, got %s", string(params[1]))
		}
	})
}

// ---------------------------------------------------------------------------
// resolveRange
// ---------------------------------------------------------------------------

func TestResolveRange(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *testContext
		wantStart int32
		wantEnd   int32
	}{
		{
			name: "both set",
			ctx: &testContext{
				tipHeight: 5000,
				config:    &Config{StartHeight: 100, EndHeight: 200, StartSet: true, EndSet: true},
			},
			wantStart: 100,
			wantEnd:   200,
		},
		{
			name: "start only",
			ctx: &testContext{
				tipHeight: 5000,
				config:    &Config{StartHeight: 100, StartSet: true, EndSet: false},
			},
			wantStart: 100,
			wantEnd:   1100,
		},
		{
			name: "start only capped to tip",
			ctx: &testContext{
				tipHeight: 500,
				config:    &Config{StartHeight: 100, StartSet: true, EndSet: false},
			},
			wantStart: 100,
			wantEnd:   500,
		},
		{
			name: "end only",
			ctx: &testContext{
				tipHeight: 5000,
				config:    &Config{EndHeight: 2000, StartSet: false, EndSet: true},
			},
			wantStart: 1000,
			wantEnd:   2000,
		},
		{
			name: "end only clamp start to 0",
			ctx: &testContext{
				tipHeight: 5000,
				config:    &Config{EndHeight: 500, StartSet: false, EndSet: true},
			},
			wantStart: 0,
			wantEnd:   500,
		},
		{
			name: "neither set",
			ctx: &testContext{
				tipHeight: 5000,
				config:    &Config{StartSet: false, EndSet: false},
			},
			wantStart: 4000,
			wantEnd:   5000,
		},
		{
			name: "neither set low tip",
			ctx: &testContext{
				tipHeight: 500,
				config:    &Config{StartSet: false, EndSet: false},
			},
			wantStart: 0,
			wantEnd:   500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := resolveRange(tc.ctx)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("resolveRange = (%d, %d), want (%d, %d)",
					start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// stressRange
// ---------------------------------------------------------------------------

func TestStressRange(t *testing.T) {
	ctx := &testContext{tipHeight: 5000, config: &Config{}}

	for i := int32(0); i < 10; i++ {
		start, end := stressRange(ctx, i)
		if start < 0 {
			t.Fatalf("start must be non-negative, got %d for i=%d", start, i)
		}
		if end <= start {
			t.Fatalf("end must be > start, got start=%d end=%d for i=%d", start, end, i)
		}
	}
}

func TestStressRangeLowTip(t *testing.T) {
	ctx := &testContext{tipHeight: 10, config: &Config{}}
	start, end := stressRange(ctx, 50)
	if start < 0 {
		t.Fatalf("start clamped to 0, got %d", start)
	}
	if end <= start {
		t.Fatalf("end must be > start, got start=%d end=%d", start, end)
	}
}

// ---------------------------------------------------------------------------
// benchmarkWindow
// ---------------------------------------------------------------------------

func TestBenchmarkWindow(t *testing.T) {
	ctx := &testContext{tipHeight: 1000, config: &Config{}}

	start, end := benchmarkWindow(ctx, 100, 0)
	if start != 900 || end != 1000 {
		t.Fatalf("expected (900,1000), got (%d,%d)", start, end)
	}

	start, end = benchmarkWindow(ctx, 100, 950)
	if start < 0 {
		t.Fatalf("start must be non-negative, got %d", start)
	}
	if end != start+100 {
		t.Fatalf("expected end = start+100, got start=%d end=%d", start, end)
	}
}

// ---------------------------------------------------------------------------
// successRate
// ---------------------------------------------------------------------------

func TestSuccessRate(t *testing.T) {
	tests := []struct {
		passed, total int
		want          float64
	}{
		{0, 0, 0},
		{1, 1, 100},
		{5, 10, 50},
		{3, 4, 75},
		{0, 5, 0},
	}

	for _, tc := range tests {
		got := successRate(tc.passed, tc.total)
		if got != tc.want {
			t.Fatalf("successRate(%d,%d) = %f, want %f", tc.passed, tc.total, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// buildReport
// ---------------------------------------------------------------------------

func TestBuildReport(t *testing.T) {
	ctx := &testContext{
		tipHeight: 1000,
		config: &Config{
			Network:    "obtcregtest",
			RPCHost:    "localhost:29528",
			MaxResults: 100,
			StartSet:   false,
			EndSet:     false,
		},
	}

	results := []ValidationResult{
		{TestName: "test1", Success: true, ExecutionTime: "1ms", Timestamp: time.Now().Format(time.RFC3339)},
		{TestName: "test2", Success: false, ErrorMessage: "fail", ExecutionTime: "2ms", Timestamp: time.Now().Format(time.RFC3339)},
		{TestName: "test3", Success: true, ExecutionTime: "3ms", Timestamp: time.Now().Format(time.RFC3339)},
	}

	report := buildReport(ctx, results)

	if report.Summary.TotalTests != 3 {
		t.Fatalf("expected 3 total tests, got %d", report.Summary.TotalTests)
	}
	if report.Summary.Passed != 2 {
		t.Fatalf("expected 2 passed, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", report.Summary.Failed)
	}
	if report.Summary.Network != "obtcregtest" {
		t.Fatalf("expected network obtcregtest, got %s", report.Summary.Network)
	}
	if len(report.TestResults) != 3 {
		t.Fatalf("expected 3 test results, got %d", len(report.TestResults))
	}
}

// ---------------------------------------------------------------------------
// writeReport + round-trip
// ---------------------------------------------------------------------------

func TestWriteReportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_report.json")

	report := ValidationReport{
		Summary: ValidationSummary{
			TotalTests:  2,
			Passed:      1,
			Failed:      1,
			SuccessRate: 50,
			Network:     "obtcregtest",
			Timestamp:   time.Now().Format(time.RFC3339),
		},
		TestResults: []ValidationResult{
			{TestName: "a", Success: true},
			{TestName: "b", Success: false, ErrorMessage: "err"},
		},
		Configuration: map[string]interface{}{
			"network": "obtcregtest",
		},
	}

	if err := writeReport(path, report); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	var loaded ValidationReport
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	if loaded.Summary.TotalTests != 2 {
		t.Fatalf("expected 2 total tests, got %d", loaded.Summary.TotalTests)
	}
	if loaded.Summary.SuccessRate != 50 {
		t.Fatalf("expected 50%% success rate, got %f", loaded.Summary.SuccessRate)
	}
}

// ---------------------------------------------------------------------------
// mustMarshal
// ---------------------------------------------------------------------------

func TestMustMarshal(t *testing.T) {
	raw := mustMarshal(42)
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
}

// ---------------------------------------------------------------------------
// ExpiryIndexStatsResult JSON serialization
// ---------------------------------------------------------------------------

func TestExpiryIndexStatsResultJSON(t *testing.T) {
	result := btcjson.ExpiryIndexStatsResult{
		Disabled:        false,
		TipHeight:       1000,
		TotalUTXOs:      500,
		TotalExpiryKeys: 100,
		NetworkParams: &btcjson.ExpiryParamsResult{
			WindowBlocks:    52560,
			ListBatchLimit:  5000,
			StartScanHeight: 0,
			EnableAtHeight:  100,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var loaded btcjson.ExpiryIndexStatsResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.TipHeight != 1000 {
		t.Fatalf("expected tip_height 1000, got %d", loaded.TipHeight)
	}
	if loaded.NetworkParams == nil || loaded.NetworkParams.WindowBlocks != 52560 {
		t.Fatalf("unexpected network params: %+v", loaded.NetworkParams)
	}
}
