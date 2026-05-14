mempool
=======

Package mempool provides a policy-enforced pool of unmined transactions for
obtcd.

The package keeps the inherited btcd transaction admission model and adds the
OBTC policy hooks needed by this fork:

- replay-protection script flags are applied after OBTC activation
- REAP system transactions are rejected from ordinary mempool admission
- orphan resolution and replacement policy tests cover replay-protected spends

REAP transactions are constructed by mining template logic from expiry index
state, not relayed through the normal user transaction pool.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package mempool is licensed under the [copyfree](http://copyfree.org) ISC
License.
