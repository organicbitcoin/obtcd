# OBTC Mainnet Candidate Test Report

Status: ready for source-only external technical review with non-blocking
limitations. This is not a production mainnet launch report.

## Test Environment

| Field | Value |
|---|---|
| Date | `2026-07-02` |
| Host OS | `Darwin mac.lan 25.5.0 Darwin Kernel Version 25.5.0 RELEASE_ARM64_T8112 arm64` |
| Go runtime | `go version go1.25.3 darwin/arm64` |
| `obtcd` gate-cleanup baseline | `2828dad2aeba136ae1539ccc47b0a28c331a8729` |
| `obtcwallet` gate-cleanup baseline | `ea0070517641fa9f0ba5731b903aae1c57f24d5a` |
| Candidate | `v0.1.0-mainnet-candidate.1` |

## Build Scope

MC1 is source-only. Reviewers build locally from source. No project-built binary
archives or project `SHA256SUMS` are distributed for this scope.

## Unit And Focused Test Results

| Area | Command | Result |
|---|---|---|
| replay, sighash, chain params, mempool | `go test ./chaincfg ./txscript ./mempool -run 'OBTC|Replay|SigHash|REAP' -count=1` | passed |
| blockchain consensus paths | `go test ./blockchain -run 'Replay|REAP|OBTCFullBlock|Consensus|Coinbase' -count=1` | passed |
| expiry index | `go test ./blockchain/expiryindex -run 'Commitment|ReapPrefix|Reorg|Rebuild|Expiry' -count=1` | passed |
| mining and REAP construction | `go test ./mining ./mining/reap -run 'REAP|Template|Accounting|Boundary|Marker|Select|Dust|Tax|Weight' -count=1` | passed |
| full node repository local test suite | `go test ./... -count=1` | passed |
| wallet replay/signing/PSBT/import/renewal | `go test ./wallet -run 'Replay|Sign|Psbt|Renew|Import' -count=1` in `obtcwallet` | passed |
| wallet HD scope manager | `go test ./waddrmgr -run 'KeyScope|Manager|TaprootPubKeyDerivation|OBTC' -count=1` in `obtcwallet` | passed |
| wallet RPC, signer, publish, expiry | `go test ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -run 'Renew|Unsynced|Signer|Publish|Expiry' -count=1` in `obtcwallet` | passed |
| wallet scoped broad local gate | `go test $(go list ./... \| grep -v github.com/btcsuite/btcwallet/chain) -count=1` in `obtcwallet` | passed |

## Review Evidence Added Before Gate

- `obtcd` replay-protection sighash matrix now covers `SIGHASH_ALL`,
  `SIGHASH_NONE`, `SIGHASH_SINGLE`, and valid `ANYONECANPAY` combinations
  across legacy, SegWit v0 P2WPKH, SegWit v0 P2WSH multisig, Taproot key path,
  and Taproot script path.
- `REVIEW_FIXTURE_VECTORS.md` provides concrete fixtures for replay activation,
  replay hash bytes, expiry boundaries, REAP marker digest, REAP accounting,
  canonical ordering, and coinbase overclaim rejection.
- `obtcwallet` PR #12 fixed stale-height replay signing policy and merged into
  master.
- `obtcwallet` PR #12 documented and tested OBTC wallet HD namespace policy.
- `obtcwallet` PR #12 added local finalized PSBT validation for remote
  signer/publish paths.
- `WALLET_OPERATOR_RISK_REVIEW.md` records funded-wallet failure-mode guidance.

## Uncovered Or Evidence-Gated Scenarios

Non-blocking for MC1 source review, but required before broader claims:

- final DNS seed A/AAAA records;
- long-lived seed/fallback nodes;
- fresh-node sync from final published bootstrap policy;
- 72h final mainnet-candidate node-set observation;
- funded auto-renew scheduler drill;
- project-built binary artifacts, checksums, and signatures;
- formal third-party security audit evidence;
- independent third-party implementation evidence.

## Failed Tests

No failed commands were recorded in the final gate command set above.

## Conclusion

Based on the commands above, the current source commits are ready for external
technical review with the limitations listed in `KNOWN_LIMITATIONS.md` and
`FINAL_ISSUE_GATE_REVIEW.md`. They are not a production launch package.
