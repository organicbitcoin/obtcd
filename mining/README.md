mining
======

Package mining builds block templates for obtcd.

The package retains the btcd template generator and adds OBTC-specific template
behavior:

- expiry commitment outputs in coinbase data when required
- deterministic REAP candidate selection and transaction injection
- REAP tax/refund accounting in generated templates
- policy tests for boundary, conflict, dependency, and witness cases

The `reap/` subpackage contains the selector, packer, marker, and dry-run
helpers used by template generation and observability RPCs.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package mining is licensed under the [copyfree](http://copyfree.org) ISC
License.
