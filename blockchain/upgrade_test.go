// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"reflect"
	"testing"

	"github.com/organicbitcoin/obtcd/utxo"
)

// TestDeserializeUtxoEntryV0 ensures deserializing unspent trasaction output
// entries from the legacy version 0 format works as expected.
func TestDeserializeUtxoEntryV0(t *testing.T) {
	tests := []struct {
		name       string
		entries    map[uint32]*utxo.UtxoEntry
		serialized []byte
	}{
		// From tx in main blockchain:
		// 0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098
		{
			name: "Only output 0, coinbase",
			entries: map[uint32]*utxo.UtxoEntry{
				0: {
					Amount:      5000000000,
					PkScript:    hexToBytes("410496b538e853519c726a2c91e61ec11600ae1390813a627c66fb8be7947be63c52da7589379515d4e0a604f8141781e62294721166bf621e73a82cbf2342c858eeac"),
					BlockHeight: 1,
					PackedFlags: utxo.TfCoinBase,
				},
			},
			serialized: hexToBytes("010103320496b538e853519c726a2c91e61ec11600ae1390813a627c66fb8be7947be63c52"),
		},
		// From tx in main blockchain:
		// 8131ffb0a2c945ecaf9b9063e59558784f9c3a74741ce6ae2a18d0571dac15bb
		{
			name: "Only output 1, not coinbase",
			entries: map[uint32]*utxo.UtxoEntry{
				1: {
					Amount:      1000000,
					PkScript:    hexToBytes("76a914ee8bd501094a7d5ca318da2506de35e1cb025ddc88ac"),
					BlockHeight: 100001,
					PackedFlags: 0,
				},
			},
			serialized: hexToBytes("01858c21040700ee8bd501094a7d5ca318da2506de35e1cb025ddc"),
		},
		// Adapted from tx in main blockchain:
		// df3f3f442d9699857f7f49de4ff0b5d0f3448bec31cdc7b5bf6d25f2abd637d5
		{
			name: "Only output 2, coinbase",
			entries: map[uint32]*utxo.UtxoEntry{
				2: {
					Amount:      100937281,
					PkScript:    hexToBytes("76a914da33f77cee27c2a975ed5124d7e4f7f97513510188ac"),
					BlockHeight: 99004,
					PackedFlags: utxo.TfCoinBase,
				},
			},
			serialized: hexToBytes("0185843c010182b095bf4100da33f77cee27c2a975ed5124d7e4f7f975135101"),
		},
		// Adapted from tx in main blockchain:
		// 4a16969aa4764dd7507fc1de7f0baa4850a246de90c45e59a3207f9a26b5036f
		{
			name: "outputs 0 and 2 not coinbase",
			entries: map[uint32]*utxo.UtxoEntry{
				0: {
					Amount:      20000000,
					PkScript:    hexToBytes("76a914e2ccd6ec7c6e2e581349c77e067385fa8236bf8a88ac"),
					BlockHeight: 113931,
					PackedFlags: 0,
				},
				2: {
					Amount:      15000000,
					PkScript:    hexToBytes("76a914b8025be1b3efc63b0ad48e7f9f10e87544528d5888ac"),
					BlockHeight: 113931,
					PackedFlags: 0,
				},
			},
			serialized: hexToBytes("0185f90b0a011200e2ccd6ec7c6e2e581349c77e067385fa8236bf8a800900b8025be1b3efc63b0ad48e7f9f10e87544528d58"),
		},
		// Adapted from tx in main blockchain:
		// 1b02d1c8cfef60a189017b9a420c682cf4a0028175f2f563209e4ff61c8c3620
		{
			name: "Only output 22, not coinbase",
			entries: map[uint32]*utxo.UtxoEntry{
				22: {
					Amount:      366875659,
					PkScript:    hexToBytes("a9141dd46a006572d820e448e12d2bbb38640bc718e687"),
					BlockHeight: 338156,
					PackedFlags: 0,
				},
			},
			serialized: hexToBytes("0193d06c100000108ba5b9e763011dd46a006572d820e448e12d2bbb38640bc718e6"),
		},
	}

	for i, test := range tests {
		// Deserialize to map of utxos keyed by the output index.
		entries, err := deserializeUtxoEntryV0(test.serialized)
		if err != nil {
			t.Errorf("deserializeUtxoEntryV0 #%d (%s) unexpected "+
				"error: %v", i, test.name, err)
			continue
		}

		// Ensure the deserialized entry has the same properties as the
		// ones in the test entry.
		if !reflect.DeepEqual(entries, test.entries) {
			t.Errorf("deserializeUtxoEntryV0 #%d (%s) unexpected "+
				"entries: got %v, want %v", i, test.name,
				entries, test.entries)
			continue
		}
	}
}
