# First local OBTC check

Use Git and Go **1.24.0 or newer** in a Linux/macOS shell. See the
[dated verification record](../contributor-readiness-verification.md) for
tested environments; Linux builds are also covered by existing CI.
This node-only path needs no wallet, coins, public peers, or running RPC service.
Go may download dependencies on the first run. The inherited module path is
`github.com/btcsuite/btcd`, and the node binary is named `btcd`.

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
git rev-parse --short HEAD
go version
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go test ./chaincfg ./wire -run OBTC -count=1
go test ./blockchain/expiryindex -run '^TestConnectDisconnectTxOutRoundTrip$' -count=1
```

Build commands exit successfully without output. Tests print `ok` for their
packages. The last test connects and disconnects synthetic outputs in an
isolated test database; it does not use real keys, funds, or public peers.
A failure is useful evidence: retain the first error and environment details.

For a broader change, run `go test ./... -count=1` from the repository root.
This covers the root Go module; nested modules and the `rpctest` build-tag
suite are separate gates in the existing [CI](../../.github/workflows/main.yml).
Further focused commands are in the [Reviewer Quick Start](../reviewer-quickstart.md).

If Go is too old, install a supported version before retrying. If dependency
downloads fail, record the module/proxy error after redacting private URLs;
do not change `go.mod` to work around it. Never report a skipped or failed
command as a pass.

Search [existing reports](https://github.com/organicbitcoin/obtcd/issues), then
use the [bug template](https://github.com/organicbitcoin/obtcd/issues/new?template=bug_report.md).
Include commit, OS/architecture, Go version, `network: none (local tests)`,
exact commands, expected/actual results, and redacted output. Sensitive
findings go through [SECURITY.md](../../SECURITY.md).

Continue with [CONTRIBUTING.md](../../CONTRIBUTING.md) for fork → branch → PR,
or the optional [testnet guide](../limited-public-testnet-user-test.md) for
network/wallet tests. Keep any later RPC listener on `127.0.0.1`, and never
import real Bitcoin seeds. The local checks above are a complete first step;
public testnet coins are not required to contribute.
