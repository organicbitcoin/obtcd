# OBTC Security Policy

## Scope

This policy applies to the OBTC node implementation in this repository,
including consensus validation, network parameters, expiry indexing, REAP,
mining template behavior, RPC surfaces, release artifacts, and operator
documentation.

OBTC is experimental software. It is not production financial infrastructure,
and no public claim tooling should be treated as safe for real funds unless a
separate reviewed release says so explicitly.

## Supported Material

Security review should target the current default branch and any tagged OBTC
testnet or mainnet-candidate release artifacts that are explicitly published by
the project.

Old draft branches, archived parameter sets, and historical experiments are not
treated as supported versions unless a current project page links them as active
review targets.

## Reporting Sensitive Issues

Do not open a public issue with exploit details, private keys, seed phrases,
wallet backups, real-fund claim attempts, unpublished peer addresses, private
logs, or other sensitive material.

For sensitive reports, use GitHub private vulnerability reporting for this
repository if it is available. If private reporting is not available, open a
minimal public issue asking for a secure reporting channel and include no exploit
details or secrets.

Public issues are appropriate for non-sensitive bugs, documentation gaps,
reproducible testnet-only failures, and questions that do not reveal private
material.

## Private-Key And Claim Safety

Never import a Bitcoin seed phrase or Bitcoin private key into experimental OBTC
software, a website, a support form, a public issue, or a debugging transcript.

Do not send real-fund claim attempts, real wallet material, or recovery phrases
to maintainers. Final claim tooling, if published, must provide a safer reviewed
flow.

## What To Include

For non-sensitive technical reports, include:

- affected repository and commit hash;
- network (`obtc mainnet-candidate`, `obtctestnet`, or `obtcregtest`);
- commands needed to reproduce;
- expected behavior and observed behavior;
- relevant block heights, txids, and logs with secrets removed.

For consensus or REAP reports, include the smallest reproducible chain state or
test case you can provide without exposing private material.
