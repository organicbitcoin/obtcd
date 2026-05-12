# Using Docker

This repository can build a local Docker image for the OBTC node.  There is no
published container image for OBTC mainnet-candidate releases in this
repository; build from the checked-out source you intend to run.

## Build Image

From the repository root:

```bash
docker build . -t obtcd:local
```

For arm64:

```bash
docker build . -t obtcd:local --build-arg ARCH=arm64v8
```

## Data Volume

The container stores node data under `/root/.btcd`.  Use a named volume or bind
mount so chain data survives container replacement.

```bash
docker volume create obtcd-data
```

## OBTC Testnet Example

This example exposes the OBTC testnet P2P port and keeps the node RPC listener
on localhost inside the container.  Do not expose RPC publicly.

```yaml
services:
  obtcd:
    image: obtcd:local
    container_name: obtcd
    hostname: obtcd
    restart: unless-stopped
    volumes:
      - obtcd-data:/root/.btcd
    ports:
      - "19527:19527"
    command:
      - "--obtctestnet"
      - "--listen=0.0.0.0:19527"
      - "--rpclisten=127.0.0.1:19528"
      - "--rpcuser=${OBTCD_RPC_USER}"
      - "--rpcpass=${OBTCD_RPC_PASS}"

volumes:
  obtcd-data:
```

Run it with:

```bash
OBTCD_RPC_USER=user OBTCD_RPC_PASS='change-this' docker compose up -d
```

## OBTC Mainnet-Candidate Ports

When using `--obtcmainnet`, the default P2P port is `9527` and the default node
RPC port is `9528`.  Mainnet-candidate nodes should follow the release runbook
for seed or `addpeer` policy before being exposed.

## Logs and Shutdown

```bash
docker logs -f obtcd
docker compose down
```
