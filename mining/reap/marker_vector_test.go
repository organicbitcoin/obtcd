// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestMarkerDigestVector(t *testing.T) {
	h1 := chainhash.Hash{0x01, 0x02, 0x03}
	h2 := chainhash.Hash{0xaa, 0xbb, 0xcc, 0xdd}
	inputs := []wire.OutPoint{
		{Hash: h1, Index: 1},
		{Hash: h2, Index: 257},
	}

	got := MarkerDigest(inputs)
	const want = "eb39b68688466fd7494c455587d1cf7a593137eb305a92b532fb6bb782f8597b"
	if got != want {
		t.Fatalf("unexpected marker digest: got=%s want=%s", got, want)
	}
}
