# DevNet Traffic Simulator Guide

This guide covers the upgraded OBTC DevNet traffic simulator built around:

- `scripts/devnet-up.sh`
- `cmd/devnetsim`
- `scripts/validation/devnet_sim_smoke.sh`

The devnet now runs as a **2-node OBTC regtest devnet** by default (`obtcregtest`), not a plain Bitcoin simnet. That change is intentional: it allows every existing traffic scenario to validate **OBTC-specific logic** as well, including:

- expiry index RPCs
- expiry commitment activation
- REAP planning
- REAP block inclusion
- OBTC replay-protected sighash enforcement

## What It Can Simulate

The devnet supports:

- empty blocks
- sparse traffic
- dense traffic
- mempool backlog pressure
- fee market traffic
- double-spend conflict attempts
- multi-input consolidation transactions
- dependent transaction chains
- multi-wallet / multi-node traffic
- restart persistence checks
- continuous multi-block traffic
- dynamic mixed traffic runs

And on top of those traffic patterns, every scenario now validates OBTC-specific state.

## Quick Start

```bash
# Build the main node binary once.
go build -o btcd

# Start the 2-node OBTC devnet.
./scripts/devnet-up.sh start

# Show node, wallet, mempool, and OBTC-specific status.
./scripts/devnet-up.sh status

# Prepare both wallets.
./scripts/devnet-up.sh prepare 512 300000
./scripts/devnet-up.sh prepare-peer 256 240000

# Inject traffic from both nodes.
./scripts/devnet-up.sh spam --count 200 --mode feemarket --value 150000
./scripts/devnet-up.sh spam-peer --count 80 --mode mixed --value 110000

# Mine one block.
./scripts/devnet-up.sh mine 1

# Validate OBTC-specific state explicitly.
./scripts/devnet-up.sh validate-obtc

# Stop / resume.
./scripts/devnet-up.sh stop
./scripts/devnet-up.sh restart
```

If you want the shortest end-to-end run of the new behavior:

```bash
go build -o btcd
./scripts/devnet-up.sh start
./scripts/devnet-up.sh scenario multisource
./scripts/devnet-up.sh stop
```

## Core Commands

### Lifecycle

```bash
./scripts/devnet-up.sh start
./scripts/devnet-up.sh restart
./scripts/devnet-up.sh stop
./scripts/devnet-up.sh status
./scripts/devnet-up.sh logs
./scripts/devnet-up.sh clean
```

### Mining + Validation

```bash
./scripts/devnet-up.sh mine 1
./scripts/devnet-up.sh mine 10
./scripts/devnet-up.sh miner on
./scripts/devnet-up.sh miner off
./scripts/devnet-up.sh validate-obtc
```

### Wallet Preparation

```bash
./scripts/devnet-up.sh prepare 4000 300000
./scripts/devnet-up.sh prepare-peer 512 240000
```

`prepare-peer` funds a second deterministic wallet from the primary wallet, confirms the funding transaction, and leaves the peer wallet ready to spend through Node 2.

### Traffic Injection

```bash
./scripts/devnet-up.sh spam --count 500 --mode mixed --value 150000
./scripts/devnet-up.sh spam --count 800 --mode feemarket --value 150000 --pace-ms 10
./scripts/devnet-up.sh spam --count 60 --mode conflict
./scripts/devnet-up.sh spam --count 40 --mode consolidate
./scripts/devnet-up.sh spam-peer --count 120 --mode mixed --value 110000
```

## Traffic Modes

- `simple`: independent single-output payments
- `mixed`: a more realistic wallet mix of sends, splits, and self-transfers
- `chain`: dependent descendant chains using unconfirmed outputs
- `consolidate`: heavier multi-input consolidation transactions
- `feemarket`: fee-banded traffic across multiple fee tiers
- `conflict`: valid spend + conflicting double-spend attempt

## Canned Scenarios

```bash
./scripts/devnet-up.sh scenario empty
./scripts/devnet-up.sh scenario sparse
./scripts/devnet-up.sh scenario dense
./scripts/devnet-up.sh scenario backlog
./scripts/devnet-up.sh scenario feemarket
./scripts/devnet-up.sh scenario conflict
./scripts/devnet-up.sh scenario consolidation
./scripts/devnet-up.sh scenario multisource
./scripts/devnet-up.sh scenario steady
./scripts/devnet-up.sh scenario dynamic
```

### Scenario Summary

- `empty`: mine an empty block while still validating OBTC block state
- `sparse`: a few ordinary transactions then mine
- `dense`: many independent transactions then mine
- `backlog`: overfill mempool and leave backlog after one block
- `feemarket`: build a fee-layered mempool then mine
- `conflict`: create valid + conflicting double-spend attempts
- `consolidation`: create heavier multi-input transactions
- `multisource`: drive two independent wallets through both nodes
- `steady`: feed paced traffic across multiple blocks
- `dynamic`: execute several traffic classes in sequence

## OBTC-Specific Validation Model

The devnet bootstraps itself to a height where OBTC-specific logic is active before the scenarios begin.

That means every scenario runs with:

- `expiryindex` enabled
- expiry commitment active
- REAP planning active
- replay-protected sighash enforcement active

The validation layer checks:

- `getexpiryindexstats`
- `getexpirycommitment`
- `getreapplan`
- the latest mined block contains a REAP marker transaction

## Smoke / Integration Validation

Run:

```bash
./scripts/validation/devnet_sim_smoke.sh
```

The smoke flow verifies:

1. OBTC devnet startup
2. explicit OBTC RPC validation
3. primary wallet UTXO preparation
4. peer wallet funding and readiness
5. multi-source mempool admission on both nodes
6. conflict rejection accounting
7. presence of consolidation transactions
8. mining a non-empty block that also contains a REAP marker transaction
9. stop/resume persistence
10. continued peer spending after restart

Example success tail:

```text
[smoke] PASS mempool_size=20 node2_mempool_size=20 mined_block_txs=28 reap_picked=1
```

## Important Implementation Note

During development I found that **locally RPC-injected transactions did not reliably converge into a shared mempool through native relay alone** in this 2-node local setup.

To keep multi-source traffic realistic and stable enough for miner-side validation, `devnetsim` now supports mirrored broadcast to the opposite node RPC endpoint.

That is a devnet engineering workaround, not a claim that the local setup fully models production relay behavior.

## Another Important Note: Replay Protection

When OBTC replay-protection becomes active, locally generated legacy spends must use:

- `SigHashOBTCReplayProtection | SigHashAll`

The upgraded simulator now switches to the replay-protected sighash automatically once the OBTC activation height has passed.

## Current Limitations

The upgraded devnet is substantially stronger, but it still does **not** fully model production behavior.

Notable gaps include:

- no peer partition / disconnect / reconnect simulation yet
- no packet delay / packet loss / adversarial topology model yet
- no larger population model with many independent wallets and fee preferences
- no deeper package relay / ancestor policy stress matrix yet
- native relay convergence for locally injected RPC traffic still relies on mirrored broadcast in this local setup

These are the next high-value realism upgrades.
