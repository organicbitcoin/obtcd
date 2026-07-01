# OBTC Mainnet Candidate Test Report

Status: ready for external technical review, with human confirmation items
listed below. This is not a production mainnet launch report.

## Test Environment

| Field | Value |
|---|---|
| Date | `2026-07-01` |
| Host OS | `Darwin mac.lan 25.5.0 Darwin Kernel Version 25.5.0 ... RELEASE_ARM64_T8112 arm64` |
| Go runtime | `go version go1.25.3 darwin/arm64` |
| `obtcd` commit | `2ad6af61176b9f23e9eb3dc0574c6c15600eb939` |
| `obtcwallet` commit | `c0e8a03fd7fac02a831a65f117679d05aa4625ef` |
| `obtc-website` commit | `55649a5fe960aa9da5ed8f34b1714c4d21a42fbd` |
| Branch used for this package | `codex/plan-08-mainnet-candidate-release-package` |

## Build Commands

```bash
rm -rf /tmp/obtc-plan08-bin && mkdir -p /tmp/obtc-plan08-bin
go build -o /tmp/obtc-plan08-bin/btcd .
go build -o /tmp/obtc-plan08-bin/btcctl ./cmd/btcctl
go build -o /tmp/obtc-plan08-bin/obtc-status ./cmd/obtc-status
```

Result: passed.

## Parameter Consistency

Command:

```bash
python3 scripts/check_obtc_parameters.py --format json
```

Summary command:

```bash
python3 scripts/check_obtc_parameters.py --format json | \
  python3 -c 'import json,sys; d=json.load(sys.stdin); from collections import Counter; print(Counter(d["parameter_statuses"].values())); print(len(d["parameter_statuses"]));'
```

Result:

```text
Counter({'exact match': 68})
68
```

Conclusion: parameter consistency passed for the manifest-backed code and
documentation references. The checker does not prove operational readiness.

## Unit And Focused Test Results

| Area | Command | Result |
|---|---|---|
| chaincfg/wire/status CLI | `go test ./chaincfg ./wire ./cmd/btcctl ./cmd/obtc-status -count=1` | passed |
| mempool isolation and replay policy | `go test ./mempool -run 'REAP|Replay' -count=1` | passed |
| mining/template REAP paths | `go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1` | passed |
| expiry commitment, REAP prefix, rebuild/reorg | `go test ./blockchain/expiryindex -run 'Commitment|REAP|Rebuild|Reorg' -count=1` | passed |
| blockchain REAP/replay/fullblock consensus | `go test ./blockchain -run 'REAP|Replay|OBTCFullBlock|ExpiryCommitment' -count=1` | passed |
| wallet lifecycle and renewal packages | `go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1` in `obtcwallet` | passed |

Observed output:

```text
ok github.com/btcsuite/btcd/chaincfg
ok github.com/btcsuite/btcd/wire
ok github.com/btcsuite/btcd/cmd/btcctl
ok github.com/btcsuite/btcd/cmd/obtc-status
ok github.com/btcsuite/btcd/mempool
ok github.com/btcsuite/btcd/mining
ok github.com/btcsuite/btcd/blockchain/expiryindex
ok github.com/btcsuite/btcd/blockchain
ok github.com/btcsuite/btcwallet/wallet
ok github.com/btcsuite/btcwallet/rpc/legacyrpc
ok github.com/btcsuite/btcwallet/rpc/rpcserver
ok github.com/btcsuite/btcwallet/cmd/renewall
```

## Integration And Regtest Demo Result

Plan 07 reproducible local demo evidence exists in PR #14:

<https://github.com/organicbitcoin/obtcd/pull/14>

Observed there:

- `RESET=1 ./scripts/demo-regtest-expiry-reap.sh` built local binaries;
- started `obtcregtest` with `--txindex --expiryindex --notls`;
- mined through regtest activation;
- showed `getexpiryindexstats`, `getexpirycommitment`, `listexpiring`, and
  `getreapplan`;
- mined a block containing a version `3` REAP transaction;
- printed status JSON with `last_reap`, marker, commitment root, and next
  `reap_plan`.

TODO-HUMAN-CONFIRM: merge PR #14 or re-run the demo from the final release
package branch before publishing a release draft.

## CI Result

Latest related CI evidence:

- PR #14 `docs: add reproducible demo and testnet runbooks` completed all
  Build and Test checks successfully.

Plan 08 package CI:

- TODO-HUMAN-CONFIRM after this PR opens and checks complete.

## Manual Test Steps

Recommended manual review before publishing release artifacts:

1. Build release artifacts with `scripts/phase6/build_release_artifacts.sh`.
2. Verify `SHA256SUMS` and `MANIFEST.md`.
3. Start an `obtctestnet` node with `--txindex --expiryindex`.
4. Start a clean `obtcregtest` node and run the reproducible demo after PR #14
   is merged or equivalent commands are available.
5. Create a fresh test wallet in `obtcwallet`; run `obtc.getexpiry`,
   `obtc.renew` dry review, and `renewall --dry-run`.
6. Run seed preflight and firewall preflight against candidate seed nodes.
7. Capture a 72h observation file if mainnet-candidate seed nodes are publicly
   staged.

## Uncovered Or Evidence-Gated Scenarios

- No independent third-party implementation evidence is recorded in this repo.
- Formal external security audit evidence is `TODO-HUMAN-CONFIRM`.
- Funded mainnet-candidate wallet renewal evidence is not claimed here.
- Long-running public mainnet-candidate seed observation is not included in this
  local report.
- Snapshot/pruned-node behavior is not claimed beyond existing code/test scope.
- Final release tag, artifacts, checksums, and signatures remain
  `TODO-HUMAN-CONFIRM`.

## Failed Tests

No failed commands were recorded in the local focused test set above.

## Conclusion

Based on the commands above, this package is ready for external technical
review. It is not yet a production launch package. The release should not be
published as final until the TODO-HUMAN-CONFIRM items are resolved, especially
final tags/artifacts, Plan 08 CI, Plan 07 demo merge or equivalent demo evidence,
security contact confirmation, and public node observation evidence.
