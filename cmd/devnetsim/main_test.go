// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
)

func TestResolveNetworkSupportsOBTCTestnet(t *testing.T) {
	net, err := resolveNetwork("obtctestnet")
	if err != nil {
		t.Fatalf("resolveNetwork: %v", err)
	}
	if net != &chaincfg.ObtcTestNetParams {
		t.Fatalf("expected OBTC testnet params, got %s", net.Name)
	}
}

func TestNewSpamRandomizerUsesBaseValueAsMissingBound(t *testing.T) {
	randomizer, err := newSpamRandomizer(120_000, 80_000, 0, false, 123)
	if err != nil {
		t.Fatalf("newSpamRandomizer: %v", err)
	}
	if randomizer == nil {
		t.Fatal("expected randomizer")
	}
	if randomizer.valueMin != 80_000 || randomizer.valueMax != 120_000 {
		t.Fatalf("unexpected bounds: min=%d max=%d", randomizer.valueMin, randomizer.valueMax)
	}
}

func TestSpamRandomizerTxValueWithinRange(t *testing.T) {
	randomizer, err := newSpamRandomizer(120_000, 80_000, 140_000, false, 99)
	if err != nil {
		t.Fatalf("newSpamRandomizer: %v", err)
	}

	for i := 0; i < 100; i++ {
		value := randomizer.txValue(120_000)
		if value < 80_000 || value > 140_000 {
			t.Fatalf("value %d outside expected range", value)
		}
	}
}

func TestSplitValueRandomPreservesTotalAndAvoidsDust(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	total := btcutil.Amount(12_000)
	parts := splitValueRandom(total, 3, rng)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}

	var sum btcutil.Amount
	for _, part := range parts {
		sum += part
		if part <= defaultDustThreshold {
			t.Fatalf("expected non-dust part, got %d", part)
		}
	}
	if sum != total {
		t.Fatalf("expected sum %d, got %d", total, sum)
	}
}
