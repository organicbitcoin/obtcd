# CLAUDE.md — OBTC (btcd fork)

## Project Overview

OBTC is a hard-fork of btcd (Go Bitcoin full node) implementing REAP (Reclaim Expired Assets Protocol). Key additions: 7-year UTXO expiry, 30% value redistribution to miners, replay protection, and separate network parameters.

## Build & Test

```bash
make build          # Build all binaries
make install        # Install to $GOPATH/bin
make unit           # Run unit tests (20m timeout)
make unit-race      # Tests with race detector
make unit-cover     # Coverage reports
make lint           # golangci-lint v2.1.6
make fmt            # goimports + gofmt
```

Integration tests require the `rpctest` build tag:
```bash
go test -tags=rpctest ./integration/... -count=1 -v
```

## Key OBTC Packages

- `chaincfg/params_obtc.go` — OBTC network params, fork heights, `IsOBTC()`, `IsPostOBTCFork()`, `IsOBTCReplayProtectionActive()`
- `blockchain/expiryindex/` — UTXO expiry height indexing
- `blockchain/validation_reap.go` — REAP consensus validation
- `mining/reap/` — REAP block template selection
- `txscript/` — Script validation with replay-protection sighash domain
- `wire/` — Protocol messages with OBTC magic numbers

## Code Conventions

- Go standard: `gofmt` + `goimports`
- Linter config: `.golangci.yml`
- Tests: table-driven with `t.Run()` subtests
- OBTC-specific test files: `*_obtc_*_test.go`
- Module path: `github.com/btcsuite/btcd` (preserved from upstream)

## Network Parameters

| Network | Magic        | Port  |
|---------|-------------|-------|
| MainNet | `0x4F425443` | 9527  |
| TestNet | `0x4F544553` | 19527 |
| RegTest | `0x4F524547` | 29527 |

## Language Rules

- Match the user's language for interactive discussion.
- Use English for PR titles, PR bodies, commit messages, and code comments.

## Git Workflow

- Do not commit directly to `master`.
- Start each task from an up-to-date `master`: `git checkout master && git pull && git checkout -b <branch-name>`
- Branch naming examples: `feat/xxx`, `fix/xxx`, `docs/xxx`, `refactor/xxx`

## Documentation

- `docs/` — Public node documentation and operator guides
- `AGENTS.md` — Directory structure and module boundaries
- `scripts/devnet-up.sh` — DevNet launcher
