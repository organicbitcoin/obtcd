# OBTC Parameter Consistency

This repository keeps a lightweight parameter consistency check for OBTC
mainnet-candidate review.

## What It Checks

Run:

```bash
python3 scripts/check_obtc_parameters.py
```

or:

```bash
make check-obtc-parameters
```

The checker reads
`config/obtc-mainnet-candidate-parameters.json`, extracts the listed values
from code, then checks explicitly listed Markdown and HTML references.

The manifest covers:

- consensus-critical activation and lifecycle parameters: fork height, replay
  protection height, expiry enable height, expiry commitment height, REAP
  hardening height, expiry window, REAP tax/refund ratio, dust threshold, input
  caps, and weight budget;
- network namespace parameters: network magic, P2P/RPC ports, Bech32 HRP,
  P2PKH/P2SH/WIF/witness prefixes, HD key versions, BIP44 coin type, and DNS
  seed name;
- wallet policy defaults: wallet legacy RPC ports and wallet-facing coin type
  references.

The command can run from a clean `obtcd` checkout. If sibling workspaces exist
next to this repo, it also checks optional references in:

- `../obtcwallet`
- `../obtc-website`
- `../obtc-control-plane`

Missing optional sibling repositories are reported as skipped and do not fail the
default check.

## Output

The default output is a Markdown report with:

- a status count summary;
- a parameter table;
- findings for required mismatches, missing references, ambiguous references, or
  extractor errors;
- all checked code and documentation references.

Statuses mean:

- `exact match`: the extracted value matches the manifest value or an explicit
  alias.
- `missing`: the required file, parameter, or document reference was not found.
- `mismatch`: an extracted value disagrees with the manifest.
- `ambiguous`: both expected and unexpected values were found in the same
  reference.
- `skipped`: an optional sibling repo or optional file was not present.
- `warning`: only optional cross-repo references had non-clean results.

JSON output is available for local tooling:

```bash
python3 scripts/check_obtc_parameters.py --format json
```

Use `--strict-optional` when a release workspace should fail on optional
cross-repo mismatches too:

```bash
python3 scripts/check_obtc_parameters.py --strict-optional
```

## Failure Policy

The checker returns non-zero for required `missing`, `mismatch`, `ambiguous`, or
extractor `error` results. It does not rewrite code or docs and does not choose
which side is correct.

When a mismatch appears:

1. Treat it as requiring human confirmation.
2. Identify whether the code, manifest, or document reference is stale.
3. Update the stale item in a separate PR after review.
4. Re-run this checker and the relevant unit/integration tests.

## Parameter Ownership

Consensus-critical:

- fork height;
- replay protection activation height;
- expiry enable height;
- expiry commitment activation height;
- REAP consensus/hardening height;
- `WindowBlocks`;
- tax/refund ratio;
- dust threshold;
- REAP input caps;
- REAP max weight.

Network namespace:

- wire magic;
- P2P port;
- node RPC port;
- Bech32 HRP;
- P2PKH/P2SH/WIF/witness prefixes;
- HD key versions;
- DNS seed name.

Wallet policy defaults:

- wallet legacy RPC ports;
- wallet-facing BIP44 coin type references.

## CI

The GitHub Actions `OBTC Integration Tests` job runs:

```bash
make check-obtc-parameters
```

CI only has the `obtcd` checkout, so optional sibling-repo references are skipped
there. Full release workspaces should run the same command locally with sibling
repos present, and use `--strict-optional` before cutting release artifacts.
