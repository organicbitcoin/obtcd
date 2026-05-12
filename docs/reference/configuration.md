# Configuration

btcd has a number of configuration options, which can be viewed by running
`btcd --help`.  OBTC networks are selected with `--obtcmainnet`,
`--obtctestnet`, or `--obtcregtest`.

## Peer server listen interface

btcd allows you to bind to specific interfaces which enables you to setup
configurations with varying levels of complexity.  The listen parameter can be
specified on the command line as shown below with the -- prefix or in the
configuration file without the -- prefix (as can all long command line options).
The configuration file takes one entry per line.

**NOTE:** The listen flag can be specified multiple times to listen on multiple
interfaces as a couple of the examples below illustrate.

Command Line Examples:

|Flags|Comment|
|----------|------------|
|--listen=|all interfaces on the active network default P2P port|
|--listen=0.0.0.0|all IPv4 interfaces on the active network default P2P port|
|--listen=::|all IPv6 interfaces on the active network default P2P port|
|--listen=:19527|all interfaces on OBTC testnet P2P port 19527|
|--listen=0.0.0.0:19527|all IPv4 interfaces on OBTC testnet P2P port 19527|
|--listen=[::]:19527|all IPv6 interfaces on OBTC testnet P2P port 19527|
|--listen=127.0.0.1:19527|only IPv4 localhost on OBTC testnet P2P port 19527|
|--listen=[::1]:19527|only IPv6 localhost on OBTC testnet P2P port 19527|
|--listen=:9527|all interfaces on OBTC mainnet P2P port 9527|
|--listen=127.0.0.1:29527|only IPv4 localhost on OBTC regtest P2P port 29527|

The following config file would configure btcd to only listen on localhost for both IPv4 and IPv6:

```text
[Application Options]

listen=127.0.0.1:19527
listen=[::1]:19527
```

In addition, if you are starting btcd with TLS and want to make it
available via a hostname, then you will need to generate the TLS
certificates for that host. For example,

```
gencerts --host=myhostname.example.com --directory=/home/me/.btcd/
```

## RPC server listen interface

btcd allows you to bind the RPC server to specific interfaces which enables you
to setup configurations with varying levels of complexity.  The `rpclisten`
parameter can be specified on the command line as shown below with the -- prefix
or in the configuration file without the -- prefix (as can all long command line
options).  The configuration file takes one entry per line.

A few things to note regarding the RPC server:

* The RPC server will **not** be enabled unless the `rpcuser` and `rpcpass`
  options are specified.
* When the `rpcuser` and `rpcpass` and/or `rpclimituser` and `rpclimitpass`
  options are specified, the RPC server will only listen on localhost IPv4 and
  IPv6 interfaces by default.  You will need to override the RPC listen
  interfaces to include external interfaces if you want to connect from a remote
  machine.
* The RPC server has TLS enabled by default, even for localhost.  You may use
  the `--notls` option to disable it, but only when all listeners are on
  localhost interfaces.
* The `--rpclisten` flag can be specified multiple times to listen on multiple
  interfaces as a couple of the examples below illustrate.
* The RPC server is disabled by default when using the `--regtest`,
  `--obtcregtest`, and `--simnet` networks.  You can override this by
  specifying listen interfaces.

Command Line Examples:

|Flags|Comment|
|----------|------------|
|--rpclisten=|all interfaces on the active network default RPC port|
|--rpclisten=0.0.0.0|all IPv4 interfaces on the active network default RPC port|
|--rpclisten=::|all IPv6 interfaces on the active network default RPC port|
|--rpclisten=:19528|all interfaces on OBTC testnet RPC port 19528|
|--rpclisten=127.0.0.1:19528|only IPv4 localhost on OBTC testnet RPC port 19528|
|--rpclisten=[::1]:19528|only IPv6 localhost on OBTC testnet RPC port 19528|
|--rpclisten=:9528|all interfaces on OBTC mainnet RPC port 9528|
|--rpclisten=127.0.0.1:29528|only IPv4 localhost on OBTC regtest RPC port 29528|

The following config file would configure the btcd RPC server to listen to all interfaces on the default port, including external interfaces, for both IPv4 and IPv6:

```text
[Application Options]

rpclisten=
```

## Default ports

While btcd is highly configurable when it comes to the network configuration,
the following is intended to be a quick reference for the default ports used so
port forwarding can be configured as required.

btcd provides a `--upnp` flag which can be used to automatically map the
peer-to-peer listening port if your router supports UPnP.  If your router does
not support UPnP, or you don't wish to use it, please note that only the
peer-to-peer port should be forwarded unless you specifically want to allow RPC
access to your btcd from external sources such as in more advanced network
configurations.

|Network|P2P Port|Node RPC Port|
|----|----:|----:|
|OBTC mainnet|TCP 9527|TCP 9528|
|OBTC testnet|TCP 19527|TCP 19528|
|OBTC regtest|TCP 29527|TCP 29528|
|Bitcoin mainnet|TCP 8333|TCP 8334|

## Using bootstrap.dat

No OBTC bootstrap archive is published by this repository.  New OBTC nodes
should sync from peers using the network flag and seed/addpeer policy in the
current release runbook.  Do not import a Bitcoin bootstrap file into an OBTC
node data directory.
