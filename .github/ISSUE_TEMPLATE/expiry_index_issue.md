---
name: Expiry index issue
about: Report expiry-index, commitment, scan, rebuild, or reorg behavior.
title: "[expiry-index]: "
labels: ["bug", "expiry index", "needs triage"]
assignees: ""
---

## Summary

Describe the expiry-index behavior that looks wrong.

## Area

- [ ] Expiry key calculation
- [ ] Connect block
- [ ] Disconnect block / reorg
- [ ] Rebuild / reindex
- [ ] Expiry commitment
- [ ] REAP prefix scan
- [ ] RPC or observability

## Environment

- `obtcd` commit or tag:
- Network flag:
- Node height:
- Best block hash:
- `--expiryindex` enabled: yes / no
- Reindex used: yes / no
- OS / architecture:
- Go version:

## Reproduction

1.
2.
3.

Command or RPC request:

```text

```

## Expected Result

What expiry state, commitment root, or scan output did you expect?

## Actual Result

What happened instead?

## Evidence

- Block hash before / after:
- Reorg depth, if any:
- OutPoint and expiry height:
- Commitment root:
- `getexpiryindexstats` output:
- Redacted logs:

## Safety Check

- [ ] I removed private paths, RPC credentials, private IPs, and wallet secrets.
