# btcd Heritage Notes

`obtcd` is derived from btcd and intentionally preserves several upstream
shapes:

* the executable name is still `btcd`;
* the Go module path remains `github.com/btcsuite/btcd`;
* many packages, RPC types, and examples retain btcd naming;
* shared node behavior such as database, P2P, RPC, TLS, Tor, and mining
  configuration follows btcd unless an OBTC document or code path says
  otherwise.

OBTC-specific behavior is selected at runtime with:

* `--obtcmainnet`
* `--obtctestnet`
* `--obtcregtest`

When a reference document mentions inherited btcd behavior, read it as a shared
implementation detail. When a document mentions OBTC ports, activation rules,
expiry index, REAP, replay protection, or address/key namespaces, the OBTC
specific values are authoritative.
