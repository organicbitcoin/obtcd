# AGENTS.md

This document describes the recommended directory structure and module
boundaries for the OBTC repository.

## Structure Overview

```
obtcd/ (repo root)
  blockchain/
    expiryindex/       # Week 2: expiry index
    validation_reap.go # Week 4+: REAP consensus validation
  chaincfg/
    params_obtc.go     # OBTC network parameters
  mining/
    reap/              # Week 3: selector and blueprint construction
    template_reap.go   # Week 4: template injection
  mempool/
    policy.go          # Week 4: REAP policy limits
  rpc/
    rpcserver.go       # RPC entry point
  cmd/
    gengenesis/        # Week 6/8: genesis generator
    checkgenesis/      # Week 6/8: genesis validator
    obtc-status/       # Week 6: minimal status page
  scripts/
    devnet-up.sh       # Week 1: devnet bootstrap script
    validation/        # Week 2: validation scripts and tools
  docs/
    phase1-validation.md
    phase2-summary.md
    phase2-validation.md
    phase3-validation.md
    phase4-validation.md
    testnet-join.md
    mainnet-join.md
  obtc_doc/
    AGENTS.md
    obtc_roadmap_plan.md
    newcomer_reading_guide.md
    expiry_commitment_implementation.md
    reap_dust_behavior.md
    phase5_implementation.md
    phase6_implementation.md
    phase7_implementation.md
    phase8_implementation.md
```

## Notes

- The layout above is a recommended placement guide and mixes implemented and planned work.
- If the live repository layout differs, treat the repository state as the source of truth and update this file when needed.
- Keep new modules grouped by responsibility instead of adding unrelated files at the repository root.

## Interaction Constraints

- Match the user's language for interactive discussion.
- Use English for commit messages.
- Do not use `--no-verify` to bypass `pre-commit` or `pre-push`.
- If a hook fails, fix the issue or report the blocker instead of bypassing validation.
