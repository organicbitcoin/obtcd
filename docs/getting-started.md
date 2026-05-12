# Getting Started

Build `obtcd` from this repository.  The executable is still named `btcd`, but
OBTC networks are selected explicitly with `--obtcmainnet`, `--obtctestnet`, or
`--obtcregtest`.

## Requirements

[Go](http://golang.org) 1.22 or newer.

## Build From Source

* Install Go according to the [installation instructions](https://go.dev/doc/install).
* Ensure Go was installed properly and is a supported version:

```bash
go version
go env GOROOT GOPATH
```

* Clone and build:

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
go build ./...
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
```

To install into your Go binary directory instead of keeping local build
artifacts:

```bash
go install -v . ./cmd/...
```

## Startup

Start with an explicit OBTC network flag.  For the public test network:

```bash
./btcd --obtctestnet \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --rpcuser=<user> \
  --rpcpass=<pass>
```

Mainnet-candidate releases should use the release notes and mainnet runbook for
seed/addpeer policy before exposing a public node.
