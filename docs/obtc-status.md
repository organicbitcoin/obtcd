# obtc-status

`obtc-status` 是一个只读 HTTP 状态页工具。它通过现有的 `btcd` JSON-RPC
读取链高、peer、mempool、`ExpiryIndex`、expiry commitment 和 `REAP`
dry-run 概况，不直接修改节点状态。

## 构建

```bash
go build ./cmd/obtc-status
```

## 运行

```bash
./cmd/obtc-status/obtc-status \
  --obtctestnet \
  --rpcuser=<user> \
  --rpcpass=<pass> \
  --rpcserver=127.0.0.1:18556 \
  --notls
```

`18556` 是当前 `obtctestnet` 的默认 RPC 端口；如果节点启动时显式改成了其他端口
（例如 [testnet-join.md](testnet-join.md) 里的 `19528`），这里也要改成对应值。

默认会在 `127.0.0.1:9680` 提供以下端点：

- `/`：HTML 状态页
- `/status`：JSON 快照
- `/healthz`：探活检查

## 可选项

- `--listen`：HTTP 监听地址
- `--refresh`：HTML 自动刷新间隔
- `--rpctimeout`：单次上游 RPC 超时
- `--rpccert`：上游 `btcd` 的 RPC 证书
- `--skipverify`：跳过 TLS 校验，仅适合受控环境

## 说明

- `getexpiryindexstats`、`getexpirycommitment`、`getreapplan` 失败时，页面仍会返回基础链状态，并在 `warnings` 中标记失败。
- 若节点未开启 `--expiryindex`，`ExpiryIndex` 相关观测通常会显示为 disabled 或 warning。
