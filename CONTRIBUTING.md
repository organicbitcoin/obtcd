# Contributing to OBTC

OBTC welcomes focused contributions to documentation, tests, reproducibility,
operator experience, and implementation code. A useful first contribution does
not require understanding the entire protocol.

## Read

Start with the [README](README.md), then choose the shortest relevant guide:

- [Reviewer Quick Start](docs/reviewer-quickstart.md) for protocol review;
- [Public Testnet Self-Test](docs/limited-public-testnet-user-test.md) for node
  and wallet testing;
- [Mining Review Checklist](docs/mining-review-checklist.md) for REAP and block
  template work.

Use Go 1.24.6 or newer. The node currently requires Go 1.24.0, while the
companion wallet requires Go 1.24.6. Git is also required.

## Build

```bash
git clone https://github.com/organicbitcoin/obtcd.git
git clone https://github.com/organicbitcoin/obtcwallet.git

(cd obtcd && go build -o ./btcd . && go build -o ./btcctl ./cmd/btcctl)
(cd obtcwallet && go build -o ./btcwallet . && \
  go build -o ./renewall ./cmd/renewall)
```

Do not import Bitcoin seeds, private keys, or wallet files into OBTC software.
Public testnet coins have no real-world value. Keep RPC listeners on
`127.0.0.1`, and never include credentials or unredacted private logs in an
issue, pull request, or AI conversation.

## Test

Run the tests relevant to your change. For a general node change, start with:

```bash
cd obtcd
go test ./...
go test ./chaincfg ./wire -run OBTC -count=1
go test ./mempool -run 'REAP|RejectREAPSystemTxFromMempool' -count=1
go test ./mining -run 'REAP|Accounting|Witness' -count=1
```

For wallet changes:

```bash
cd obtcwallet
go test ./...
```

Record the operating system, `go version`, commit hash, exact commands, and
results. If a command fails, report the failure accurately instead of claiming
the path is verified. See the latest
[clean-clone verification record](docs/contributor-readiness-verification.md)
for an example.

## Report

Search [existing issues](https://github.com/organicbitcoin/obtcd/issues) before
opening a report. Include the commit hash, network, reproduction steps,
expected result, actual result, and redacted logs. Wallet-only reports belong in
the [wallet issue tracker](https://github.com/organicbitcoin/obtcwallet/issues).

Do not publish exploit details or secrets. Use
[private vulnerability reporting](https://github.com/organicbitcoin/obtcd/security/advisories/new)
for sensitive security reports.

## Contribute

Open a small pull request with a clear purpose and reproducible test evidence.
Documentation fixes, test additions, and well-scoped reproduction reports are
welcome when they materially improve accuracy or usability.

Consensus, signature, REAP, Replay Protection, expiry, reorganization, mining,
and wallet key-management changes receive stricter review. Those pull requests
must include positive, negative, and boundary tests, describe activation-height
effects, and explain any compatibility or fund-safety implications. Do not
change consensus parameters as part of unrelated contributor-experience work.

Format Go code with `gofmt`. Keep logs structured and free of secrets. Write
commit subjects in the imperative mood, prefix them with the affected package
when useful, and keep each commit reviewable. The detailed upstream-derived
[development guidelines](docs/development/code-contribution-guidelines.md)
remain applicable to substantial code changes.

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
