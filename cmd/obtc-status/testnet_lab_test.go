// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateLabAlertsDetectsCriticalConditions(t *testing.T) {
	manifest := &labManifest{
		NoBlockWarningSeconds:  1200,
		NoBlockCriticalSeconds: 1800,
	}
	snapshot := &testnetLabSnapshot{
		Summary: labSummary{HeightSpread: 4},
		Nodes: []labNodeSnapshot{
			{
				Node:    labNode{Name: "seed-1"},
				Healthy: true,
				Snapshot: &statusSnapshot{
					Chain: chainStatus{
						Blocks:        120,
						TipAgeSeconds: 1900,
					},
					ExpiryIndex: expiryIndexStatus{
						Available: false,
					},
					ExpiryCommitment: expiryCommitmentStatus{
						Available: false,
					},
				},
			},
		},
		UserWallets: []labWalletSnapshot{
			{Wallet: labWallet{Name: "w-passive"}, Healthy: false, Error: "connection refused"},
		},
		Logs: []labLogSummary{
			{Log: labLog{Name: "obtcd"}, CriticalCount: 1},
		},
	}

	alerts := evaluateLabAlerts(snapshot, manifest)
	if len(alerts) < 5 {
		t.Fatalf("expected multiple alerts, got %+v", alerts)
	}
	if alerts[0].Severity != "critical" {
		t.Fatalf("expected critical alerts to sort first, got %+v", alerts[0])
	}
	if !hasLabAlert(alerts, "node_stale_tip") ||
		!hasLabAlert(alerts, "expiryindex_unavailable") ||
		!hasLabAlert(alerts, "wallet_unavailable") ||
		!hasLabAlert(alerts, "log_critical_pattern") {

		t.Fatalf("missing expected alerts: %+v", alerts)
	}
}

func TestSummarizeLabLogDetectsPatterns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	if err := os.WriteFile(path, []byte("ok\nWARN slow peer\nERROR rpc failed\npanic: boom\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	summary := summarizeLabLog(labLog{Name: "service", Path: path}, 10)
	if summary.WarningCount != 1 || summary.ErrorCount != 1 || summary.CriticalCount != 1 {
		t.Fatalf("unexpected log summary: %+v", summary)
	}
}

func TestReadLabManifestDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lab.json")
	if err := os.WriteFile(path, []byte(`{"nodes":[]}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, warnings, err := readLabManifest(path)
	if err != nil {
		t.Fatalf("readLabManifest: %v", err)
	}
	if manifest.Network != "obtctestnet" {
		t.Fatalf("expected default network, got %q", manifest.Network)
	}
	if manifest.BlockIntervalSeconds != defaultLabBlockIntervalSeconds {
		t.Fatalf("expected default interval, got %d", manifest.BlockIntervalSeconds)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for missing network")
	}
}

func TestResolveLabActionValidatesScriptArgs(t *testing.T) {
	label, args, err := resolveLabAction("renewall-dry-run", map[string][]string{
		"wallet": {"w-renewall"},
	})
	if err != nil {
		t.Fatalf("resolveLabAction: %v", err)
	}
	if label != "renewall-dry-run" || len(args) != 2 || args[1] != "w-renewall" {
		t.Fatalf("unexpected action: label=%s args=%v", label, args)
	}

	if _, _, err := resolveLabAction("renewall-dry-run", map[string][]string{
		"wallet": {"bad;wallet"},
	}); err == nil {
		t.Fatal("expected unsafe wallet token to fail")
	}
}

func hasLabAlert(alerts []labAlert, code string) bool {
	for _, alert := range alerts {
		if alert.Code == code {
			return true
		}
	}
	return false
}
