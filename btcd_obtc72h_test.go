// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/wire"
)

func TestObtcMainNet72hBlockDatabaseSelection(t *testing.T) {
	originalCfg := cfg
	originalParams := activeNetParams
	t.Cleanup(func() {
		cfg = originalCfg
		activeNetParams = originalParams
	})

	baseDir := t.TempDir()
	tests := []struct {
		name    string
		params  *params
		netDir  string
		reuse   bool
		wantDir string
		wantNet wire.BitcoinNet
	}{
		{
			name:    "bitcoin mainnet unchanged",
			params:  &mainNetParams,
			netDir:  "mainnet",
			wantDir: "mainnet",
			wantNet: wire.MainNet,
		},
		{
			name:    "official obtc mainnet unchanged",
			params:  &obtcMainNetParams,
			netDir:  "obtcmainnet",
			wantDir: "obtcmainnet",
			wantNet: wire.ObtcMainNet,
		},
		{
			name:    "rehearsal isolated by default",
			params:  &obtcMainNet72hParams,
			netDir:  "obtcmainnet72h",
			wantDir: "obtcmainnet72h",
			wantNet: wire.ObtcMainNet72h,
		},
		{
			name:    "rehearsal explicitly reuses mainnet database",
			params:  &obtcMainNet72hParams,
			netDir:  "obtcmainnet72h",
			reuse:   true,
			wantDir: "mainnet",
			wantNet: wire.MainNet,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			activeNetParams = test.params
			cfg = &config{
				DataDir:           filepath.Join(baseDir, test.netDir),
				ObtcMainNet72h:    test.params == &obtcMainNet72hParams,
				ReuseMainNetDB72h: test.reuse,
			}

			wantPath := filepath.Join(baseDir, test.wantDir, "blocks_ffldb")
			if got := blockDbPath("ffldb"); got != wantPath {
				t.Fatalf("block db path got %q want %q", got, wantPath)
			}
			if got := blockDbNetwork(); got != test.wantNet {
				t.Fatalf("block db network got %v want %v", got, test.wantNet)
			}
		})
	}
}
