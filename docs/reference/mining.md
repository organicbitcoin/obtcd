# Mining

btcd supports the `getblocktemplate` RPC.
The limited user cannot access this RPC.

## Add the payment addresses with the `miningaddr` option

```bash
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
miningaddr=12c6DSiU4Rq3P4ZxziKxzrL5LmMBrzjrJX
miningaddr=1M83ju3EChKYyysmM2FXtLNftbacagd8FR
```

For OBTC networks, use an address for the selected OBTC network, for example an
`obtc1...` mainnet address or an `obtct1...` testnet address.

## Add btcd's RPC TLS certificate to system Certificate Authority list

`cgminer` uses [curl](http://curl.haxx.se/) to fetch data from the RPC server.
Since curl validates the certificate by default, we must install the `btcd` RPC
certificate into the default system Certificate Authority list.

## Ubuntu

1. Copy rpc.cert to /usr/share/ca-certificates: `cp /home/user/.btcd/rpc.cert /usr/share/ca-certificates/btcd.crt`
2. Add btcd.crt to /etc/ca-certificates.conf: `echo btcd.crt >> /etc/ca-certificates.conf`
3. Update the CA certificate list: `update-ca-certificates`

## Set your mining software url to use https

For OBTC testnet:

```bash
cgminer -o https://127.0.0.1:19528 -u rpcuser -p rpcpassword
```

For OBTC mainnet-candidate, use port `9528` unless the node was started with a
custom `--rpclisten`.
