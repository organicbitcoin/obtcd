# OBTC Mainnet DNS Seed Design

This document records the DNS layout intended for the mainnet-candidate seed
infrastructure. The `obtcd` code uses the aggregate seed name only; individual
node names are for operations, release notes, and explicit peer bootstrapping.

## Names

| Name | Purpose |
|---|---|
| `seed.mainnet.organicbitcoin.org` | Aggregate DNS seed used by `obtcd` |
| `seed1.mainnet.organicbitcoin.org` | Individual fallback seed node |
| `seed2.mainnet.organicbitcoin.org` | Individual fallback seed node |
| `seed3.mainnet.organicbitcoin.org` | Individual fallback seed node |

`obtcd` does not consume SRV records for peer discovery. The seed names must
resolve to A and, where available, AAAA records. Peers connect to the network
default P2P port `9527`.

## AWS Route 53 Layout

Use the public hosted zone for `organicbitcoin.org`.

Recommended records:

| Record | Type | Routing | TTL | Target |
|---|---|---|---|---|
| `seed.mainnet.organicbitcoin.org` | A/AAAA | Multivalue or weighted | `300` | Healthy seed node addresses |
| `seed1.mainnet.organicbitcoin.org` | A/AAAA | Simple | `300` | Seed node 1 Elastic IP / IPv6 |
| `seed2.mainnet.organicbitcoin.org` | A/AAAA | Simple | `300` | Seed node 2 Elastic IP / IPv6 |
| `seed3.mainnet.organicbitcoin.org` | A/AAAA | Simple | `300` | Seed node 3 Elastic IP / IPv6 |

Place nodes in at least three independent AWS Availability Zones, and preferably
more than one AWS region. The exact region list is an operator decision and
needs human confirmation before release.

## Node Exposure

Seed nodes should expose only the public P2P surface:

| Port | Exposure |
|---|---|
| TCP `9527` | Public inbound from the Internet |
| TCP `9528` | Private only; bind to localhost, VPC, VPN, or SSM-managed host |
| Wallet RPC ports | Do not publish through seed DNS |

Use AWS security groups or host firewall rules so the aggregate seed record
cannot become an RPC endpoint by mistake.

## Candidate Release Checklist

* Confirm `seed.mainnet.organicbitcoin.org` resolves to live P2P nodes.
* Confirm each `seedN.mainnet.organicbitcoin.org` resolves to one node.
* Confirm fresh bootstrap succeeds using DNS seed discovery.
* Confirm fresh bootstrap succeeds using explicit fallback peers:
  `--addpeer=seed1.mainnet.organicbitcoin.org:9527`,
  `--addpeer=seed2.mainnet.organicbitcoin.org:9527`, and
  `--addpeer=seed3.mainnet.organicbitcoin.org:9527`.
* Capture DNS answers, node health, peer count, tip height, and expiry index
  state as release evidence.
* Document the final region/provider layout in release notes.
