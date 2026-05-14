// Copyright (c) 2013 The btcsuite developers
// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// Package chaincfg defines chain configuration parameters.
//
// In addition to the standard Bitcoin networks, this fork registers OBTC
// mainnet, testnet, and regtest parameters. OBTC networks are intentionally
// isolated with distinct wire magic values, default ports, address/key
// namespaces, fork heights, UTXO expiry parameters, REAP activation heights,
// expiry commitment activation heights, and replay-protection activation
// heights.
//
// Software should handle errors where input intended for one network is used on
// an application instance running on a different network. This is particularly
// important for OBTC because addresses, keys, P2P messages, and replay-protected
// signatures are designed not to overlap Bitcoin namespaces.
//
// For library packages, chaincfg provides the ability to lookup chain
// parameters and encoding magics when passed a *Params.  Older APIs not updated
// to the new convention of passing a *Params may lookup the parameters for a
// wire.BitcoinNet using ParamsForNet, but be aware that this usage is
// deprecated and will be removed from chaincfg in the future.
//
// For main packages, a (typically global) var may be assigned the address of
// one of the standard Param vars for use as the application's "active" network.
// When a network parameter is needed, it may then be looked up through this
// variable (either directly, or hidden in a library call).
//
//	package main
//
//	import (
//	        "flag"
//	        "fmt"
//	        "log"
//
//	        "github.com/btcsuite/btcd/btcutil"
//	        "github.com/btcsuite/btcd/chaincfg"
//	)
//
//	var testnet = flag.Bool("testnet", false, "operate on the testnet Bitcoin network")
//
//	// By default (without -testnet), use mainnet.
//	var chainParams = &chaincfg.MainNetParams
//
//	func main() {
//	        flag.Parse()
//
//	        // Modify active network parameters if operating on testnet.
//	        if *testnet {
//	                chainParams = &chaincfg.TestNet3Params
//	        }
//
//	        // later...
//
//	        // Create and print new payment address, specific to the active network.
//	        pubKeyHash := make([]byte, 20)
//	        addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, chainParams)
//	        if err != nil {
//	                log.Fatal(err)
//	        }
//	        fmt.Println(addr)
//	}
//
// If an application does not use one of the registered Bitcoin or OBTC
// networks, a new Params struct may be created which defines the parameters for
// the non-standard network. As a general rule of thumb, all network parameters
// should be unique to the network, but parameter collisions can still occur.
package chaincfg
