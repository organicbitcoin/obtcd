# Phase 8 计划（主网候选发布 | Mainnet-Candidate）— 参数冻结、发布、72h 观察

> 修订（2026-02-11）：建议把“主网候选”改为“公开候选测试网里程碑”。若必须主网候选，需新增独立安全审计与外部回放验证门槛（至少 1 次外部审计）。

**时间预算：18–20 小时** ｜ **目标**：冻结 **OBTC-Mainnet** 共识参数与创世，构建并签名发布产物，部署 ≥3 个地域的主网种子节点，完成 **72 小时**稳定出块与外部可同步验证，达到门槛后对外宣布“Mainnet-Candidate 上线”。

---

## 🎯 本周目标（Definition of Done）

* **参数冻结**：主网 **魔数/端口/HRP/WIF/BIP32/创世/税率/上限/REAP\_VERSION/BurnPolicy** 写入代码与文档，打标签。
* **发布产物**：三平台二进制 + Docker 镜像 + `SHA256SUMS` + `minisign` 签名；复现构建指引更新。
* **主网种子**：EU / US / AS 至少各一台，互联可见，状态页正常。
* **对外文档**：`mainnet-join.md`（一键接入、校验、常见问题），发布说明 `release-notes-v1.0.0-candidate.md`。
* **运行验证**：连续 **≥72h** 稳定出块；外部新节点 **<2h** 同步到头；REAP 正常触发；无 >1 深度重组。

---

## 🗺️ 主网参数（本周一次性冻结）

> 数值示例为占位，**实际以你在仓库中写入的常量为准**（一旦发布即不可更改）。

* **网络名**：`obtc-mainnet`
* **魔数**：`wire.OBTCNet = 0xF1C0B7C1`（示例；需与所有 BTC 网络不同）
* **端口**：P2P `38555`，RPC `38556`
* **地址 HRP**：`"ob"`（或 `"obtc"`，保持唯一性）
* **WIF/BIP32 前缀**：自定义，**不得**为 `0x80 / 0xEF` 与 `xpub/xprv` 系列（写入参数表与 README）
* **到期窗口**：`ExpiryMode=ByHeight`，`WindowBlocks ≈ 7 年 = 52,596 × 7 = 368,172`
* **税率**：`TaxRate = 30/100`（逐输入 `floor` 累加）
* **REAP 上限**：`MaxREAPInputsPerBlock`（例如 200）；`MaxReapTaxPerBlock`（例如 ≤ BlockSubsidy 的 20%）
* **BurnPolicy**：`OP_RETURN` 或 `P2WSH_Zero`（二选一，**冻结**）
* **REAP\_VERSION**：整数常量（与交易识别强绑定）
* **Seeds**：三地域静态 IP（后续可补 DNSSeeds）

> ⚠️ 主网**不可**像 Testnet 一样“重启链”，所以创世与参数必须先在文档中二次复核（见下文“Go/No-Go 清单”）。

---

## 📦 本周交付物（Deliverables）

* 代码：

  * `chaincfg/params_obtc.go`：`OBTCMainNetParams` 填实并 `Register()`
  * `cmd/gengenesis/` & `cmd/checkgenesis/`：用于生成/校验创世（输出常量、再计算校验）
  * `build/release.sh` & `Dockerfile.release`：三平台产物、校验与签名
  * `cmd/obtc-status/`：状态页构建与服务（沿用 Phase 6）
* 基础设施：

  * `infra/mainnet-userdata.sh`：一键初始化种子节点（systemd、UFW、日志轮转）
  * `systemd` 单元：`btcd.service` / `obtc-status.service`（主网端口）
* 文档：

  * `docs/mainnet-params.md`（冻结参数表）
  * `docs/mainnet-join.md`（下载/校验/运行/排错）
  * `docs/release-notes-v1.0.0-candidate.md`（发布说明）
  * `docs/phase8-validation.md`（72h 观察记录与指标快照）

---

## 🧩 任务拆解与时间分配（≤ 20h）

### 1) 参数冻结 & 创世生成（4h）

* 填实 `OBTCMainNetParams`（见“主网参数”），`init() { Register(&OBTCMainNetParams) }`；
* 用 `gengenesis` 生成创世（含时间戳/消息/nonce/bits），导出到常量文件；
* 用 `checkgenesis` 复算哈希与 merkle，**双人流程**：在文档中记录两次独立计算结果（你可以自检两遍）；
* 本地起 **两节点（mainnet）** 连通并出第一个区块（可临时自挖以验证）。

### 2) 发布构建与签名（3h）

* 锁 `go.mod`（Phase 7 已做）；
* 容器化构建三平台产物（`-trimpath -ldflags "-s -w -buildid="`），生成 `SHA256SUMS`；
* 用 `minisign` 对 `SHA256SUMS` 签名（私钥离线保存）；
* 构建 `obtc/node:mainnet` Docker 镜像（包含 `btcd` 与 `obtc-status`）。

### 3) 主网种子节点部署（6h）

