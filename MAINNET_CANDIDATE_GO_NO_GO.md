# OBTC Mainnet Candidate Go/No-Go Report

Assessment date: 2026-07-02

Decision: **GO WITH NON-BLOCKING LIMITATIONS** for publishing
`v0.1.0-mainnet-candidate.1` as a source-only external technical review release.

This is **not** a GO for a production mainnet launch, a seed-backed public
mainnet operation, exchange integration, custody integration, or real-funds
wallet recommendation.

## Scope Boundary

This decision answers one question:

> Can an external technical reviewer build, run, inspect, test, and challenge
> the OBTC candidate from the public source repositories?

It does not decide whether public DNS seeds, fallback nodes, funded auto-renew,
binary release artifacts, exchanges, custodians, or production operators are
ready.

## Source State

| Item | Value |
|---|---|
| Candidate name | `v0.1.0-mainnet-candidate.1` |
| Release type | source-only external technical review release |
| `obtcd` repository | `organicbitcoin/obtcd` |
| `obtcd` source commit | `115c9255919f8f266e1b7c7ed2ede8df47087807` |
| `obtcwallet` repository | `organicbitcoin/obtcwallet` |
| `obtcwallet` source commit | `0bde8d27b8853fd9cf58e0084dba12788a32fab2` |
| Binary artifacts | not distributed for MC1 |
| Project `SHA256SUMS` | not produced because MC1 has no project-built archives |

Reviewers should build locally from the source commits above. Later RC or
production-scope releases may add binary archives, checksums, and signatures as
a separate gate.

## Mechanical Decision Rules

| Rule | Requirement | Result | Evidence |
|---|---|---:|---|
| R1 | No unresolved P0/P1 implementation blocker for MC1 scope. | PASS | Wallet replay-signing P1 fixed in `obtcwallet` PR #12 and merged. |
| R2 | No open `mainnet-blocker` issues after issue-gate cleanup unless decision is NO-GO. | PASS | The final issue-gate actions close or downgrade prior blocker issues; see `FINAL_ISSUE_GATE_REVIEW.md`. |
| R3 | MC1 release wording must stay external technical review only. | PASS | Release notes, known limitations, and review packet state source-only review scope. |
| R4 | Consensus rules, mainnet parameters, replay protection semantics, expiry, and REAP rules must not be changed for this gate. | PASS | This gate is documentation and issue-scope cleanup only. |
| R5 | Required source/test evidence must be current. | PASS | Focused and broad local tests passed for current `obtcd` and `obtcwallet` commits. |
| R6 | Deferred items must be visible. | PASS | DNS seeds, seed nodes, fresh sync, 72h observation, auto-renew funded drill, and binary artifacts are listed in `KNOWN_LIMITATIONS.md`. |

## Issue Gate Summary

`FINAL_ISSUE_GATE_REVIEW.md` classifies all current open issues.

Items downgraded out of MC1 scope:

- `obtcd` #2: DNS seed provisioning;
- `obtcd` #3: seed/fallback nodes;
- `obtcd` #4: fresh sync from final bootstrap policy;
- `obtcd` #5: 72h mainnet-candidate observation;
- `obtcwallet` #4: funded auto-renew scheduler drill.

Items resolved by explicit source-only policy or new risk documentation:

- `obtcd` #6: node artifacts/checksums for MC1;
- `obtcwallet` #7: wallet artifacts/checksums for MC1;
- `obtcwallet` #8: funded-wallet failure-mode risk review.

## Verification Commands

Commands run during the final gate pass:

```bash
# obtcd
go test ./chaincfg ./txscript ./mempool -run 'OBTC|Replay|SigHash|REAP' -count=1
go test ./blockchain -run 'Replay|REAP|OBTCFullBlock|Consensus|Coinbase' -count=1
go test ./blockchain/expiryindex -run 'Commitment|ReapPrefix|Reorg|Rebuild|Expiry' -count=1
go test ./mining ./mining/reap -run 'REAP|Template|Accounting|Boundary|Marker|Select|Dust|Tax|Weight' -count=1
go test ./... -count=1

# obtcwallet
go test ./wallet -run 'Replay|Sign|Psbt|Renew|Import' -count=1
go test ./waddrmgr -run 'KeyScope|Manager|TaprootPubKeyDerivation|OBTC' -count=1
go test ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -run 'Renew|Unsynced|Signer|Publish|Expiry' -count=1
go test $(go list ./... | grep -v github.com/btcsuite/btcwallet/chain) -count=1
```

All commands above passed.

## No-Go Conditions That Remain For Broader Scopes

The following are still **NO-GO** for seed-backed public operation or production
mainnet wording:

- final DNS seed records are not live;
- long-lived seed/fallback nodes are not deployed;
- fresh sync from the final published bootstrap policy is not recorded;
- 72h observation of the final mainnet-candidate node set is not recorded;
- funded auto-renew scheduler evidence is not recorded;
- project-built binary artifacts/checksums/signatures are not part of MC1;
- no formal third-party security audit is recorded.

If the intended release definition includes any of those claims, this decision
changes to **NO-GO**.
