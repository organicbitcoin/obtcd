# OBTC Mining Review Checklist

This checklist is for miners, pool implementers, Stratum bridge authors, and
protocol reviewers who want to check whether OBTC's mining-facing documentation
and block-template assumptions are clear.

Scope: ordinary SHA256d proof-of-work should remain standard. OBTC-specific
review should focus on block templates, coinbase/accounting, expiry and REAP
commitments, and validation rules.

Non-goals: this document does not request mining adoption, pool adoption,
firmware changes, Stratum protocol changes, or any revenue expectation.

## 1. Node and RPC baseline

Start the node with the correct network flag and private RPC binding:

- testnet: `--obtctestnet`, P2P `19527`, RPC `19528`;
- mainnet-candidate: `--obtcmainnet`, P2P `9527`, RPC `9528`;
- keep RPC on `127.0.0.1` unless it is behind strict access controls;
- use `--txindex` and `--expiryindex` when reviewing lifecycle state.

Confirm:

```bash
./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getblockchaininfo

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getmininginfo
```

## 2. `getblocktemplate`

Review questions:

- Does the response make clear whether the caller receives `coinbasevalue` or a
  full `coinbasetxn`?
- Are the returned `height`, `previousblockhash`, `bits`, `target`,
  `mintime`, `curtime`, and `mutable` fields consistent with the current chain
  tip?
- Does long polling update when the previous block changes or when the template
  becomes stale?
- Are template errors actionable for external pool software?
- Does documentation explain which network flag and RPC port were used?

Relevant code paths:

- `rpcserver.go` for `getblocktemplate` and `submitblock` RPC handling;
- `mining/mining.go` for template construction;
- `mining/template_reap.go` for OBTC REAP template integration;
- `mining/newblocktemplate_*_test.go` for mining-template regression tests.

## 3. Coinbase value and output handling

Review questions:

- If a pool constructs its own coinbase, does documentation explain how to use
  `coinbasevalue` and preserve required commitments?
- If a caller asks for `coinbasetxn`, does the configured mining address match
  the selected OBTC network?
- Does the template include the expected block subsidy, fees, and any
  protocol-defined REAP accounting at the reviewed height?
- Are extra coinbase outputs clearly bounded so they do not overwrite required
  commitment outputs?
- Do logs make coinbase/accounting errors visible enough to debug?

Relevant code paths:

- `mining/mining.go`;
- `mining/template_reap.go`;
- `mining/reap/`;
- `rpcserver.go`.

## 4. Expiry commitment and REAP commitment

Review questions:

- Does the node expose expiry-index state when started with `--expiryindex`?
- Does the block template include an expiry commitment once the commitment
  height requires it?
- Does the commitment root match the local expiry-index state?
- Are REAP candidates selected deterministically and with the documented input
  caps?
- Are refundless dust inputs and normal inputs capped separately?
- Is pre-activation behavior documented as not applicable rather than as a
  failed mining result?

Useful RPCs:

```bash
./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getexpiryindexstats

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getexpirycommitment

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getreapplan
```

Relevant code paths:

- `blockchain/expiryindex/`;
- `mining/reap/`;
- `mining/template_reap.go`;
- `chaincfg/params_obtc.go`.

## 5. Template mutation boundaries

Pool or gateway software should know what it may mutate and what it must
preserve.

Review questions:

- Can timestamp, version rolling, and nonce search proceed normally?
- Does any mutation change the merkle root without preserving required
  coinbase commitments?
- If the caller rebuilds coinbase outputs, does it preserve expiry/REAP
  commitment data required by consensus at that height?
- Are transaction selection or ordering changes valid under the current
  lifecycle and REAP rules?
- Are stale templates rejected clearly after a new previous block arrives?

## 6. Share difficulty versus block target

Review questions:

- Does pool software distinguish share target from network block target?
- Does the runbook explain that valid shares are pool-local accounting events,
  while `submitblock` requires a full valid block at or below the block target?
