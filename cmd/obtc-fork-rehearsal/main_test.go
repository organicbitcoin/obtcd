// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import "testing"

func TestParseConfigRequiresAnchor(t *testing.T) {
	_, err := parseConfig([]string{"--dbpath=/tmp/blocks_ffldb", "--fork-height=100"})
	if err == nil {
		t.Fatal("expected missing fork hash error")
	}
}

func TestRehearsalParamsOverrideForkDAA(t *testing.T) {
	params := rehearsalParams(953600)
	if params.ForkDAAStartHeight != 953601 {
		t.Fatalf("start height got %d", params.ForkDAAStartHeight)
	}
	if params.ForkDAABootstrapEndHeight != 955616 {
		t.Fatalf("bootstrap end got %d", params.ForkDAABootstrapEndHeight)
	}
	if params.ForkDAAForkResetBits != 0x1d00ffff {
		t.Fatalf("reset bits got %08x", params.ForkDAAForkResetBits)
	}
}

func TestResolveDBNet(t *testing.T) {
	for _, name := range []string{"mainnet", "testnet3", "testnet4", "regtest", "simnet"} {
		if _, err := resolveDBNet(name); err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
	}
	if _, err := resolveDBNet("unknown"); err == nil {
		t.Fatal("expected unsupported network error")
	}
}
