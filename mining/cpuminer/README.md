cpuminer
========

Package cpuminer provides the CPU mining loop used by tests and local
development networks.

It delegates block construction to the mining template generator. On OBTC
networks that means mined templates may include expiry commitments and REAP
transactions when those rules are active.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package cpuminer is licensed under the [copyfree](http://copyfree.org) ISC
License.
