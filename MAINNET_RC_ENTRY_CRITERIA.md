# OBTC Mainnet RC Entry Criteria

This document defines the evidence needed before an OBTC Mainnet Release
Candidate may be presented for external technical review. It does not authorize
a production mainnet launch or any third-party integration claim.

## Scope

The RC scope is source, binaries, reproducibility evidence, public review
materials, and long-running testnet validation. It does not require a complete
third-party review of the entire project before publishing an RC, but the
release wording must clearly state that no formal third-party security audit has
been completed unless such an audit actually exists.

## Required Entry Evidence

| Area | Entry requirement |
|---|---|
| Source state | Candidate commit and tag are identified. |
| Parameter consistency | Network, fork, replay, expiry, REAP, tax/refund, dust, and weight parameters are checked against public docs. |
| Public review packet | `EXTERNAL_REVIEW_PACKET.md`, `OBTC_REVIEWER_PRIMER.md`, `review-cards/`, `REVIEW_TEST_VECTORS.md`, and `KNOWN_LIMITATIONS.md` are current. |
| Modular review flow | Review cards allow a reviewer to inspect one mechanism without claiming whole-project review. |
| Test vectors | Valid and invalid cases cover REAP, replay protection, expiry index, coinbase accounting, and wallet renewal. |
| Local reproducibility | Build, focused tests, and local demo commands are documented and pass for the candidate source. |
| Wallet evidence | Wallet renewal and auto-renew safety tests are linked from the review packet, with funded-operation limits stated. |
| Testnet validation | Long-running public testnet or controlled multi-node validation is recorded with commands, commit, network, heights, and logs. |
| Limitations | No formal third-party audit and all other known release limits are stated plainly. |
| Issue intake | Public issue templates route review findings into reproducible reports. |

## Substitute For Complete Third-Party Review

Because OBTC's protocol review surface is specialized, the RC may proceed
without a complete third-party review of the entire project when all of the
following are true:

1. The public review packet is complete and links to code, tests, and run
   commands for each critical mechanism.
2. Test vectors list valid and invalid behavior for the core protocol and
   wallet-review surfaces.
3. Long-running testnet validation has been collected for the candidate or a
   commit with documented equivalence.
4. The release notes and limitations state that there is no formal third-party
   security audit.
5. The release wording does not describe the candidate as audited, proven,
   production-ready, or risk-free.

This substitute is not a security guarantee. It is a review accessibility and
reproducibility standard.

## No-Go Conditions For RC Entry

The candidate should not be presented as a Mainnet RC if any of these are true:

- consensus parameters changed without corresponding public parameter review;
- focused tests for replay protection, expiry, REAP, expiry index, coinbase
  accounting, or wallet renewal fail without documented exclusion;
- review cards point to missing or stale code paths;
- test vectors do not include both valid and invalid cases for the core review
  areas;
- long-running testnet validation is absent and no replacement observation
  evidence is documented;
- release notes imply a formal third-party audit exists when it does not;
- public issue intake cannot capture reproducible review findings.

## Release Wording Requirement

Every RC release note or review packet must include wording equivalent to:

```text
This candidate has public review materials, test vectors, reproducibility
commands, and testnet validation evidence. It has not completed a formal
third-party security audit.
```
