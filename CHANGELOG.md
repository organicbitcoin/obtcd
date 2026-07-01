# Changelog

This changelog follows an engineering review style for OBTC Mainnet Candidate
work. It is not a production launch announcement.

## mainnet-candidate-2026-07

Status: candidate for external technical review.

### Added

- OBTC mainnet, testnet, and regtest parameter sets.
- Expiry index state, scan RPCs, rebuild support, and reorg safety tests.
- Expiry commitment RPC and coinbase commitment validation.
- Replay protection activation tests.
- REAP candidate selection, marker digest, refund/tax construction, validation,
  and mining template append paths.
- Mempool policy isolation for REAP-like system transactions.
- `obtc-status` read-only operator status surface.
- Phase6 operator helpers for seed preflight, validation snapshots, 72h
  observation, config generation, firewall preflight, and release artifacts.
- Mainnet Candidate release package documents:
  - `MAINNET_CANDIDATE_RELEASE_NOTES.md`
  - `MAINNET_CANDIDATE_TEST_REPORT.md`
  - `KNOWN_LIMITATIONS.md`
  - `EXTERNAL_REVIEW_PACKET.md`
  - `SECURITY_REVIEW_CHECKLIST.md`
  - `NODE_OPERATOR_RUNBOOK.md`
  - `WALLET_OPERATOR_RUNBOOK.md`

### Changed

- Documentation now routes reviewers through a Mainnet Candidate review packet.
- Mainnet-candidate wording distinguishes external technical review from
  production launch material.
- Operator documentation emphasizes RPC isolation, expiry index state, and
  release evidence capture.

### Fixed

- No code fixes are introduced by this release package change.
- Prior candidate work added focused fixes and tests for expiry index reorgs,
  REAP accounting, mempool policy, and mining template behavior; see Git history
  and linked PRs for code-level details.

### Tests

- Parameter consistency checker: `68 exact match`.
- Focused local node tests passed:
  - `go test ./chaincfg ./wire ./cmd/btcctl ./cmd/obtc-status -count=1`
  - `go test ./mempool -run 'REAP|Replay' -count=1`
  - `go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1`
  - `go test ./blockchain/expiryindex -run 'Commitment|REAP|Rebuild|Reorg' -count=1`
  - `go test ./blockchain -run 'REAP|Replay|OBTCFullBlock|ExpiryCommitment' -count=1`
- Focused companion wallet tests passed:
  - `go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1`

### Docs

- Added release notes, test report, known limitations, external review packet,
  security checklist, and node/wallet operator runbooks.
- Updated README and docs index with Mainnet Candidate review entry points.

### Known Limitations

- This is not a production mainnet launch.
- Final release tag, artifact URLs, checksums, signatures, and security contact
  wording are `TODO-HUMAN-CONFIRM`.
- Independent third-party implementation and formal security audit evidence are
  not recorded in this repository.
- Public seed/DNS and 72h observation evidence require final operator capture.
- Companion wallet non-dry-run funded evidence remains release-scope dependent.

### Breaking Changes

- No breaking API or command-line changes are introduced by this documentation
  package.

### Consensus-Impacting Changes

- No consensus-impacting code or parameter changes are introduced by this
  documentation package.
- Current consensus parameters remain those recorded in
  `docs/mainnet-params.md` and `chaincfg/params_obtc.go`.

### Non-Consensus Changes

- Release package documentation and review routing.
- Operator and security review checklists.
- Test report summarizing actual local commands.
