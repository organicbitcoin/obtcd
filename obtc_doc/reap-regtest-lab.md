# OBTC Regtest REAP Lab

这份文档记录一套从 `0` 开始，在本地 `obtcregtest` 环境中挖出包含 REAP 交易区块的最短命令路径。

## 目标

- 编译 `obtcd` 和 `btcctl`
- 启动本地 `obtcregtest` 节点
- 挖到 REAP 激活并真正进块
- 查看区块内的 REAP 交易
- 查看新增的 REAP 调试日志

## 前置说明

- 当前仓库节点本身**没有集成钱包**。
- 所以下面的实验不依赖 `getnewaddress` / `sendtoaddress`，而是直接让旧的 coinbase UTXO 自然到期并被 REAP。
- 如果你要做真正的钱包收发实验，需要接支持 OBTC 网络参数的外部 wallet 进程。

## 1. 编译

```bash
go build -o ./obtcd .
go build -o ./cmd/btcctl/btcctl ./cmd/btcctl
```

## 2. 定义一个可复用的 btcctl 函数

`zsh` 里不要把整条命令塞进一个普通字符串变量后再直接执行，否则会把整串内容当成一个“文件名”。

推荐直接定义函数：

```bash
BTCCTL() {
  ./cmd/btcctl/btcctl \
    --obtcregtest \
    --notls \
    --rpcuser=test \
    --rpcpass=test \
    --rpcserver=127.0.0.1:29528 \
    "$@"
}
```

## 3. 启动节点

下面这个地址已经验证可用于本地实验：

- WIF: `coMYYsmzB6c7XATTJksgABoRs42nrDz8qsoe8TCTgx8rnUQUBy2D`
- Address: `obtcrt1q0xcqpzrky6eff2g52qdye53xkk9jxkvrcfe6tt`

启动命令：

```bash
./obtcd \
  --obtcregtest \
  --notls \
  --expiryindex \
  --rpcuser=test \
  --rpcpass=test \
  --miningaddr=obtcrt1q0xcqpzrky6eff2g52qdye53xkk9jxkvrcfe6tt \
  --debuglevel=CHAN=debug,INDX=debug,MINR=debug \
  --datadir=/tmp/obtc-lab/data \
  --logdir=/tmp/obtc-lab/logs \
  --nodnsseed \
  --nolisten
```

说明：

- `CHAN=debug`：看 REAP 共识校验日志
- `INDX=debug`：看 ExpiryIndex 扫描和连接日志
- `MINR=debug`：看 REAP 选择、蓝图构造和模板注入日志

## 4. 确认节点已启动

另开一个终端：

```bash
BTCCTL getblockcount
```

预期输出：

```text
0
```

## 5. 先挖到 REAP 即将出现的高度

```bash
BTCCTL generate 144
BTCCTL getblockcount
BTCCTL getreapplan
```

这时 `getreapplan` 应该会返回类似：

```json
{
  "height": 145,
  "enabled": true,
  "active": true,
  "picked": 1,
  "tax_total": 1500000000,
  "refund_total": 3500000000,
  "est_weight": 932
}
```

## 6. 再挖 1 个块，让 REAP 真正进块

```bash
BTCCTL generate 1
```

## 7. 查看包含 REAP 的区块

```bash
HASH=$(BTCCTL getblockhash 145)
BTCCTL getblock "$HASH" 2
```

预期现象：

- `rawtx` 里有 2 笔交易
- 第 1 笔是 coinbase
- 第 2 笔是 `version=3` 的 REAP 交易

## 8. 查看新增的 REAP 日志

```bash
tail -f /tmp/obtc-lab/logs/obtcregtest/btcd.log
```

也可以直接筛关键日志：

```bash
grep "REAP build" /tmp/obtc-lab/logs/obtcregtest/btcd.log
grep "template appended REAP" /tmp/obtc-lab/logs/obtcregtest/btcd.log
grep "template REAP tx structure" /tmp/obtc-lab/logs/obtcregtest/btcd.log
grep "REAP marker check ok" /tmp/obtc-lab/logs/obtcregtest/btcd.log
```

你现在最值得看的那条结构日志会长这样：

```text
MINR: template REAP tx structure height=145 txid=... version=3 locktime=144 inputs=[...] outputs=[...]
```

它会包含：

- `txid`
- `version`
- `locktime`
- 每个输入的 `prevout` 和 `sequence`
- 每个输出的 `value`
- 普通退款输出脚本
- REAP marker 输出的 `payload`

## 9. 实验完成后清理

```bash
rm -rf /tmp/obtc-lab
```

## 10. 本地验证记录

这套命令已经本地验证通过，结果包括：

- 在高度 `145` 出现 REAP 交易
- REAP 交易 `version=3`
- 50 BTC 的过期 coinbase 被处理为：
  - `refund_total = 3500000000`
  - `tax_total = 1500000000`
- 对应区块 coinbase 实际收入变成 `65 BTC`
