# OBTC Baseline Audit And Engineering Status

Plan: OBTC 基线审计与工程状态盘点

Branch: `codex/plan-01-baseline-audit`

Audit date: 2026-06-30

Scope: audit only. This report does not change consensus rules, mainnet
parameters, activation heights, REAP semantics, expiry semantics, replay
protection semantics, or wallet renewal semantics.

## Repositories Reviewed

| Repository | Local path | Default branch observed | Audit baseline |
|---|---|---:|---|
| `organicbitcoin/obtcd` | `/Users/pengyu/src/obtcd` | `master` | `36e94c508ea35d1e9d36e992c5d3efa23f5b6ee4` |
| `organicbitcoin/obtcwallet` | `/Users/pengyu/src/obtcwallet` | `master` | local `master`, fetched before audit |
| `organicbitcoin/obtc-website` | `/Users/pengyu/src/obtc-website` | `main` | local `main`, fetched before audit |

需要人工确认:

- The wallet and website baselines should be pinned to exact commits in the
  final external-review packet. This audit used the local fetched checkouts.
- Public GitHub release, issue, milestone, and artifact availability were not
  treated as source of truth unless a local repository file referenced them.

## Repository Structure Summary

`obtcd` owns protocol and node behavior:

- OBTC network parameters: `chaincfg/params_obtc.go`,
  `wire/protocol.go`, `params.go`, `cmd/btcctl/config.go`.
- Expiry, REAP, replay protection, and expiry commitment validation:
  `blockchain/validation_reap.go`, `blockchain/validate.go`,
  `blockchain/validation_obtc_replay.go`,
  `blockchain/expiryindex/`.
- Mining and template behavior:
  `mining/template_reap.go`, `mining/reap/`, `mining/`.
- Node/operator docs, CI, release artifact scripts:
  `README.md`, `docs/`, `.github/workflows/`,
  `scripts/phase6/build_release_artifacts.sh`, `release/README.md`.

`obtcwallet` owns wallet-side expiry inspection and renewal:

- OBTC wallet network ports: `netparams/params.go`.
- Expiry policy and status: `wallet/expiry.go`,
  `wallet/expiry_policy.go`.
- `obtc.getexpiry` and `obtc.renew`: `rpc/legacyrpc/obtc_methods.go`.
- `renewall`: `cmd/renewall/main.go`.
- Auto-renew config and runtime path: `config.go`, `btcwallet.go`,
  `wallet/autorenew.go`.
- Wallet readiness docs and release scripts: `README.md`,
  `docs/mainnet-readiness.md`, `docs/releases/`,
  `scripts/release/build_release_artifacts.sh`.

`obtc-website` owns public-facing material:

- Candidate status and parameter pages: `mainnet-candidate.html`,
  `docs.html`, `versions.html`.
- Whitepaper export/source: `whitepaper.html`, `whitepaper.md`,
  `content/whitepaper-v1.md`.
- Wallet public page/source: `wallet.html`, `wallet.md`.
- Static CI: `.github/workflows/static-checks.yml`,
  `.github/workflows/deploy.yml`.

## Mainnet Candidate Parameter Summary

Code source of truth: `obtcd/chaincfg/params_obtc.go`.

