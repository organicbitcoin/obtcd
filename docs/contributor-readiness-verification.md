# Contributor Build Verification

This record captures clean-clone verification of the public contributor path.
It is evidence for the listed commits and environment only, not a claim that a
public mainnet is ready.

## 2026-11-08

Environment and scope:

- macOS 26.5.1, arm64; Go 1.24.0 selected with `GOTOOLCHAIN=go1.24.0`.
- Fresh remote clone of `docs/contributor-onboarding`, commit
  `1bd907ce47e87bf380afbe1a3c4815ae769ddd32`, using the maintainer SSH profile.
  The contributor guide uses the public HTTPS clone URL for the same repository.
- Independent, initially empty `GOCACHE` and `GOMODCACHE`; toolchain and module
  downloads completed. Tests used `-count=1` to avoid cached test results.
- Fetched `master` was `ed926d74c60a917534f3b2c9ba11eedc16655651`.
  The starting onboarding commit and this preparation change modify no Go
  source, module dependencies, or consensus parameters relative to that baseline.
- The final documentation/template/workflow edits were copied to that isolated
  checkout for the parameter check and minimal quickstart test. No wallet,
  valuable funds, public peer sync, or real keys were used.

| Command | Result |
| --- | --- |
| `go build -o ./btcd .` | passed |
| `go build -o ./btcctl ./cmd/btcctl` | passed |
| `go build -o ./obtc-status ./cmd/obtc-status` | passed |
| `go test ./chaincfg ./wire -run OBTC -count=1` | passed |
| `go test ./mempool -run 'REAP\|RejectREAPSystemTxFromMempool' -count=1` | passed |
| `go test ./mining -run 'REAP\|Accounting\|Witness' -count=1` | passed |
| `go test ./mining/reap ./blockchain/expiryindex -count=1` | passed |
| `go test ./... -count=1` | passed; root module, 30 tested packages |
| `go test ./... -count=1` in each of `btcec`, `btcutil`, `btcutil/psbt`, `chaincfg/chainhash` | all passed; nested modules checked separately |
| `go vet ./...` | passed |
| `go test ./blockchain/expiryindex -run '^TestConnectDisconnectTxOutRoundTrip$' -count=1 -v` | passed; actual named test ran using OBTC regtest parameters |
| `go test -p 1 -tags=rpctest ./integration/... -count=1` | passed; isolated integration harness |
| `make check-obtc-parameters` | passed; 68 matching parameters; unavailable optional sibling-repository sources explicitly skipped |

The minimal round-trip test printed:

```text
=== RUN   TestConnectDisconnectTxOutRoundTrip
--- PASS: TestConnectDisconnectTxOutRoundTrip
PASS
```

Repository checks also passed: `git diff --check`, relative Markdown targets
and anchors in changed pages, Issue template YAML/labels against the live label
list, `bash -n` for 18 existing workflow command blocks, and actionlint v1.7.12
on all four workflows. Actionlint was invoked with `-shellcheck= -pyflakes=`;
those optional linters were not run. No new CI workflow or Go dependency was
introduced. Added content was checked for private paths, email addresses,
credential patterns, and private network endpoints, with manual review of examples.

Remote evidence is separately attributable:

- [Build and Test on the onboarding baseline](https://github.com/organicbitcoin/obtcd/actions/runs/41430822739)
  succeeded for `1bd907ce`, including Go 1.24.0 builds on Linux, macOS and Windows,
  unit coverage/race, OBTC, RPC integration and code-quality jobs.
- [Build and Test on master](https://github.com/organicbitcoin/obtcd/actions/runs/32724234019)
  succeeded for `ed926d74`.
- These historical runs do **not** validate the new, unpushed workflow edits.
  Those still require Actions after an approved push. The new Linux/macOS
  shell instructions were executed locally only on macOS; no new local Linux
  or Windows run is claimed.

Not run in this pass: local race/coverage jobs (available in the linked baseline
CI), devnet demo scenarios, public testnet sync, funded renewal, or mainnet
connectivity. Wallet source and links were inspected read-only, at local
`2171e03` and remote `master` `9372e1d`; `cmd/renewall` code is identical at those
revisions. The guide's removal of `--walletpass` from the dry-run example is
based on its early return before signer execution and existing mock tests,
not a newly executed funded-wallet test. The older wallet timing result below
remains unresolved by this node-only pass.

## 2026-11-02

Environment:

- macOS 26.5.1, arm64;
- Go 1.25.3;
- `obtcd` commit `ed926d74c60a917534f3b2c9ba11eedc16655651`;
- `obtcwallet` commit `9372e1dbc87feba57bbde32a398ee9366c140052`.

Commands and results:

| Repository | Command | Result |
| --- | --- | --- |
| `obtcd` | `go build -o ./btcd .` | passed |
| `obtcd` | `go build -o ./btcctl ./cmd/btcctl` | passed |
| `obtcd` | `go build -o ./obtc-status ./cmd/obtc-status` | passed |
| `obtcd` | `go test ./...` | passed |
| `obtcwallet` | `go build -o ./btcwallet .` | passed |
| `obtcwallet` | `go build -o ./renewall ./cmd/renewall` | passed |
| `obtcwallet` | `go test ./...` | one failure in `chain/TestJitterTicker` |
| `obtcwallet` | `go test ./chain -run '^TestJitterTicker$' -count=10 -v` | passed 10 of 10 reruns |

The wallet result is recorded as a possible timing-sensitive test failure, not
as a successful full-suite verification. This pass did not attempt a public
testnet sync, request test coins, or exercise wallet renewal because those
steps require live network coordination and manually issued testnet coins.
