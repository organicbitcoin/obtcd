integration
===========

This directory contains integration tests that drive obtcd nodes through RPC.

The rpctest harness can launch the inherited Bitcoin-family networks and the
OBTC networks. OBTC integration coverage exercises expiry index RPCs, REAP
validation, replay-protected wallet signing, and expiry commitment behavior.

Run integration tests with the `rpctest` build tag:

```bash
go test -tags=rpctest ./integration/... -count=1
```

## License

This code is licensed under the [copyfree](http://copyfree.org) ISC License.
