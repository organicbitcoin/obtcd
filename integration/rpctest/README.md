rpctest
=======

Package rpctest provides an RPC-driven integration test harness for obtcd.

It can launch nodes with Bitcoin-family chain parameters or OBTC parameters.
The in-memory wallet used by tests includes OBTC-specific behavior for excluding
expired UTXOs from spendable balance and using replay-protected sighashes after
activation.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package rpctest is licensed under the [copyfree](http://copyfree.org) ISC
License.