* EU/US/AS 各 1 台云主机：

  * 安装二进制到 `/opt/obtc/`，数据 `/var/lib/obtc`；
  * `btcd.conf`（主网端口/参数），`systemd` 启动；
  * 开放 `38555/tcp`（P2P），RPC 仅本机；状态页 `:38580` 只读；
  * 彼此 `addpeer` 互连，验证 `peerCount`；
  * 将 IP 写入代码 `addnode` 与 `docs/mainnet-join.md`。
* 快速健康检查：出块间隔、peer 数、状态页可用。

### 4) 公布接入指南 & 发布页（2h）

* `docs/mainnet-join.md`：

  * 二进制与 Docker 下载地址；
  * **校验步骤**（`sha256sum` + `minisign -Vm`）；
  * 快速启动命令（命令行 & Compose）；
  * 连接种子（3 IP）；
  * 常见故障：端口占用、时钟不同步、带宽不足、区块同步慢。
* `release-notes-v1.0.0-candidate.md`：

  * 协议摘要、核心差异（REAP/税/上限/隔离参数）、兼容性声明、已知问题与降级开关。

### 5) 72 小时观察与热修（3–4h）

* 观察窗口从“发布页上线 + 种子正常”起计：

  * 每 1–2 小时采集：高度、出块间隔中位数（近 50/288 块）、孤块率、REAP 税总额、REAP 积压（到期未清理量）。
  * 外部新节点自零同步一次，记录总时长（目标 **<2h**）。
  * 如出现异常：**仅做最小热修**（非共识），例如状态页/日志/阈值微调；共识问题则**保持候选状态**、发布修复说明。
* 将结果持续写入 `docs/phase8-validation.md`。

### 6) 宣布与标签（1h）

* 若 72h 指标达标，打标签 `v1.0.0-candidate`（或 `v1.0.0` 视你策略），更新发布页“Mainnet-Candidate 上线”；
* 若未达标：保持 “Candidate” 状态，发补丁版本 `-rc2` 与说明，**不改创世**。

> 预留 **1–2h 机动** 用于突发排障或文档修订。

---

## 🚀 启动顺序（Runbook 摘要）

1. 合并 `OBTCMainNetParams` & 创世常量 → 构建签名 → 放出下载页；
2. 启动三地域种子节点，确认互连；
3. 公开 `mainnet-join.md` 与参数表；
4. 观察窗口 T0 开始（社区可接入），你保留一台私有小矿机防“冷启动”；
5. 每 1–2h 例行检查与记录；
6. 72h 达标 → 公告“Mainnet-Candidate 上线”。

---

## ✅ Go/No-Go 清单（发布前必须打勾）

* [ ] **参数冻结表**与代码一致（魔数/端口/HRP/WIF/BIP32/税率/上限/REAP\_VERSION/BurnPolicy）
* [ ] 创世哈希/merkle/nonce 经 **双重校验**；`checkgenesis` 通过
* [ ] 三地域种子互连可见，状态页正常
* [ ] 三平台产物可复现构建，`SHA256SUMS` 与 `minisign` 验签通过
* [ ] `mainnet-join.md` 可让新人 15 分钟起节点并开始同步
* [ ] 观察脚本/状态页能正确汇总 REAP 指标

---

## 📊 指标门槛（72h 内）

* **出块间隔中位数**：600s ± 20%（近 288 块）
* **孤块率**：≤ 3%（短时峰值 ≤ 5%）
* **REAP 覆盖率**：≥ 95%（到期后 N 块内被处理）
* **REAP 积压**：稳态 < `3 × MaxREAPInputsPerBlock`
* **同步时长**：外部新节点 < 2 小时到头
* **稳定性**：无 >1 深度重组；无崩溃

---

## 🧱 风险与回退策略

* **参数/创世出错**：若未公开发布即发现 → 重新生成并重构建；若已对外发布 → **保持 Candidate，不作硬分叉**，发公告与修复计划（必要时宣布新链为 `obtc-mainnet2`，避免污染）。
* **共识缺陷**：立即冻结发布，**不建议继续接入**；推出修复版候选（新 tag），通过 Testnet 回放后再重启观察窗口。
* **出块停滞**：短时用私有算力“引导”恢复；如长期算力不足，评估 PoW 难度参数的策略层微调（不改共识）。
* **种子故障**：快速替换 IP 并更新 `mainnet-join.md`；尽量保持 ≥2 台在线。

---

## 🧰 附：示例配置片段

**`/etc/obtc/btcd.conf`（主网）**

```
listen=0.0.0.0:38555
rpclisten=127.0.0.1:38556
network=obtc-mainnet
txindex=1
notls=1
; 固定若干种子
addpeer=<EU-SEED-IP>
addpeer=<US-SEED-IP>
addpeer=<AS-SEED-IP>
; 建议仅 v1 传输（如 v2 未全面验证）
nov2=1
```

**Docker Compose（单机全节点 + 状态页）**

```yaml
services:
  node:
    image: obtc/node:mainnet
    ports: ["38555:38555","38580:38580"]
    volumes: ["./data:/var/lib/obtc"]
    restart: unless-stopped
```

---

## 📑 文档清单（需要更新/新增）

* `docs/mainnet-params.md`（参数冻结表）
* `docs/mainnet-join.md`（一键接入指南）
* `docs/release-notes-v1.0.0-candidate.md`（发布说明）
* `docs/phase8-validation.md`（72h 观察与指标）

---