| Parameter | Current mainnet candidate value | Source |
|---|---:|---|
| Fork height | `1000000` | `chaincfg/params_obtc.go:40` |
| First independent OBTC block | `1000001` | `chaincfg/params_obtc.go:50` |
| Replay protection activation height | `1000001` | `chaincfg/params_obtc.go:392` |
| Expiry enable height | `1002016` | `chaincfg/params_obtc.go:390` |
| Expiry commitment activation height | `1002016` | `chaincfg/params_obtc.go:399` |
| REAP hardening height | `1002016` | `chaincfg/params_obtc.go:391` |
| `WindowBlocks` | `362880` | `chaincfg/params_obtc.go:387` |
| Tax ratio | `30 / 100` | `chaincfg/params_obtc.go:396` |
| Refund ratio | `70 / 100` derived from tax | `blockchain/validation_reap.go:491` |
| Dust threshold | `720 sat` | `chaincfg/params_obtc.go:398` |
| REAP normal input cap | `256` | `chaincfg/params_obtc.go:393` |
| REAP dust input cap | `1024` | `chaincfg/params_obtc.go:394` |
| REAP weight budget / consensus max | `400000 WU` | `chaincfg/params_obtc.go:395` |
| Network magic | `0x4F425443` | `wire/protocol.go:199` |
| P2P port | `9527` | `chaincfg/params_obtc.go:71` |
| Node RPC port | `9528` | `params.go:74` |
| Wallet RPC port | `9554` | `obtcwallet/netparams/params.go:44` |
| Bech32 HRP | `obtc` | `chaincfg/params_obtc.go:149` |
| P2PKH / P2SH / WIF | `0x47` / `0x32` / `0x9A` | `chaincfg/params_obtc.go:150` |
| Witness prefixes | `0x2A` / `0x2B` | `chaincfg/params_obtc.go:153` |
| HD private / public | `0B47B01E` / `0B47B5D4` | `chaincfg/params_obtc.go:156` |
| BIP44 coin type | `20260` | `chaincfg/params_obtc.go:160` |

Full cross-network parameter references are in `PARAMETER_REFERENCES.md`.

## Implementation Summary

Expiry and REAP consensus validation are present and centrally wired:

- `blockchain/validation_reap.go:113` validates REAP marker height, count,
  and digest.
- `blockchain/validation_reap.go:159` limits blocks to at most one REAP
  transaction after hardening.
- `blockchain/validation_reap.go:198` checks the REAP transaction against the
  canonical global prefix.
- `blockchain/validation_reap.go:333` enforces canonical order, tier input
  caps, and max weight.
- `blockchain/validation_reap.go:466` enforces tax, refund, and dust-fold
  distribution.
- `blockchain/validation_reap.go:547` rejects ordinary spends of expired UTXOs
  and rejects REAP spends of non-expired UTXOs.
- `blockchain/validate.go:973` wires REAP/expiry checks into transaction input
  validation.

Expiry commitment validation is present:

- `blockchain/expiryindex/commitment.go:14` defines the OP_RETURN commitment
  format and tag.
- `blockchain/expiryindex/expiryindex.go:254` validates commitment before
  processing a block.
- `blockchain/expiryindex/expiryindex.go:608` rejects missing, duplicate,
  wrong-version, or mismatched commitments after activation.

Replay protection is present:

- `txscript/sighash.go:32` defines `SigHashOBTCReplayProtection = 0x40`.
- `txscript/sighash.go:43` defines OBTC replay-protected signature domains.
- `txscript/engine.go:124` defines the script verification flag.
- `blockchain/validation_obtc_replay.go:12` activates the script flag by
  height.
- `blockchain/validate.go:1355`, `mempool/mempool.go:1545`, and
  `mining/mining.go:793` apply the flag in block, mempool, and template paths.

Wallet renewal behavior is present:

- `obtcwallet/rpc/legacyrpc/obtc_methods.go:70` registers `obtc.getexpiry`
  and `obtc.renew`.
- `obtcwallet/rpc/legacyrpc/obtc_methods.go:126` implements `obtc.getexpiry`.
- `obtcwallet/rpc/legacyrpc/obtc_methods.go:315` implements `obtc.renew`.
- `obtcwallet/cmd/renewall/main.go:514` implements dry-run behavior.
- `obtcwallet/btcwallet.go:92` wires auto-renew config after wallet load.
- `obtcwallet/wallet/autorenew.go:187` configures auto-renew.
- `obtcwallet/wallet/autorenew.go:282` runs the scheduler loop.

## Test Coverage Summary

The current codebase has broad OBTC-specific tests across parameters,
consensus validation, expiry index, mining template construction, replay
protection, mempool policy, RPC observability, wallet expiry, renewal, and
focused integration/regtest paths.

Examples:

