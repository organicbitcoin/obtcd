---
name: Bug report
about: Report a reproducible OBTC node, testnet, REAP, mining, or RPC problem.
title: '[bug]: '
labels: ["bug"]
assignees: ''
---

## Safety check

Do not continue with a public report if this issue contains exploit details,
private keys, seed phrases, passphrases, RPC credentials, private peer
addresses, or unredacted sensitive logs. Use the repository's private
vulnerability reporting link instead.

## Summary

Describe the problem and its impact.

## Environment

- `obtcd` commit hash:
- network (`obtcmainnet` mainnet-candidate, `obtctestnet`, `obtcregtest`, isolated devnet, or none for local tests):
- operating system and architecture:
- `go version`:
- enabled indexes and relevant flags, with credentials removed:

## Steps to reproduce

List the smallest reliable sequence of commands or actions. Use only testnet or
regtest funds in public reproductions.

## Expected result

What should have happened?

## Actual result

What happened instead?

## Redacted evidence

Include relevant heights, txids, RPC responses, stack traces, or logs. Remove
credentials, wallet material, private addresses, hostnames, usernames, and
local filesystem paths that reveal personal information.
