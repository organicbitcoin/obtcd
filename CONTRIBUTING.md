# Contributing to OBTC

OBTC is an independent Bitcoin-derived UTXO lifecycle and proof-of-work
experiment. It is not a Bitcoin consensus change proposal or production
financial infrastructure. Documentation, tests, reproducible bug reports,
tooling, and wallet UX contributions are welcome.

## Build and test

Install Git and Go **1.24.0 or newer** for this node repository. Existing CI
tests Go 1.24.0; the [verification record](docs/contributor-readiness-verification.md)
identifies the exact environments actually tested. The Go module path remains
`github.com/btcsuite/btcd` for upstream compatibility; do not rename it. The
node and RPC client binaries are still named `btcd` and `btcctl`.

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go test ./chaincfg ./wire -run OBTC -count=1
go test ./...
```

The [one-page quickstart](docs/contributing/QUICKSTART.md) adds an isolated
state test and explains expected results. No wallet, test coins, public peers,
or RPC service are needed for this path. Dependency downloads need internet
access. Make and Python 3 are needed for `make check-obtc-parameters`.

Choose an existing guide for deeper work:

- [Reviewer Quick Start](docs/reviewer-quickstart.md): protocol review and focused tests.
- [Public Testnet Self-Test](docs/limited-public-testnet-user-test.md): optional
  node/wallet integration, requiring Go 1.24.6 or newer and manually issued test coins.
- [Mining Review Checklist](docs/mining-review-checklist.md): REAP and templates.
- [Wallet repository](https://github.com/organicbitcoin/obtcwallet): wallet-only
  changes and its own setup, tests, and contribution rules.

## Your first contribution

1. Search [existing issues](https://github.com/organicbitcoin/obtcd/issues),
   especially [good first issue](https://github.com/organicbitcoin/obtcd/labels/good%20first%20issue)
   and [help wanted](https://github.com/organicbitcoin/obtcd/labels/help%20wanted).
   Discuss the task's scope on the issue before starting substantial work.
2. Fork the repository, clone your fork, and create a branch such as
   `git switch -c docs/clarify-build`. Keep the change focused.
3. Run relevant checks. Record the commit, OS, `go version`, exact commands,
   and results, including failures or tests you did not run. Format Go with `gofmt`.
4. Push to your fork and [open a pull request against `master`](https://github.com/organicbitcoin/obtcd/compare).
   Fill in the [PR template](.github/pull_request_template.md) and link the issue.
   Small, useful documentation and test fixes are welcome.

Consensus, cryptography, serialization, REAP, expiry, reorganizations, mining,
and key-management paths require stricter review, positive/negative/boundary
tests, and an explanation of compatibility and activation effects. Do not mix
parameter changes into unrelated work. The upstream-derived
[development guidelines](docs/development/code-contribution-guidelines.md)
provide further code-review conventions; upstream attribution and the
[ISC License](LICENSE) remain in place.

## AI-assisted contributions

Codex, Claude Code, Copilot, and similar tools are allowed. You remain
responsible for correctness: understand and explain the change, inspect every
generated edit, and report only tests you actually ran. Mention material AI
assistance in the PR. Never send private keys, seeds, passwords, production
logs, or other private data to an AI conversation or attach them to a report.

## Safety and review

Use regtest or testnet assets only. Never import a real Bitcoin seed, private
key, or wallet into experimental OBTC software. RPC defaults to loopback;
keep examples and local testing explicitly bound to `127.0.0.1`. Do not expose
unencrypted RPC publicly.

Use [SECURITY.md](SECURITY.md) for sensitive findings, rather than publishing
exploit details. Non-sensitive bug reports should include the commit, network,
OS/Go version, reproduction, expected/actual results, and redacted logs. Remove
private endpoints, usernames, and personal filesystem paths as well as secrets.

Maintainers work on this project in their spare time and cannot guarantee
response times or round-the-clock support. Review may request a narrower scope
or decline a proposal. Follow the [Code of Conduct](CODE_OF_CONDUCT.md).
