// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/*
Package expiryindex indexes UTXOs by expiry height for scans, REAP selection,
and expiry commitments.

The index is OBTC-specific. It tracks spendable outputs in expiry-height
buckets, supports paginated scans for RPC and miner selection, maintains a
MuHash accumulator used by expiry commitments, and provides rebuild support for
nodes that enable or reset persisted expiry state.
*/
package expiryindex
