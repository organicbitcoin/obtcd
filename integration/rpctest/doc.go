// Copyright (c) 2016 The btcsuite developers
// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// Package rpctest provides a btcd-specific RPC testing harness crafting and
// executing integration tests by driving a `btcd` instance via the `RPC`
// interface. Each instance of an active harness comes equipped with a simple
// in-memory HD wallet capable of properly syncing to the generated chain,
// creating new addresses, and crafting fully signed transactions paying to an
// arbitrary set of outputs.
//
// This package was designed specifically to act as an RPC testing harness for
// `btcd`. However, the constructs presented are general enough to be adapted to
// any project wishing to programmatically drive a `btcd` instance of its
// systems/integration tests.
//
// In obtcd, the harness also understands OBTC chain parameters and applies
// replay-protected signing after the active OBTC network height requires it.
package rpctest
