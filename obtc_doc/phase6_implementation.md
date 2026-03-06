# Phase 6 计划（加速 Testnet 部署与观测）— 网络上线、三地域种子、最小观测面

> 修订（2026-02-11）：若 Week4/5 仍在稳定期，Week6 先做“私有 testnet + 单地域双节点 + 可复现部署”，三地域公网种子可顺延到 Week6.5，优先保证协议稳定性。

**时间预算：18–20 小时**｜**目标**：把 **OBTC-Testnet** 跑起来并稳定运行 7–10 天；提供最小观测（区块/节点/REAP 指标）；验证“到期→REAP→矿工收税”的**端到端**在公网可复现。

---

## 🎯 本周目标（Definition of Done）

* 启用 **OBTC-Testnet**（独立网络参数、独立创世、独立端口/HRP/WIF/BIP32），**到期窗口=7 天**（仅 Testnet 加速）。
* 3 台\*\*种子节点（不同地域）\*\*上线可联通：至少 **EU / US / AS**。
* 发布 **Testnet 二进制 & Docker 镜像** + 一键接入文档。
* 部署最小只读状态页（`obtc-status`），公开：高度、出块间隔、节点数、mempool、近 288 块 REAP 税额汇总。
* 从外部新节点\*\*<2 小时内\*\*可同步到头。
* 连续稳定出块 ≥ 72 小时（本周内先跑满 72h；7–10 天的长观测继续运行）。

---

## 🗺️ 网络与参数（本周冻结）

> Week1 已有 `params_obtc.go` 骨架；本周**填实并启用** Testnet 参数。

* **网络名**：`obtc-testnet`
* **魔数**：`wire.OBTCTestNet`（全新且与 BTC/Testnet/Regtest 不冲突，如 `0xF1C0B7D7`）
* **默认端口**：P2P `19527`，RPC `19528`
* **地址 HRP**：`"tbob"`（示例，避免与 `tb`/`bc` 混淆）
* **WIF/BIP32**：自定义前缀（**不得**使用 BTC/xpub/xprv 前缀），写入 README 参数表
* **到期窗口**：`ExpiryParams{Mode:ByHeight, WindowBlocks = 7d * 144 = 1008}`（按 10 分钟块间隔计算）
* **PoW/难度**：开启 **AllowMinDifficultyBlocks=true**（类似 BTC Testnet 规则），便于低算力维持活性
* **REAP 上限**：`MaxREAPInputsPerBlock`（例如 200） 与 `MaxReapTaxPerBlock`（例如 0.2 \* BlockSubsidy）
* **DNSSeeds**：本周先用**静态 IP 种子**，`DNSSeeds` 留占位；若你有域名，可同时配置 `seed-eu.test.obtc.org` 等

---

## 🗂️ 本周交付物（Deliverables）

* `chaincfg/params_obtc.go`：`OBTCTestNetParams` 填实并 `Register()` 生效
* `cmd/gengenesis/`：创世生成器（输出创世哈希/merkle/nonce/时间戳；附校验器）
* `build/`：Dockerfile、Compose、打包脚本（linux/amd64, darwin/arm64, windows/amd64）
* `cmd/obtc-status/`：最小只读状态页（JSON/HTML）部署在每台种子上
* `infra/`：`systemd` 单元与防火墙脚本；`seed-userdata.sh` 云主机一键初始化
* `docs/testnet-join.md`：对外接入指南（端口/参数/校验/常见故障）
* `docs/phase6-validation.md`：观测记录、同步用时、REAP 指标快照

---

## 🧩 任务拆解与时间分配（≤ 20h）

### 1) 参数启用与创世冻结（4h）

* **填实与启用** `OBTCTestNetParams`（见“网络与参数”）；`init() { Register(&OBTCTestNetParams) }`
* **创世生成**：

  * 写 `cmd/gengenesis/`：输入创世时间/消息，搜索 nonce（允许最小难度）
  * 输出：`genesisHash`, `merkleRoot`, `time`, `bits`, `nonce`
  * 同时生成 `genesis.json` + `genesis.h`（Go 常量）
* **校验器**：`cmd/checkgenesis` 读取常量再算一遍，保证可复现
* **DoD**：切到 `--net=obtc-testnet` 本地两节点能起链出块

### 2) 构建产物与发布（3h）

* **CI**：在现有 Linux 任务上新增 **Testnet 构建步骤**；产出三平台二进制 + sha256 校验
* **Dockerfile**：静态构建 `btcd` + `obtc-status`（scratch 或 distroless）
* **Compose**：`docker compose up obtc-fullnode` 一键拉起（映射 P2P/RPC/STATUS 端口）
* **DoD**：本地可 `docker run` 起完整节点并接入种子

### 3) 三地域种子节点部署（6h）

* **云主机**：3 台（EU/US/AS），Ubuntu LTS，固定公网 IP
* **初始化脚本** `infra/seed-userdata.sh`：

  * 安装二进制到 `/opt/obtc/`
  * 建立用户 `obtc`、数据目录 `/var/lib/obtc`
  * 写入 `btcd.conf`（见下）
  * 安装 `systemd` 单元并启动
  * 开放防火墙：`ufw allow 19527/tcp`；RPC 仅 `localhost`
* **`/etc/obtc/btcd.conf` 示例**

  ```
  ; OBTCTestNet
  addpeer=<EU-IP-OTHER-SEED>
  addpeer=<US-IP-OTHER-SEED>
  listen=0.0.0.0:19527
  rpclisten=127.0.0.1:19528
  nobanning=1
  noblacklist=1
  txindex=1
  notls=1
  network=obtc-testnet
  ; P2P v1 only（如默认有 v2）
  nov2=1
  ```