- Consensus expiry and REAP tests:
  `blockchain/validation_reap_test.go:114`,
  `blockchain/validation_reap_test.go:137`,
  `blockchain/validation_reap_test.go:401`,
  `blockchain/validation_reap_test.go:762`,
  `blockchain/validation_reap_test.go:787`.
- Expiry commitment tests:
  `blockchain/expiryindex/commitment_test.go:166`,
  `blockchain/expiryindex/commitment_test.go:216`,
  `blockchain/expiryindex/commitment_test.go:254`.
- Expiry index connect/disconnect/rebuild tests:
  `blockchain/expiryindex/sequence_fuzz_test.go:20`,
  `blockchain/expiryindex/recovery_integration_test.go:66`,
  `blockchain/expiryindex/expiryindex_test.go:940`,
  `blockchain/expiryindex/expiryindex_test.go:1275`.
- Replay protection tests:
  `txscript/sighash_obtc_replay_test.go:38`,
  `txscript/taproot_obtc_replay_test.go:96`,
  `mempool/policy_matrix_test.go:165`.
- Mining/template tests:
  `mining/template_reap_test.go`,
  `mining/reap/selector_test.go`,
  `mining/newblocktemplate_reap_boundary_test.go`,
  `mining/newblocktemplate_accounting_and_helpers_test.go`.
- Integration/regtest tests:
  `integration/obtc_integration_test.go:96`,
  `integration/obtc_integration_test.go:204`,
  `integration/obtc_integration_test.go:260`,
  `integration/obtc_integration_test.go:799`,
  `integration/obtc_integration_test.go:911`.
- Wallet tests:
  `obtcwallet/wallet/expiry_test.go:9`,
  `obtcwallet/wallet/expiry_policy_test.go:30`,
  `obtcwallet/rpc/legacyrpc/obtc_methods_test.go:15`,
  `obtcwallet/cmd/renewall/main_test.go:308`,
  `obtcwallet/config_autorenew_test.go:25`,
  `obtcwallet/wallet/autorenew_test.go:12`.

Remaining gaps and partial coverage are listed in `TEST_COVERAGE_GAPS.md`.

## CI Summary

`obtcd` CI:

- Build job: `.github/workflows/main.yml:13`.
- Unit coverage: `.github/workflows/main.yml:28`.
- Race tests: `.github/workflows/main.yml:85`.
- OBTC parameter checks: `.github/workflows/main.yml:100`.
- `rpctest` integration: `.github/workflows/main.yml:191`.
- Vet and gofmt: `.github/workflows/main.yml:210`.
- Build matrix: `.github/workflows/main.yml:233`.
- Release artifact workflow: `.github/workflows/release-artifacts.yml:1`.

`obtcwallet` CI:

- Focused wallet readiness smoke: `.github/workflows/obtc-wallet-smoke.yml:1`.
- Wallet release artifact smoke: `.github/workflows/release-artifacts.yml:1`.

`obtc-website` CI:

- Static local-link check only: `.github/workflows/static-checks.yml:1`.
- Deploy workflow also runs the link check before sync:
  `.github/workflows/deploy.yml:30`.

CI gaps:

- No observed cross-repository parameter consistency job that compares
  `obtcd`, `obtcwallet`, and website/whitepaper values.
- Website CI does not run the public-claims checker listed in
  `obtc-website/AGENTS.md:45`.
- Mainnet-candidate release workflows build one selected target per run; a
  complete external-review release matrix and signed manifest flow still need
  human confirmation.

## Documentation Summary

Parameter values are mostly aligned across current docs:

- `obtcd/README.md:96` lists network magic, ports, fork heights, HRP,
  prefixes, and coin types.
- `obtcd/docs/mainnet-params.md:17` lists mainnet network identity and ports.
- `obtcd/docs/mainnet-params.md:83` lists fork, replay, expiry, REAP,
  commitment, window, caps, tax, dust, and weight.
- `obtcd/docs/mainnet-join.md:14` lists the mainnet-candidate network
  baseline.
