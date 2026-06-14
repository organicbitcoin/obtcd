// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import "testing"

func TestParseConfigRequiresPathsAndAnchor(t *testing.T) {
	_, err := parseConfig([]string{
		"--source-dbpath=/tmp/source",
		"--index-dbpath=/tmp/index",
		"--fork-height=100",
	})
	if err == nil {
		t.Fatal("expected missing fork hash error")
	}

	cfg, err := parseConfig([]string{
		"--source-dbpath=/tmp/source",
		"--index-dbpath=/tmp/index",
		"--fork-height=100",
		"--fork-hash=0000000000000000000000000000000000000000000000000000000000000001",
		"--reset-index",
	})
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if !cfg.ResetIndex || cfg.BatchSize <= 0 || cfg.ReapLimit <= 0 || cfg.ListLimit <= 0 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestResolveDBNet(t *testing.T) {
	for _, name := range []string{"mainnet", "testnet3", "testnet4", "regtest", "simnet"} {
		if _, err := resolveDBNet(name); err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
	}
	if _, err := resolveDBNet("badnet"); err == nil {
		t.Fatal("expected unsupported network error")
	}
}
