# OBTC Documentation Risks

Audit date: 2026-06-30

Scope: stale, conflicting, or potentially misleading docs across `obtcd`,
`obtcwallet`, and `obtc-website`. This report is not promotional copy and does
not change any documentation.

## Overall Status

No direct mainnet-candidate parameter value mismatch was found between the
primary code source `obtcd/chaincfg/params_obtc.go` and the main public
parameter summaries reviewed.

The main risks are:

- unresolved release/bootstrap state,
- stale provenance or phase-status docs,
- inherited help/release wording,
- missing automated public-claim and cross-repo parameter checks.

## P0 Risks

No P0 documentation risk was found.

Rationale:

- Primary parameter docs state the current fork, replay, expiry, REAP,
  commitment, window, dust, split, and weight values consistently.
- Website status pages repeatedly state technical review / mainnet-candidate
  boundaries and do not claim production or investment readiness in the
  reviewed files.

## P1 Risks

### DNS Seed / Bootstrap Policy Not Final

References:

- `obtcd/chaincfg/params_obtc.go:73`
- `obtcd/docs/mainnet-params.md:28`
- `obtcd/docs/mainnet-params.md:30`
- `obtcd/docs/mainnet-join.md:26`
- `obtcd/docs/mainnet-join.md:160`

Issue:

The code contains `seed.obtc.example.com`. Docs correctly mark this as a
placeholder/release blocker and instruct explicit peers until final policy is
published.

Risk:

External reviewers need a deterministic way to join or simulate the candidate
network. A placeholder DNS seed is acceptable only if the review packet clearly
states the intended bootstrap policy.

Status:

需要人工确认: replace DNS seed, publish fallback/seed peers, or document that DNS
seed is intentionally unused for the first candidate release.

### External Reviewer Release Packet Is Incomplete

References:

- `obtcd/scripts/phase6/build_release_artifacts.sh:121`
- `obtcd/.github/workflows/release-artifacts.yml:10`
- `obtcd/release/README.md:1`
- `obtcwallet/scripts/release/build_release_artifacts.sh:137`
- `obtcwallet/.github/workflows/release-artifacts.yml:36`
- `obtcwallet/docs/mainnet-readiness.md:94`

Issue:

There are release artifact scripts and checksum manifests, but the observed
workflow is not yet a complete public external-review release packet. The
`obtcd/release/README.md` file is still upstream btcd release-process text and
does not describe OBTC-specific reviewer artifacts, exact candidate commits,
seed/fallback policy, known exclusions, or public evidence links.

Risk:

Reviewers may not know which binaries, commits, checksums, build commands, and
network access policy are authoritative.

Status:

需要人工确认: final artifact matrix, signing/attestation policy, release notes,
reviewer quickstart, and exact `obtcd`/`obtcwallet` commit pins.

### Wallet Funded Operations Are Evidence-Gated

References:

- `obtcwallet/docs/mainnet-readiness.md:55`
- `obtcwallet/docs/mainnet-readiness.md:57`
- `obtcwallet/docs/mainnet-readiness.md:82`
- `obtc-website/wallet.md:21`
- `obtc-website/mainnet-candidate.html:137`

Issue:

Wallet docs intentionally gate funded `obtc.renew`, non-dry-run `renewall`, and
funded auto-renew on recorded evidence. Website wallet text also says funded
renewal and non-dry-run batch renewal remain evidence-gated.

Risk:

This is safe wording, but it means the external reviewer scope must not imply
funded wallet readiness unless evidence is published.

Status:

需要人工确认: either close the evidence items before mainnet-candidate review or
explicitly exclude them from the first reviewer scope.

## P2 Risks

### Whitepaper Commit Provenance Is Stale

References:

- `obtc-website/whitepaper.md:16`
- `obtc-website/whitepaper.html:229`
- `obtc-website/content/whitepaper-v1.md:16`
- Current audited `obtcd` baseline:
  `36e94c508ea35d1e9d36e992c5d3efa23f5b6ee4`

Issue:

The whitepaper source/export says the implementation values were checked
against `cd9ae639500bbffd82a8b42b1c6ca1c0152c629d`, while this audit reviewed
`obtcd` at `36e94c508ea35d1e9d36e992c5d3efa23f5b6ee4`.

Risk:

The parameter values still appear consistent, but the provenance claim is stale.
External reviewers may reasonably ask which commit the whitepaper actually
binds to.

Status:

需要人工确认: regenerate the website/whitepaper export, update the checked
commit, or add a dated verification note.

### Inherited `obtcd` Help Text Mentions Bitcoin Ports

References:

- `obtcd/config.go:131`
- `obtcd/config.go:168`
- Runtime default selection: `obtcd/config.go:820`
- OBTC RPC ports: `obtcd/params.go:74`

Issue:

The help strings for `--listen` and `--rpclisten` still mention inherited
Bitcoin defaults (`8333`, `18333`, `8334`, `18334`). Runtime default selection
uses the active network parameters, including OBTC RPC ports.

Risk:

Operators reading `--help` may choose wrong port assumptions even though the
runtime defaults are correct.

Status:

Report only in this plan. Any wording change should be done in a later docs/UX
plan.

### Wallet Phase 5 Summary Is Stale About Auto-Renew Wiring

References:

- `obtcwallet/docs/phase5_implementation_summary.md:95`
- `obtcwallet/docs/phase5_implementation_summary.md:116`
- `obtcwallet/btcwallet.go:92`
- `obtcwallet/wallet/autorenew.go:187`
- `obtcwallet/wallet/autorenew.go:282`