- `obtcwallet/README.md:28` lists wallet network flags and RPC ports.
- `obtcwallet/docs/mainnet-readiness.md:24` lists wallet mainnet readiness
  checks and known limits.
- `obtc-website/mainnet-candidate.html:97` lists candidate mainnet
  parameters.
- `obtc-website/docs.html:107` lists current candidate parameters.
- `obtc-website/whitepaper.md:264` lists activation matrix and
  `whitepaper.md:347` lists REAP parameter values.
- `obtc-website/versions.html:74` explicitly labels older draft values as
  superseded.

Main documentation risks are stale status text, release-readiness ambiguity,
and missing automated cross-repo doc checks. Details are in
`DOCUMENTATION_RISKS.md`.

## Release Readiness Summary

Current state is mainnet-candidate preparation, not final public mainnet
release.

Observed positives:

- `obtcd/scripts/phase6/build_release_artifacts.sh:1` builds `btcd`,
  `btcctl`, and `obtc-status` artifacts with checksums.
- `obtcd/.github/workflows/release-artifacts.yml:1` exercises artifact build
  and checksum verification.
- `obtcwallet/scripts/release/build_release_artifacts.sh:1` builds
  `btcwallet` and `renewall` artifacts and records the sibling `obtcd` commit.
- `obtcwallet/.github/workflows/release-artifacts.yml:1` exercises wallet
  artifact build and checksum verification.
- `obtcd/docs/mainnet-params.md:30` and `obtcd/docs/mainnet-join.md:26`
  explicitly identify the DNS seed as a placeholder/release blocker.
- Public website status pages repeatedly state review/candidate boundaries
  instead of production or investment claims.

Observed gaps:

- Mainnet DNS seed/fallback-node policy is not final:
  `chaincfg/params_obtc.go:73`, `docs/mainnet-params.md:30`,
  `docs/mainnet-join.md:160`.
- `obtcd/release/README.md:1` is still upstream btcd release-process text and
  references `btcd`/btcsuite release examples; it is not an OBTC-specific
  external reviewer release packet.
- Artifact workflows build a selected target, not a full published multi-OS
  matrix with signed/attested manifests.
- Wallet readiness docs explicitly keep funded `obtc.renew`, non-dry-run
  `renewall`, funded `--autorenew`, remote signer, and backup/restore evidence
  open: `obtcwallet/docs/mainnet-readiness.md:53`.

## Priority Findings

### P0

No P0 issue was found in this audit.

Rationale: no direct mismatch was observed between current mainnet-candidate
consensus parameters in `chaincfg/params_obtc.go` and the primary public
parameter summaries. This is not a final security sign-off.

### P1

1. Mainnet bootstrap policy remains unresolved.
   `chaincfg/params_obtc.go:73` contains `seed.obtc.example.com`; docs mark it
   as a placeholder at `docs/mainnet-params.md:30` and
   `docs/mainnet-join.md:160`.
   Reason: external reviewers need deterministic network-access instructions.
   Status: 需要人工确认 final DNS seed, fallback nodes, or explicit
   no-DNS-seed policy.

2. External-review release artifacts are not yet a complete release packet.
   `scripts/phase6/build_release_artifacts.sh:121` builds three node binaries
   for one target; `.github/workflows/release-artifacts.yml:10` accepts one
   GOOS/GOARCH target per dispatch.
   Reason: reviewers need reproducible artifacts, checksums, signatures or
   attestations, exact commits, and release notes.
   Status: 需要人工确认 final artifact matrix, signing policy, and release
   notes.

3. Wallet funded-operation readiness is explicitly evidence-gated.
   `obtcwallet/docs/mainnet-readiness.md:55` gates funded `obtc.renew`,
   `:57` gates non-dry-run `renewall`, and `:82` gates funded `--autorenew`.
   Reason: mainnet-candidate external review should not imply funded wallet
   readiness until evidence is attached.
   Status: 需要人工确认 whether these are excluded from the first external
   reviewer scope or completed before release.

### P2

