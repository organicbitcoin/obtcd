// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// OBTC-only: expiry index used by OBTC networks.

/*
Package expiryindex provides an index for tracking UTXO expiry times.

The expiry index maintains a mapping of UTXOs to their expiry keys (block heights)
to enable efficient scanning of UTXOs that are approaching expiration. This index is used
by the mining subsystem to identify and collect expired UTXOs for network health.

The index maintains two-way mappings:
  - Forward: OutPoint -> ExpiryKey (for fast deletion when UTXOs are spent)
  - Reverse: ExpiryKey -> []OutPoint (for efficient scanning by expiry order)

OBTC uses height-based expiry for deterministic and consensus-friendly behavior.
All UTXOs expire after a fixed number of blocks from their creation height.
All operations are atomic and support blockchain reorganizations through
Connect/Disconnect operations.

Design principles:
  - Deterministic: Same inputs always produce same outputs
  - Atomic: All database operations use transactions
  - Reorg-safe: Support both forward and reverse operations
  - Efficient: Optimized for both insertion and scanning workloads
*/
package expiryindex
