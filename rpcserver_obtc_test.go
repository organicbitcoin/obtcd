// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"
)

func TestParseOutPointCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  string
		wantErr bool
		errMsg  string
		vout    uint32
	}{
		{
			name:   "valid cursor",
			cursor: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:0",
			vout:   0,
		},
		{
			name:   "valid cursor vout 42",
			cursor: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:42",
			vout:   42,
		},
		{
			name:    "missing colon",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "too many colons",
			cursor:  "abc:def:0",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "empty string",
			cursor:  "",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "invalid txid",
			cursor:  "not_a_valid_hex_hash:0",
			wantErr: true,
			errMsg:  "invalid txid",
		},
		{
			name:    "invalid vout not a number",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:abc",
			wantErr: true,
			errMsg:  "invalid vout",
		},
		{
			name:    "negative vout",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:-1",
			wantErr: true,
			errMsg:  "invalid vout",
		},
		{
			name:    "vout overflow uint32",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:4294967296",
			wantErr: true,
			errMsg:  "invalid vout",
		},
		{
			name:   "short txid accepted by chainhash",
			cursor: "abcdef:0",
			vout:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := parseOutPointCursor(tc.cursor)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got %v", tc.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.Index != tc.vout {
				t.Fatalf("expected vout %d, got %d", tc.vout, op.Index)
			}
		})
	}
}