* **`systemd` 单元 `btcd.service`**

  ```
  [Unit]
  Description=OBTC Testnet Node
  After=network-online.target

  [Service]
  User=obtc
  ExecStart=/opt/obtc/btcd --config=/etc/obtc/btcd.conf
  Restart=on-failure
  LimitNOFILE=1048576

  [Install]
  WantedBy=multi-user.target
  ```
* **状态页服务** `obtc-status.service`（监听 `:28580` 只读）

  * 展示：`height`, `hash`, `peerCount`, `mempoolSize`, `last10BlockIntervals`, `reapTax24h`, `reapInputs24h`
* **DoD**：三台均为 **`addnode`/`connect` 目标**；互连可见，状态页可访问

### 4) 最小观测面与指标计算（3h）

* **节点端**：在出块时记录 REAP 蓝图/交易统计到日志（或导出内部 `/status` JSON）
* **状态页聚合**：

  * `GET /status.json`：

    ```json
    {
      "height":12345,
      "median_block_interval_sec":612,
      "peers":23,
      "mempool":42,
      "reap_last_288_blocks":{"count":198,"tax_sum": "12.34 OBTC","inputs": 3521}
    }
    ```
* **离线脚本** `tools/reap-audit.go`：从 RPC 扫近 288 块，统计：

  * **REAP 覆盖率** = 被 REAP 的到期 UTXO / 到期总数
  * **REAP 积压** = 当前已过期但未被 REAP 的 UTXO 数
  * **孤块率** = orphan / (main+orphan)
* **DoD**：跑一次审计脚本输出 CSV/Markdown 表格，纳入 `docs/phase6-validation.md`

### 5) 外部接入指南与故障排查（2h）

* `docs/testnet-join.md`：

  * **下载**（二进制/Docker）、**校验**（sha256/minisign）
  * **运行**：命令行 / Docker Compose
  * **连种子**：三台 IP + 端口 `19527`（以及 `--network=obtc-testnet`）
  * **FAQ**：同步慢、端口被占、时间不同步、无法连接某区域等
* 截图：`obtc-status` 页、同步中日志

---

## 🚀 执行步骤速览（可直接用）

### A) 本地切换到 Testnet 验证

```bash
# 切换分支并构建
git checkout obtc-main
go build ./...

# 起两节点（Testnet）
./btcd --network=obtc-testnet --datadir=.obtc/node1 --listen=127.0.0.1:19527 --rpclisten=127.0.0.1:19528 --txindex --notls --rpcuser=u --rpcpass=p &
./btcd --network=obtc-testnet --datadir=.obtc/node2 --listen=127.0.0.1:19529 --rpclisten=127.0.0.1:19530 --txindex --notls --rpcuser=u --rpcpass=p --connect=127.0.0.1:19527 &
```

### B) 容器化（单机）

```bash
docker build -t obtc/node:tn .
docker run -p 19527:19527 -p 19580:19580 obtc/node:tn  # P2P+状态页
```

### C) 观测脚本（示例）

```bash
go run tools/reap-audit.go --rpc=http://u:p@127.0.0.1:28556 --window=288 --out=docs/reap-288.csv
```

---

## ✅ 本周验收清单（可勾选）

* [ ] `OBTCTestNetParams` 启用且与主/回归网络**完全隔离**（魔数/端口/HRP/WIF/BIP32）
* [ ] 生成并**冻结** Testnet 创世（哈希/时间/nonce/merkle）并写入校验器
* [ ] 三地域种子节点在线，**互连可见**，`obtc-status` 可访问
* [ ] 外部新节点 **< 2 小时** 同步到头
* [ ] 公开**接入指南**（二进制+Docker+校验）
* [ ] 72 小时内：**连续出块**、REAP 正常、审计脚本生成统计报告
* [ ] `docs/phase6-validation.md` 附：高度曲线、出块间隔箱线图、REAP 24h 汇总、孤块率、积压曲线

---

## 📊 运行期关键指标（7–10 天观测窗口）

* **出块间隔**：中位数接近 600s（±20% 可接受）
* **孤块率**：≤ 3%（短期波动允许 5% 峰值）
* **REAP 覆盖率**：≥ 95%（到期后 N 块内被处理）
* **REAP 积压**：稳定在**小于 MaxInputsPerBlock × 3**
* **同步时间**：新节点 < 2h（带宽正常情况下）

---

## 🧱 风险 & 预案

* **算力不足/出块停滞**：启用 `AllowMinDifficultyBlocks`；必要时临时开启**自挖**辅助。
* **链上参数错配**（地址/WIF/BIP32/魔数冲突）：**立即下线发布页**，回滚到上一个 tag，重新生成创世（Testnet 允许重启）。
* **种子单点**：三地域**互为对等**，并在代码中写入多个 `addnode`；若一台故障，更新 README 去掉该 IP。
* **观测缺失**：状态页挂了不影响出块；用审计脚本直连任一 RPC 节点补数。

---

## 🧰 Cursor 助攻清单

* 批量改写 `params_obtc.go` 与相关引用（HRP/WIF/BIP32/端口），自动生成常量表与 README 参数片段
* 生成 `systemd` 单元、Dockerfile/Compose、`seed-userdata.sh`
* 状态页与审计脚本的样板代码、JSON 结构与测试桩
* 生成 `docs/testnet-join.md` 的模板（带命令区块与截图占位）

---
