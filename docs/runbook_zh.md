# obtcd 运维手册 (Runbook)

## 目录

1. [服务概述](#1-服务概述)
2. [环境要求与安装](#2-环境要求与安装)
3. [配置说明](#3-配置说明)
4. [启动与停止](#4-启动与停止)
5. [网络与端口](#5-网络与端口)
6. [日志管理](#6-日志管理)
7. [数据库与存储](#7-数据库与存储)
8. [RPC 接口](#8-rpc-接口)
9. [P2P 网络管理](#9-p2p-网络管理)
10. [挖矿配置](#10-挖矿配置)
11. [OBTC 特有功能](#11-obtc-特有功能)
12. [辅助工具](#12-辅助工具)
13. [健康检查与监控](#13-健康检查与监控)
14. [常见故障排查](#14-常见故障排查)
15. [灾难恢复](#15-灾难恢复)
16. [安全注意事项](#16-安全注意事项)
17. [DevNet 开发环境](#17-devnet-开发环境)

---

## 1. 服务概述

obtcd 是 **Organic Bitcoin (OBTC)** 协议的全节点实现，基于 btcd 硬分叉而来。

### 1.1 核心特性

- **REAP 协议**（Reclaim Expired Assets Protocol，到期资产回收协议）：引入 UTXO 时间衰减机制
  - UTXO 在 7 年后到期
  - 到期价值的 30% 重新分配给矿工
  - 交易重放保护
  - 到期承诺（Expiry Commitment）支持
- **网络隔离**：与比特币网络完全隔离，使用独立的端口、地址格式和 Magic Number
- **基于 btcd**：继承经过生产验证的稳定代码库
- **分叉高度**：主网约 950,000（预计 2026 年 Q2），与比特币共享分叉前的全部历史

### 1.2 关键代码路径

| 模块 | 路径 | 说明 |
|------|------|------|
| 主入口 | `btcd.go` | 守护进程入口 |
| 配置解析 | `config.go` | 命令行与配置文件解析 |
| 区块链核心 | `blockchain/` | 共识规则、区块验证 |
| REAP 验证 | `blockchain/validation_reap.go` | REAP 协议验证逻辑 |
| ExpiryIndex | `blockchain/expiryindex/` | UTXO 到期索引 |
| 到期承诺 | `blockchain/expiryindex/commitment.go` | Coinbase 到期承诺 |
| 挖矿 | `mining/` | 区块模板生成 |
| REAP 选择器 | `mining/reap/` | 到期 UTXO 选取与交易构建 |
| 内存池 | `mempool/` | 交易池管理 |
| P2P 网络 | `peer/`, `connmgr/` | 对等节点管理 |
| 链参数 | `chaincfg/` | 网络参数定义 |
| OBTC 参数 | `chaincfg/params_obtc.go` | OBTC 专用网络参数 |
| Wire 协议 | `wire/` | 消息序列化与网络编码 |
| 脚本引擎 | `txscript/` | 脚本验证与重放保护 |
| RPC 服务 | `rpcserver.go` | JSON-RPC 服务端 |
| btcctl | `cmd/btcctl/` | RPC 命令行工具 |
| obtc-status | `cmd/obtc-status/` | HTTP 状态页面 |
| DevNet 脚本 | `scripts/` | 开发网络启动脚本 |

---

## 2. 环境要求与安装

### 2.1 系统要求

| 项目 | 最低要求 | 推荐配置 |
|------|---------|---------|
| 操作系统 | Linux / macOS / Windows / BSD | Linux (Ubuntu 22.04+) |
| Go 版本 | 1.22+ | 最新稳定版 |
| 内存 | 4 GB | 8 GB+ |
| 磁盘 | 500 GB SSD | 1 TB SSD（含全索引） |
| 网络 | 上行 1 Mbps | 上行 10 Mbps+ |

### 2.2 编译安装

```bash
cd /path/to/obtcd

# 编译所有二进制文件
make build

# 安装到 $GOPATH/bin
make install

# 安装精简版（strip 符号表）
make release-install
```

### 2.3 编译辅助工具

```bash
cd cmd/btcctl && go build        # RPC 命令行工具
go build ./cmd/obtc-status       # 状态页面工具
go build ./cmd/addblock          # 区块链导入工具
go build ./cmd/gencerts          # TLS 证书生成工具
go build ./cmd/findcheckpoint    # 检查点查找工具
```

### 2.4 验证安装

```bash
btcd --version
btcctl --version
```

---

## 3. 配置说明

### 3.1 配置文件位置

| 平台 | 默认路径 |
|------|---------|
| Linux / macOS | `~/.btcd/btcd.conf` |
| Windows | `%LOCALAPPDATA%\btcd\btcd.conf` |
| 自定义 | `btcd -C /path/to/btcd.conf` |

示例配置参见仓库中的 `sample-btcd.conf`。

### 3.2 核心配置项

#### 网络选择（互斥）

```conf
# OBTC 网络（选择其一）
obtcmainnet=1          # OBTC 主网（默认）
obtctestnet=1          # OBTC 测试网
obtcregtest=1          # OBTC 回归测试网

# 原始比特币网络（选择其一）
; testnet=1
; regtest=1
; simnet=1
```

#### 数据目录

```conf
datadir=~/.btcd/data           # 区块链与节点数据目录
```

#### RPC 服务

```conf
# RPC 认证（必填）
rpcuser=myuser
rpcpass=mypassword

# RPC 监听地址
rpclisten=127.0.0.1:19528

# 受限 RPC 用户（只能调用安全方法）
rpclimituser=limiteduser
rpclimitpass=limitedpassword

# TLS 设置
rpccert=~/.btcd/rpc.cert
rpckey=~/.btcd/rpc.key
notls=0                        # 生产环境务必启用 TLS

# 连接限制
rpcmaxclients=10               # 最大 RPC 客户端数
rpcmaxwebsockets=25            # 最大 WebSocket 连接数
```

#### P2P 网络

```conf
# 监听地址
listen=0.0.0.0:19527           # P2P 监听（OBTC 测试网端口）

# 节点管理
addpeer=node1.example.com:19527    # 启动时连接的节点
connect=node2.example.com:19527    # 仅连接指定节点（排他模式）
maxpeers=125                       # 最大连接数

# DNS 种子
nodnsseed=0                    # 禁用 DNS 种子发现

# 封禁策略
banthreshold=100               # 封禁分数阈值
banduration=24h                # 封禁时长
nobanning=0                    # 禁用封禁（调试用）

# 白名单
whitelist=192.168.1.0/24       # 白名单 IP/子网（不会被封禁）
```

#### 索引

```conf
txindex=1                      # 交易哈希索引（searchrawtransactions 等需要）
addrindex=1                    # 地址索引
expiryindex=1                  # OBTC 到期索引（OBTC 专用）
```

#### 挖矿

```conf
generate=true                  # 启用 CPU 挖矿
miningaddr=obtc1q...           # 挖矿奖励地址
blockmaxsize=750000            # 最大区块大小（字节）
blockminsize=0                 # 最小区块大小
blockmaxweight=3000000         # 最大区块权重
```

#### 日志

```conf
debuglevel=info                # 日志级别
logdir=~/.btcd/logs            # 日志目录
```

#### 性能调优

```conf
sigcachemaxsize=100000         # 签名缓存大小
maxorphantx=100                # 最大孤立交易数
prune=0                        # 裁剪模式（MB），0=不裁剪
```

#### OBTC 专用

```conf
reindex-expiry=0               # 启动时重建 ExpiryIndex
```

---

## 4. 启动与停止

### 4.1 基本启动

```bash
# OBTC 测试网
btcd --obtctestnet --rpcuser=myuser --rpcpass=mypassword

# OBTC 主网
btcd --obtcmainnet --rpcuser=myuser --rpcpass=mypassword

# 使用配置文件
btcd -C ~/.btcd/btcd.conf
```

### 4.2 生产环境启动示例

```bash
btcd \
  --obtcmainnet \
  --datadir=/data/obtcd \
  --logdir=/data/obtcd/logs \
  --rpclisten=0.0.0.0:9528 \
  --rpcuser=rpc_user \
  --rpcpass=rpc_password \
  --rpccert=/data/obtcd/certs/rpc.cert \
  --rpckey=/data/obtcd/certs/rpc.key \
  --listen=0.0.0.0:9527 \
  --txindex \
  --addrindex \
  --expiryindex \
  --maxpeers=200 \
  --debuglevel=info
```

### 4.3 优雅停止

```bash
# 发送 SIGINT 或 SIGTERM
kill -SIGINT $(pgrep btcd)

# 或前台运行时按 Ctrl+C
```

日志中会输出 `Shutdown complete` 表示关闭完成。

> ⚠️ **避免使用 `kill -9`**，会跳过数据库安全关闭流程，可能导致数据损坏。

### 4.4 重启

```bash
kill -SIGINT $(pgrep btcd)
sleep 3  # 等待优雅关闭
btcd -C ~/.btcd/btcd.conf
```

### 4.5 systemd 服务配置

```ini
# /etc/systemd/system/obtcd.service
[Unit]
Description=OBTC Full Node (obtcd)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=obtc
Group=obtc
ExecStart=/usr/local/bin/btcd -C /etc/obtcd/btcd.conf
ExecStop=/bin/kill -SIGINT $MAINPID
Restart=on-failure
RestartSec=10
TimeoutStopSec=60
LimitNOFILE=65535

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/data/obtcd
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable obtcd
sudo systemctl start obtcd
sudo systemctl status obtcd
sudo journalctl -u obtcd -f
```

---

## 5. 网络与端口

### 5.1 P2P 端口

| 网络 | P2P 端口 | Magic Number | 说明 |
|------|---------|--------------|------|
| OBTC MainNet | 9527 | `0x4F425443` ("OBTC") | OBTC 主网 |
| OBTC TestNet | 19527 | `0x4F544553` ("OTES") | OBTC 测试网 |
| OBTC RegTest | 29527 | `0x4F524547` ("OREG") | OBTC 回归测试 |
| Bitcoin MainNet | 8333 | `0xD9B4BEF9` | 比特币主网 |
| Bitcoin TestNet | 18333 | `0x0709110B` | 比特币测试网 |
| SimNet | 18555 | `0x12141c16` | 仿真网络 |

### 5.2 RPC 端口

| 网络 | RPC 端口 |
|------|---------|
| OBTC MainNet | 9528 |
| OBTC TestNet | 19528 |
| OBTC RegTest | 29528 |
| Bitcoin MainNet | 8334 |
| Bitcoin TestNet | 18334 |
| SimNet | 18556 |

### 5.3 防火墙规则示例

```bash
# P2P 端口 — 允许所有入站（节点发现需要）
iptables -A INPUT -p tcp --dport 9527 -j ACCEPT       # OBTC 主网
iptables -A INPUT -p tcp --dport 19527 -j ACCEPT      # OBTC 测试网

# RPC 端口 — 仅允许内网
iptables -A INPUT -p tcp --dport 9528 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 9528 -j DROP
```

### 5.4 地址格式

| 网络 | Bech32 前缀 | PubKey 前缀 | Script 前缀 | BIP44 Coin Type |
|------|------------|------------|-------------|-----------------|
| OBTC MainNet | `obtc` | `0x47` | `0x32` | 20260 |
| OBTC TestNet | `obtct` | `0x71` | `0xD1` | 20261 |
| OBTC RegTest | `obtcrt` | `0x72` | `0xD2` | 20262 |

---

## 6. 日志管理

### 6.1 日志级别

```
trace > debug > info > warn > error > critical
```

设置方式：

```bash
# 全局日志级别
btcd -d info

# 配置文件
debuglevel=info

# 按子系统设置不同级别
btcd -d blockchain=debug,mempool=info,peer=warn
```

### 6.2 日志子系统

| 子系统 | 标识 | 说明 |
|--------|------|------|
| 主程序 | `btcd` | 节点主进程 |
| 区块链 | `blockchain` | 区块验证与链管理 |
| 区块日志 | `blocklog` | 区块处理日志 |
| 内存池 | `mempool` | 交易池 |
| 挖矿 | `mining` | 区块模板与挖矿 |
| 网络 | `net` | 网络层 |
| 对等节点 | `peer` | P2P 连接管理 |
| RPC | `rpc` | RPC 服务端 |
| RPC 客户端 | `rpcclient` | RPC 客户端 |
| 数据库 | `btcdb` | 数据库操作 |
| 地址管理 | `addrmgr` | 地址簿管理 |
| 费率估算 | `fee` | 手续费估算 |
| 脚本引擎 | `txscript` | 交易脚本 |

查看所有可用子系统：

```bash
btcd --debuglevel=show
```

### 6.3 日志文件

- **默认路径：** `~/.btcd/logs/{network}/btcd.log`
- **自动轮转**

### 6.4 常用日志检查

```bash
# 实时查看日志
tail -f ~/.btcd/logs/obtctestnet/btcd.log

# 检查错误
grep -i "ERR\|error\|critical\|fail" ~/.btcd/logs/obtctestnet/btcd.log

# 检查同步进度
grep -i "New block\|Processed\|synced" ~/.btcd/logs/obtctestnet/btcd.log

# 检查 REAP 交易
grep -i "reap\|expir" ~/.btcd/logs/obtctestnet/btcd.log

# 检查节点连接
grep -i "New peer\|peer connected\|peer disconnected" ~/.btcd/logs/obtctestnet/btcd.log
```

---

## 7. 数据库与存储

### 7.1 数据库类型

| 类型 | 说明 | 用途 |
|------|------|------|
| `ffldb` | 基于 LevelDB 的文件数据库 | **默认**，生产使用 |
| `memdb` | 内存数据库 | 仅测试用 |

### 7.2 目录结构

```
~/.btcd/
├── btcd.conf                      # 配置文件
├── rpc.cert                       # RPC TLS 证书
├── rpc.key                        # RPC TLS 私钥
├── logs/
│   └── obtctestnet/
│       └── btcd.log               # 日志文件
└── data/
    └── obtctestnet/               # 网络数据目录
        ├── blocks_ffldb/          # 区块链数据库
        │   ├── blocks.meta        # 元数据
        │   └── *.ldb              # LevelDB 数据文件
        ├── expiry-index/          # ExpiryIndex 数据库（OBTC 专用）
        └── peers.json             # 已知节点列表
```

### 7.3 索引管理

```bash
# 构建索引（首次启用时自动构建）
btcd --txindex                     # 交易哈希索引
btcd --addrindex                   # 地址索引
btcd --expiryindex                 # OBTC 到期索引

# 删除索引
btcd --droptxindex                 # 删除交易索引
btcd --dropaddrindex               # 删除地址索引
btcd --dropcfindex                 # 删除紧凑过滤器索引

# 重建 ExpiryIndex（OBTC 专用）
btcd --reindex-expiry
```

### 7.4 裁剪模式

```bash
# 启用裁剪（保留最近 1536 MB 的区块数据）
btcd --prune=1536
```

> ⚠️ 裁剪模式下无法使用 `txindex` 和 `addrindex`。

### 7.5 数据备份

```bash
# 停止节点后备份
systemctl stop obtcd

# 备份核心数据
tar -czf obtcd-backup-$(date +%Y%m%d).tar.gz \
  ~/.btcd/data/obtctestnet/blocks_ffldb \
  ~/.btcd/data/obtctestnet/expiry-index \
  ~/.btcd/data/obtctestnet/peers.json

systemctl start obtcd
```

> ⚠️ **必须在节点停止后再备份**数据库文件，运行中备份可能导致数据不一致。

---

## 8. RPC 接口

### 8.1 认证方式

- **HTTP Basic Auth：** `Authorization: Basic base64(rpcuser:rpcpass)`
- **WebSocket：** JSON-RPC `authenticate` 命令
- **双层认证：**
  - 管理员：`rpcuser` / `rpcpass`（完整权限）
  - 受限用户：`rpclimituser` / `rpclimitpass`（只读方法）

### 8.2 常用 RPC 方法

#### 节点信息

| 方法 | 说明 |
|------|------|
| `getinfo` | 获取节点基本信息 |
| `getblockcount` | 获取当前区块高度 |
| `getbestblockhash` | 获取最新区块哈希 |
| `getnetworkinfo` | 获取网络状态 |
| `getpeerinfo` | 获取连接的对等节点信息 |
| `getconnectioncount` | 获取连接数 |

#### 区块与交易

| 方法 | 说明 |
|------|------|
| `getblock <hash>` | 获取区块详情 |
| `getblockhash <height>` | 根据高度获取区块哈希 |
| `getblockheader <hash>` | 获取区块头 |
| `getblocktemplate` | 获取区块模板（挖矿用） |
| `getrawtransaction <txid>` | 获取原始交易 |
| `decoderawtransaction <hex>` | 解码原始交易 |
| `sendrawtransaction <hex>` | 广播原始交易 |
| `searchrawtransactions <addr>` | 搜索地址相关交易（需 addrindex） |

#### 节点管理

| 方法 | 说明 |
|------|------|
| `addnode <ip> <cmd>` | 添加/移除/获取节点 |
| `disconnectnode <ip>` | 断开指定节点 |
| `generate <n>` | 生成 n 个区块（测试网/simnet） |
| `validateaddress <addr>` | 验证地址格式 |
| `estimatefee <blocks>` | 估算手续费 |

#### OBTC 专用方法

| 方法 | 说明 |
|------|------|
| `getexpiryindexstats` | 获取 ExpiryIndex 统计信息 |
| `getreapplan` | 获取 REAP 交易计划（预览） |
| `getexpirycommitment` | 获取到期承诺信息 |

### 8.3 btcctl 使用示例

```bash
# 基本用法
btcctl --obtctestnet \
  --rpcuser=myuser --rpcpass=mypassword \
  --rpcserver=127.0.0.1:19528 \
  <command> [args]

# 查看节点信息
btcctl --obtctestnet --rpcuser=u --rpcpass=p getinfo

# 查看区块高度
btcctl --obtctestnet --rpcuser=u --rpcpass=p getblockcount

# 查看连接的节点
btcctl --obtctestnet --rpcuser=u --rpcpass=p getpeerinfo

# 查看指定高度的区块
btcctl --obtctestnet --rpcuser=u --rpcpass=p getblockhash 1000

# 查看 ExpiryIndex 状态（OBTC 专用）
btcctl --obtctestnet --rpcuser=u --rpcpass=p getexpiryindexstats

# 禁用 TLS（本地调试）
btcctl --obtctestnet --rpcuser=u --rpcpass=p --notls getinfo
```

### 8.4 curl 调用示例

```bash
# HTTP POST
curl -s --user myuser:mypassword \
  --data-binary '{"jsonrpc":"1.0","id":"1","method":"getblockcount","params":[]}' \
  -H 'Content-Type: application/json' \
  https://localhost:19528/

# 禁用 TLS 验证（自签名证书）
curl -sk --user myuser:mypassword \
  --data-binary '{"jsonrpc":"1.0","id":"1","method":"getinfo","params":[]}' \
  https://localhost:19528/
```

---

## 9. P2P 网络管理

### 9.1 连接模式

| 模式 | 参数 | 说明 |
|------|------|------|
| 正常模式 | （默认） | 通过 DNS 种子自动发现节点 |
| 添加节点 | `--addpeer` | 在自动发现基础上额外连接指定节点 |
| 仅连接 | `--connect` | 只连接指定节点，不进行自动发现 |
| 无 DNS | `--nodnsseed` | 禁用 DNS 种子发现 |

### 9.2 节点管理命令

```bash
# 查看连接的节点
btcctl --obtctestnet --rpcuser=u --rpcpass=p getpeerinfo

# 添加节点
btcctl --obtctestnet --rpcuser=u --rpcpass=p addnode "1.2.3.4:19527" add

# 移除节点
btcctl --obtctestnet --rpcuser=u --rpcpass=p addnode "1.2.3.4:19527" remove

# 断开节点
btcctl --obtctestnet --rpcuser=u --rpcpass=p disconnectnode "1.2.3.4:19527"

# 查看连接数
btcctl --obtctestnet --rpcuser=u --rpcpass=p getconnectioncount
```

### 9.3 封禁管理

```bash
# 配置封禁策略
btcd --banthreshold=100 --banduration=24h

# 白名单（不会被封禁）
btcd --whitelist=192.168.1.0/24

# 禁用封禁（调试环境）
btcd --nobanning
```

### 9.4 网络隔离机制

OBTC 网络与比特币网络完全隔离，通过以下机制实现：

- **独立 Magic Number**：OBTC 使用 `0x4F425443` 等，无法与比特币节点握手
- **独立端口**：9527/19527/29527，避免与比特币端口冲突
- **独立地址前缀**：Bech32 使用 `obtc`/`obtct`/`obtcrt`
- **独立 BIP44 Coin Type**：20260/20261/20262

---

## 10. 挖矿配置

### 10.1 CPU 挖矿

```bash
# 启用 CPU 挖矿
btcd --obtctestnet \
  --generate \
  --miningaddr=obtct1q... \
  --rpcuser=u --rpcpass=p
```

### 10.2 挖矿参数

```conf
generate=true                  # 启用挖矿
miningaddr=obtc1q...           # 挖矿奖励地址（必填）
blockmaxsize=750000            # 最大区块大小（字节）
blockminsize=0                 # 最小区块大小
blockmaxweight=3000000         # 最大区块权重
```

### 10.3 通过 RPC 生成区块（测试网）

```bash
# 生成 10 个区块
btcctl --obtctestnet --rpcuser=u --rpcpass=p generate 10
```

### 10.4 REAP 挖矿（OBTC 专用）

分叉后的区块中，矿工自动执行 REAP 流程：
1. 扫描即将到期的 UTXO
2. 构建 REAP 系统交易（到期 UTXO 价值的 30% 分配给矿工）
3. REAP 交易包含在区块的 Coinbase 中

相关代码在 `mining/reap/` 目录。

---

## 11. OBTC 特有功能

### 11.1 REAP 协议

**核心机制：**

| 参数 | 说明 |
|------|------|
| UTXO 寿命 | 7 年（约 3,679,200 个区块） |
| 矿工分成 | 到期价值的 30% |
| 分叉高度（主网） | ~950,000 |
| 分叉高度（测试网） | 2,800,000 |
| 分叉高度（回归测试） | 100 |

**关键函数：**

| 函数 | 说明 |
|------|------|
| `IsOBTC(params)` | 判断是否为 OBTC 网络 |
| `IsPostOBTCFork(params, height)` | 判断是否已过分叉高度 |
| `IsOBTCReplayProtectionActive(params, height)` | 判断重放保护是否激活 |
| `GetExpiryParams(params)` | 获取到期配置参数 |
| `CalculateExpiryKey(createHeight)` | 计算到期高度 |

### 11.2 ExpiryIndex

ExpiryIndex 是 OBTC 专用的 UTXO 到期索引，用于高效查询即将到期的 UTXO。

```bash
# 启用 ExpiryIndex
btcd --expiryindex

# 重建 ExpiryIndex
btcd --reindex-expiry

# 查看 ExpiryIndex 状态
btcctl --obtctestnet --rpcuser=u --rpcpass=p getexpiryindexstats
```

### 11.3 到期承诺（Expiry Commitment）

每个区块的 Coinbase 交易中包含一个到期状态承诺，用于轻客户端验证。

```bash
# 查看当前到期承诺
btcctl --obtctestnet --rpcuser=u --rpcpass=p getexpirycommitment
```

### 11.4 重放保护

OBTC 使用独立的 sighash domain，确保 OBTC 交易无法在比特币网络上重放，反之亦然。

### 11.5 obtc-status 状态页面

提供只读的 HTTP 状态页面，用于监控节点状态。

```bash
# 编译
go build ./cmd/obtc-status

# 启动（会连接到 obtcd 的 RPC 接口）
./obtc-status --rpcserver=127.0.0.1:19528 --rpcuser=u --rpcpass=p --listen=:8080
```

---

## 12. 辅助工具

### 12.1 btcctl — RPC 命令行工具

详见 [8.3 btcctl 使用示例](#83-btcctl-使用示例)。

### 12.2 gencerts — TLS 证书生成

```bash
# 生成 RPC 用的 TLS 证书
gencerts --directory=~/.btcd/ --host=mynode.example.com

# 生成指定有效期的证书
gencerts --directory=~/.btcd/ --years=10
```

### 12.3 addblock — 区块链导入

从 `bootstrap.dat` 文件快速导入区块数据：

```bash
addblock --infile=bootstrap.dat --dbtype=ffldb
```

### 12.4 findcheckpoint — 检查点查找

```bash
findcheckpoint --obtctestnet
```

### 12.5 DevNet 脚本

详见 [17. DevNet 开发环境](#17-devnet-开发环境)。

---

## 13. 健康检查与监控

### 13.1 进程检查

```bash
# 进程是否存活
pgrep -a btcd

# 查看资源使用
ps aux | grep btcd
```

### 13.2 RPC 健康检查

```bash
# 检查节点信息
btcctl --obtctestnet --rpcuser=u --rpcpass=p getinfo

# 检查区块高度（与外部对比判断是否同步）
btcctl --obtctestnet --rpcuser=u --rpcpass=p getblockcount

# 检查连接数
btcctl --obtctestnet --rpcuser=u --rpcpass=p getconnectioncount

# 检查节点详情
btcctl --obtctestnet --rpcuser=u --rpcpass=p getnetworkinfo
```

### 13.3 同步状态判断

```bash
# 获取当前区块高度
LOCAL_HEIGHT=$(btcctl --obtctestnet --rpcuser=u --rpcpass=p getblockcount)

# 与已知高度比较，判断是否落后
echo "当前高度: $LOCAL_HEIGHT"
```

### 13.4 建议的监控指标

| 指标 | 检查方式 | 告警阈值 |
|------|---------|---------|
| 进程存活 | `pgrep btcd` | 进程不存在 |
| RPC 可达 | `getinfo` 调用 | 超时 > 5s |
| 区块高度 | `getblockcount` | 落后 > 10 个区块 |
| 对等节点数 | `getconnectioncount` | < 3 个连接 |
| 磁盘空间 | `df -h` 数据目录 | 使用率 > 90% |
| 磁盘 I/O | `iostat` | util > 90% 持续 5 分钟 |
| 内存使用 | `ps` RSS | RSS > 4 GB |
| ExpiryIndex | `getexpiryindexstats` | 状态异常 |

### 13.5 简单的健康检查脚本

```bash
#!/bin/bash
# obtcd-health-check.sh

RPC_USER="u"
RPC_PASS="p"
RPC_OPTS="--obtctestnet --rpcuser=$RPC_USER --rpcpass=$RPC_PASS"

# 检查进程
if ! pgrep -x btcd > /dev/null; then
    echo "CRITICAL: btcd 进程不存在"
    exit 2
fi

# 检查 RPC
HEIGHT=$(btcctl $RPC_OPTS getblockcount 2>/dev/null)
if [ $? -ne 0 ]; then
    echo "CRITICAL: RPC 连接失败"
    exit 2
fi

# 检查连接数
PEERS=$(btcctl $RPC_OPTS getconnectioncount 2>/dev/null)
if [ "$PEERS" -lt 3 ]; then
    echo "WARNING: 对等节点数过少 ($PEERS)"
    exit 1
fi

echo "OK: 高度=$HEIGHT, 节点数=$PEERS"
exit 0
```

---

## 14. 常见故障排查

### 14.1 节点无法同步

**症状：** 区块高度长时间不增长

**排查步骤：**

1. 检查对等节点连接：
   ```bash
   btcctl --obtctestnet --rpcuser=u --rpcpass=p getpeerinfo
   ```
2. 如果连接数为 0，手动添加节点：
   ```bash
   btcd --addpeer=known-node.example.com:19527
   ```
3. 检查网络连通性：
   ```bash
   nc -zv known-node.example.com 19527
   ```
4. 启用 debug 日志排查：
   ```bash
   btcd -d blockchain=debug,peer=debug
   ```

### 14.2 RPC 连接失败

**症状：** `btcctl` 报错 `connection refused` 或 `authentication failed`

**排查步骤：**

1. 确认 btcd 正在运行：`pgrep btcd`
2. 确认 RPC 用户名和密码正确
3. 确认 RPC 监听地址和端口：
   ```bash
   ss -tlnp | grep btcd
   ```
4. 检查 TLS 证书：
   ```bash
   ls -la ~/.btcd/rpc.cert ~/.btcd/rpc.key
   # 如需重新生成
   rm ~/.btcd/rpc.cert ~/.btcd/rpc.key
   # 重启 btcd 自动生成
   ```
5. 本地调试可禁用 TLS：
   ```bash
   btcd --notls
   btcctl --notls --rpcuser=u --rpcpass=p getinfo
   ```

### 14.3 ExpiryIndex 损坏

**症状：** `getexpiryindexstats` 返回异常数据，或日志中出现 ExpiryIndex 相关错误

**修复：**

```bash
# 停止节点
systemctl stop obtcd

# 重建 ExpiryIndex
btcd --obtctestnet --reindex-expiry --rpcuser=u --rpcpass=p
```

### 14.4 内存占用过高

**排查步骤：**

1. 减小签名缓存：
   ```bash
   btcd --sigcachemaxsize=50000
   ```
2. 减少最大连接数：
   ```bash
   btcd --maxpeers=50
   ```
3. 启用裁剪模式：
   ```bash
   btcd --prune=2000
   ```

### 14.5 节点卡在某个高度

**排查步骤：**

1. 启用 debug 日志：
   ```bash
   btcd -d blockchain=debug
   ```
2. 检查是否有共识验证失败：
   ```bash
   grep -i "validation failed\|reject\|invalid" ~/.btcd/logs/obtctestnet/btcd.log
   ```
3. 最后手段 — 从头同步：
   ```bash
   systemctl stop obtcd
   rm -rf ~/.btcd/data/obtctestnet/blocks_ffldb
   systemctl start obtcd
   ```

### 14.6 磁盘空间不足

**应急处理：**

1. 清理日志：
   ```bash
   truncate -s 0 ~/.btcd/logs/obtctestnet/btcd.log
   ```
2. 启用裁剪：
   ```bash
   btcd --prune=1536
   ```
3. 删除不需要的索引：
   ```bash
   btcd --droptxindex --dropaddrindex
   ```

### 14.7 启动时数据库锁冲突

**症状：** `database already in use` 或 `lock file`

**排查：**

```bash
# 确认没有其他实例在运行
pgrep -a btcd

# 如果确认没有，可能是异常退出残留
# 等待几秒后重试启动即可
```

---

## 15. 灾难恢复

### 15.1 数据库损坏

```bash
# 1. 停止节点
systemctl stop obtcd

# 2. 备份损坏数据（留存分析）
mv ~/.btcd/data/obtctestnet/blocks_ffldb \
   ~/.btcd/data/obtctestnet/blocks_ffldb.corrupted

# 3. 重启节点（从零同步）
systemctl start obtcd
```

> 全节点数据可从网络完全恢复，只需要时间重新同步。

### 15.2 从备份恢复

```bash
# 1. 停止节点
systemctl stop obtcd

# 2. 恢复备份
tar -xzf obtcd-backup-YYYYMMDD.tar.gz -C ~/.btcd/data/obtctestnet/

# 3. 重建 ExpiryIndex（如有需要）
btcd --obtctestnet --reindex-expiry --rpcuser=u --rpcpass=p
```

### 15.3 快速同步（使用 bootstrap）

```bash
# 如有 bootstrap.dat 文件
addblock --infile=bootstrap.dat --dbtype=ffldb --obtctestnet
btcd --obtctestnet --rpcuser=u --rpcpass=p
```

---

## 16. 安全注意事项

### 16.1 RPC 安全

- **必须启用 TLS**（生产环境不要使用 `--notls`）
- 使用强密码设置 `rpcuser` / `rpcpass`
- 对外部客户端使用 `rpclimituser` / `rpclimitpass`（只读权限）
- RPC 端口不要暴露到公网
- 限制 `rpcmaxclients` 防止资源耗尽

### 16.2 文件权限

```bash
chmod 700 ~/.btcd
chmod 600 ~/.btcd/btcd.conf
chmod 600 ~/.btcd/rpc.key
chmod 600 ~/.btcd/rpc.cert
```

### 16.3 网络安全

- P2P 端口可对外开放（节点发现需要）
- RPC 端口仅限内网访问
- 使用防火墙规则限制来源 IP
- 考虑使用 `--proxy` 通过 SOCKS5/Tor 增强隐私

### 16.4 运维安全

- 不以 root 用户运行
- 使用 systemd 的安全加固选项（`NoNewPrivileges`、`ProtectSystem` 等）
- 定期更新到最新版本
- 定期备份数据

---

## 17. DevNet 开发环境

obtcd 提供了 DevNet 脚本，方便本地启动 2 节点仿真网络。

### 17.1 启动 DevNet

```bash
# 启动开发网络
./scripts/devnet-up.sh start

# 检查状态
./scripts/devnet-up.sh status

# 运行示例交易
./scripts/devnet-up.sh demo

# 查看日志
./scripts/devnet-up.sh logs
```

### 17.2 停止 DevNet

```bash
# 停止网络
./scripts/devnet-up.sh stop

# 清理所有数据
./scripts/devnet-up.sh clean
```

---

## 附录 A：快速参考卡

```bash
# === 安装 ===
make install

# === 启动 ===
btcd --obtctestnet --rpcuser=u --rpcpass=p              # 测试网
btcd --obtcmainnet --rpcuser=u --rpcpass=p              # 主网
btcd -C ~/.btcd/btcd.conf                                # 配置文件

# === 停止 ===
kill -SIGINT $(pgrep btcd)

# === 版本 ===
btcd --version

# === 查看状态 ===
btcctl --obtctestnet --rpcuser=u --rpcpass=p getinfo
btcctl --obtctestnet --rpcuser=u --rpcpass=p getblockcount
btcctl --obtctestnet --rpcuser=u --rpcpass=p getpeerinfo
btcctl --obtctestnet --rpcuser=u --rpcpass=p getconnectioncount

# === OBTC 专用 ===
btcctl --obtctestnet --rpcuser=u --rpcpass=p getexpiryindexstats
btcctl --obtctestnet --rpcuser=u --rpcpass=p getreapplan
btcctl --obtctestnet --rpcuser=u --rpcpass=p getexpirycommitment

# === 索引管理 ===
btcd --reindex-expiry                                    # 重建 ExpiryIndex
btcd --droptxindex                                       # 删除交易索引
btcd --dropaddrindex                                     # 删除地址索引

# === 日志 ===
tail -f ~/.btcd/logs/obtctestnet/btcd.log

# === DevNet ===
./scripts/devnet-up.sh start
./scripts/devnet-up.sh status
./scripts/devnet-up.sh stop
```

## 附录 B：生产环境配置模板

```conf
# /etc/obtcd/btcd.conf — 生产环境参考配置

# === 网络 ===
obtcmainnet=1

# === 数据 ===
datadir=/data/obtcd/data
logdir=/data/obtcd/logs
debuglevel=info

# === RPC ===
rpcuser=CHANGE_ME
rpcpass=CHANGE_ME
rpclimituser=CHANGE_ME_LIMITED
rpclimitpass=CHANGE_ME_LIMITED
rpclisten=0.0.0.0:9528
rpccert=/data/obtcd/certs/rpc.cert
rpckey=/data/obtcd/certs/rpc.key
rpcmaxclients=20
rpcmaxwebsockets=50

# === P2P ===
listen=0.0.0.0:9527
maxpeers=200

# === 索引 ===
txindex=1
addrindex=1
expiryindex=1

# === 性能 ===
sigcachemaxsize=100000
maxorphantx=100
```