Issue:

The Phase 5 summary says in-process wallet scheduling is not fully wired and
lists wiring as future work. Current code wires auto-renew config after wallet
load and includes a scheduler loop.

Risk:

Reviewers may receive conflicting signals about whether auto-renew exists,
whether it is default-off, and what remains to validate.

Status:

需要人工确认: mark the phase doc historical, update it, or point readers to
`docs/mainnet-readiness.md` as the current authority.

### Wallet Reclaim Ratio Has Hardcoded 70% Planning Values

References:

- `obtcwallet/rpc/legacyrpc/obtc_methods.go:85`
- `obtcwallet/wallet/autorenew.go:409`
- Chain tax source: `obtcd/chaincfg/params_obtc.go:396`
- Wallet policy source: `obtcwallet/wallet/expiry_policy.go:39`

Issue:

Wallet `getexpiry` and auto-renew planning compute projected reclaim as
`(amountSat * 70) / 100`. This matches current `30 / 100` REAP tax, but it is
not visibly derived from `chaincfg.GetExpiryParams`.

Risk:

If a later parameter change occurs, wallet docs/UI/planning can drift from
consensus unless there is a drift test or derivation.

Status:

需要人工确认: acceptable fixed assumption for current candidate, or follow-up
engineering task.

### Website Claim Checker Is Manual, Not CI-Enforced

References:

- `obtc-website/AGENTS.md:45`
- `obtc-website/.github/workflows/static-checks.yml:21`
- `obtc-website/.github/workflows/deploy.yml:30`

Issue:

`AGENTS.md` requires both link checks and a public-claims checker before
pushing website changes. CI and deploy workflows only run the link checker.

Risk:

Future site changes can introduce production, investment, exchange, or
readiness wording without automated CI rejection.

Status:

Needs a follow-up website CI plan.

### Cross-Repository Parameter Drift Is Not CI-Enforced

References:

- `obtcd/.github/workflows/main.yml:122`
- `obtcwallet/.github/workflows/obtc-wallet-smoke.yml:30`
- `obtc-website/.github/workflows/static-checks.yml:21`

Issue:

`obtcd` CI checks local OBTC params, and wallet CI runs a wallet readiness
claim checker. No observed workflow checks website/whitepaper parameter values
against the `obtcd` source of truth.

Risk:

README, website, whitepaper, and wallet docs can drift from code without a CI
failure.

Status:

Needs a follow-up cross-repo consistency plan.

## P3 Risks

### Historical Draft Values Are Present But Labeled Superseded

References:

- `obtc-website/versions.html:55`
- `obtc-website/versions.html:74`
- `obtc-website/philosophy.md:3`

Issue:

Old values such as `600000`, `950000`, `367920`, dynamic dust formulas, and
older terminology are present in version/archive pages.

Risk:

The files reviewed label them as historical/superseded, so this is not a
current mismatch. It remains a discoverability risk if search engines or
reviewers land on archives without context.

Status:

Monitor. No immediate parameter correction needed.

### `obtcd/release/README.md` Is Upstream-Oriented

References:

- `obtcd/release/README.md:1`
- `obtcd/release/README.md:91`
- `obtcd/release/README.md:119`

Issue:

The release README describes btcd's reproducible build system and uses
btcsuite examples.

Risk:

It may be useful historical process text, but it is not enough for OBTC
external reviewers and can look stale.

Status:

Create OBTC-specific reviewer release instructions in a later plan.

### `renewall` Agent Default Port May Need Reviewer Context

References:

- `obtcwallet/cmd/renewall/main.go:31`
- `obtcwallet/README.md:146`

Issue:

`renewall --connect` defaults to `localhost:8332` for the experimental agent
gRPC server. OBTC wallet legacy RPC defaults are documented as `9554` /
`19554` / `29554`, so readers can confuse the agent gRPC port with legacy RPC.

Risk:

Low if docs show explicit commands, but reviewer packet should spell out legacy
wallet RPC vs agent gRPC separately.

Status:

需要人工确认: whether `8332` is the intended local agent default for all OBTC
networks.

## Public Claim Safety Observations

Reviewed website files are generally careful:

- `obtc-website/mainnet-candidate.html:41` states technical review /
  mainnet-candidate materials.
- `obtc-website/mainnet-candidate.html:42` states not Bitcoin, not a production
  financial network, and not investment advice.
- `obtc-website/mainnet-candidate.html:80` lists no production, investment,
  price, exchange, liquidity, miner revenue, faucet, or Bitcoin proposal
  claims.
- `obtc-website/mainnet-candidate.html:121` points to code as source of truth.
- `obtc-website/mainnet-candidate.html:129` says final release, checksum,
  network access, and observation links are pending.
- `obtc-website/docs.html:41` says private reviewer evidence is not linked until
  ready to publish.
- `obtcwallet/README.md:13` says the current wallet public release is
  source-only engineering preview, not production wallet release.

No promotional or investment-language P0 was found in the reviewed primary
files.

## Recommended Documentation Follow-Ups

1. Create a final external-review release packet:
   exact commits, artifact matrix, checksums, signatures or attestations, seed
   policy, known exclusions, and test commands.
2. Update/regenerate website whitepaper commit provenance.
3. Update or archive stale wallet Phase 5 docs.
4. Add cross-repo parameter consistency CI.
5. Add website public-claims checker to CI and deploy workflows.
6. Add an operator glossary distinguishing node RPC, wallet legacy RPC, and
   wallet agent gRPC ports.
