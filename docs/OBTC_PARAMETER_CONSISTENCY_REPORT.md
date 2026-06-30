# OBTC Parameter Consistency Report

Generated from local workspace scan on 2026-06-30.

Command:

```bash
python3 scripts/check_obtc_parameters.py --format json
```

## Summary

| Status | Parameter count |
|---|---:|
| exact match | 68 |
| missing | 0 |
| mismatch | 0 |
| ambiguous | 0 |
| warning | 0 |

The scan checked 196 code and documentation references from the manifest.

## Checked Paths

| Repository | Path |
|---|---|
| `obtcd` | `chaincfg/params_obtc.go` |
| `obtcd` | `wire/protocol.go` |
| `obtcd` | `params.go` |
| `obtcd` | `README.md` |
| `obtcd` | `docs/mainnet-params.md` |
| `obtcd` | `docs/mainnet-join.md` |
| `obtcd` | `docs/mainnet-dns-seed.md` |
| `obtcd` | `docs/network-parameters.md` |
| `obtcd` | `docs/testnet-join.md` |
| `obtcwallet` | `netparams/params.go` |
| `obtcwallet` | `README.md` |
| `obtcwallet` | `docs/mainnet-readiness.md` |
| `obtcwallet` | `docs/releases/obtcwallet-testnet-v0.1.0.md` |
| `obtc-website` | `whitepaper.md` |
| `obtc-website` | `content/whitepaper-v1.md` |
| `obtc-website` | `mainnet-candidate.html` |
| `obtc-website` | `docs.html` |
| `obtc-website` | `versions.html` |
| `obtc-control-plane` | `docs/release/mainnet_candidate_release_notes.md` |

## Current Findings

No required or optional mismatches were detected by the manifest-backed scan.

## Manual Confirmation Required

The scan confirms textual and code consistency for listed parameter references
only. It does not prove operational readiness. These items still require human
release confirmation outside this checker:

- final bootstrap policy and live DNS/peer evidence;
- fresh-node sync evidence;
- public 72h observation evidence;
- release artifacts, checksums, and signatures or signed manifest evidence;
- funded wallet evidence appropriate to the release scope;
- final review that optional sibling repositories are present and checked with
  `--strict-optional` before release publication.