- Are weak shares never described as accepted OBTC blocks?
- Are rejection reasons captured with the template id, target, height, and
  previous block hash?

## 7. `submitblock`

Review questions:

- Does the submitted block preserve all OBTC-required commitments?
- Does the node return actionable errors for stale, invalid, or malformed
  blocks?
- Is the accepted block visible through `getblockchaininfo` and
  `getchaintips` after submission?
- Are logs clear enough to distinguish consensus rejection from RPC or
  transport failure?

Relevant code path:

- `rpcserver.go` for `handleSubmitBlock`.

## 8. Stratum v1 assumptions

OBTC does not require firmware changes for an ordinary SHA256d mining device if
the device talks to a pool or local gateway that serves valid OBTC work.

Review questions:

- Does documentation clearly state that end devices should receive normal
  SHA256d work from a pool/gateway?
- Does the pool/gateway know which OBTC node and network it is using?
- Are worker names, pool URLs, and RPC credentials examples separated from real
  secrets?
- Does the gateway rebuild or mutate the coinbase in a way that could remove
  required OBTC commitments?
- Are device logs and pool logs sufficient to debug connection, authorization,
  share rejection, and solved-block submission?

## 9. Stratum v2 mapping

Reference: <https://github.com/stratum-mining/sv2-spec>

This section maps OBTC review questions onto Stratum v2 concepts. It is not a
proposal to change Stratum v2.

### Inherited assumptions

OBTC inherits the ordinary proof-of-work search model: miners search over block
header fields such as `nonce`, `nTime`, and BIP320-compatible version bits, and
the merkle root is derived from the coinbase transaction and transaction set.

For standard jobs, the device receives header-only work with a fixed merkle
root. For extended jobs and translators, the server or proxy may split search
space through extranonce handling and coinbase prefix/suffix construction. The
review question for OBTC is whether the pool/gateway template source creates a
valid OBTC block before that work is distributed downstream.

### Review attention points

- **Template source:** does the upstream template come from an OBTC node with
  the correct network flag, activation state, and expiry-index configuration?
- **Coinbase construction:** if coinbase prefix/suffix or outputs are rebuilt,
  are expiry/REAP commitments and accounting preserved?
- **Coinbase output constraints:** if a template provider reserves coinbase
  output space, does it reserve enough for required OBTC commitments and pool
  payout outputs?
- **Job declaration:** if miner-selected templates are used, does the declared
  job include enough data for the pool or verifier to check OBTC-specific
  validity?
- **Template distribution:** if a template API replaces or abstracts
  `getblocktemplate`, does it carry the equivalent OBTC activation, target,
  coinbase value/output, and commitment requirements?
- **Share submission:** share acceptance is pool-local; a solved block still
  needs to reconstruct to a full OBTC block that passes node validation through
  `submitblock` or an equivalent propagation path.
- **Future jobs:** future or speculative jobs must not include stale
  lifecycle/REAP state after a new previous block changes the valid template.

### Not current OBTC goals

- changing Stratum v2 message semantics;
- asking Stratum v2 projects to add OBTC support;
- claiming compatibility with a specific pool, gateway, or firmware project;
- requiring mining devices to understand expiry or REAP directly;
- treating pool-local share accounting as protocol validation.

## 10. Logs and evidence to capture

For a useful mining review report, include:

- `git rev-parse --short HEAD`;
- node network flag, RPC endpoint, height, best hash, and peer count;
- the exact `getblocktemplate` request and redacted response excerpt;
- the mining address or payout script type, with private details redacted;
- whether the template used `coinbasevalue` or `coinbasetxn`;
- expiry commitment and REAP activation state at the reviewed height;
- pool/gateway software name and version, if applicable;
- Stratum mode used, if applicable;
- share rejection or solved-block logs;
- `submitblock` response and node logs around the submission.

Do not include private keys, seed phrases, wallet passphrases, RPC passwords, or
unredacted host credentials.