1. Website/whitepaper commit pin is stale.
   `obtc-website/whitepaper.md:16`,
   `obtc-website/whitepaper.html:229`, and
   `obtc-website/content/whitepaper-v1.md:16` say the edition was checked
   against `cd9ae639500bbffd82a8b42b1c6ca1c0152c629d`, while this audit's
   `obtcd` baseline is `36e94c508ea35d1e9d36e992c5d3efa23f5b6ee4`.
   Reason: parameter values appear aligned, but the provenance claim is stale.
   Status: 需要人工确认 whether to regenerate/export whitepaper material or add
   a newer verification note.

2. `obtcd` CLI help still mentions inherited Bitcoin default ports.
   `config.go:131` mentions listen defaults `8333`/`18333`; `config.go:168`
   mentions RPC defaults `8334`/`18334`, while OBTC default ports are selected
   by active network at `params.go:74`.
   Reason: help text can mislead operators even though runtime defaults appear
   correct.
   Status: report only; no code change in this plan.

3. Wallet auto-renew status docs are stale or inconsistent.
   `obtcwallet/docs/phase5_implementation_summary.md:95` says in-process
   wallet scheduling is not fully wired, but `obtcwallet/btcwallet.go:92` calls
   `ConfigureAutoRenew` and `obtcwallet/wallet/autorenew.go:282` contains the
   scheduler loop.
   Reason: reviewers may underestimate or misunderstand current wallet behavior.
   Status: 需要人工确认 whether this phase document should be updated or marked
   historical.

4. Cross-repo parameter/document consistency is not enforced by CI.
   `obtcd/.github/workflows/main.yml:122` checks only local chaincfg values;
   `obtc-website/.github/workflows/static-checks.yml:21` checks only links.
   Reason: website, README, whitepaper, wallet docs, and code can drift.
   Status: propose a follow-up CI plan.

5. Wallet projected reclaim ratio is hardcoded in two user-facing/planning
   paths.
   `obtcwallet/rpc/legacyrpc/obtc_methods.go:85` and
   `obtcwallet/wallet/autorenew.go:409` compute `(amountSat * 70) / 100`.
   Reason: this currently matches mainnet `30/100` tax, but it is not visibly
   derived from `chaincfg.GetExpiryParams`.
   Status: 需要人工确认 whether this is acceptable for current fixed parameters
   or should get a drift test/refactor in a later plan.

### P3

1. Historical phase docs and current readiness docs coexist.
   `obtcwallet/docs/phase5_execution_plan.md` is a plan document, while
   `docs/mainnet-readiness.md` is the current readiness boundary.
   Reason: reviewers may read a plan/draft as current state unless clearly
   directed.

2. Website has manual public-claim checks in `AGENTS.md` but not CI.
   `obtc-website/AGENTS.md:45` lists `check_public_claims.py`;
   `.github/workflows/static-checks.yml:21` only runs link checks.
   Reason: not a current claim mismatch, but a maintenance risk.

3. `obtcd/release/README.md` remains upstream-style release documentation.
   Reason: useful as historical process context, but not enough for OBTC
   reviewer onboarding.

## Follow-Up Plan Suggestions

1. P1 release-readiness plan:
   finalize seed/fallback policy, artifact matrix, checksum/signature or
   attestation workflow, release notes, and external reviewer quickstart.

2. P1/P2 wallet funded-flow validation plan:
   record funded `obtc.renew`, non-dry-run `renewall`, funded auto-renew
   backoff behavior, backup/restore, unlock/restart/rescan, and signer-scope
   evidence.

3. P2 cross-repo consistency CI plan:
   add machine-readable parameter extraction from `obtcd`, compare README,
   wallet docs, website, and whitepaper exports, and fail on drift.

4. P2 documentation cleanup plan:
   refresh whitepaper commit provenance, archive or update stale phase docs,
   and replace inherited CLI/release wording that can confuse operators.

5. P2/P3 reviewer packet plan:
   assemble a non-promotional `MAINNET_CANDIDATE_REVIEW.md` with exact commits,
   commands, network access policy, artifacts, checksums, expected tests, and
   known exclusions.
