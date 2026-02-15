# Phase 7 计划（硬化 & 封版）— 稳定性、可复现构建、发布候选

**时间预算：18–20 小时** ｜ **目标**：在 Testnet 连续运行基础上完成**压力/模糊测试**与**参数冻结**，把“系统交易/税收/上限”等关键规则**上升到共识硬约束**；锁定依赖与构建流程，形成 **Mainnet-Candidate RC（发布候选）**。

---

## 🎯 本周目标（Definition of Done）

* **共识硬化**：

  * `MaxREAPInputsPerBlock`、`MaxReapTaxPerBlock`、REAP 唯一性（每块≤1 笔）、“过期仅能被 REAP 花费”等**全部在共识层校验**。
  * ExpiryIndex **版本号/迁移**机制到位（`indexVersion`）。
* **稳定性验证**：完成 4 组压力/故障注入（见下），**无崩溃、无错误分叉**，指标达标。
* **可复现构建**：固定 `go.mod`、构建容器、编译参数（`-trimpath -ldflags`），生成三平台二进制与 **SHA256 + minisign**。
* **发行资产草案**：Release Notes、参数表、校验步骤、一键部署脚本。
* **Go/No-Go** 门槛打勾（见底部清单）。

---

## 🗂️ 本周交付物（Deliverables）

* `blockchain/validation_reap.go`：补齐/提升**共识级**上限与一致性检查（如有尚在模板层的因素）。
* `blockchain/expiryindex/buckets.go`：`indexVersion`、迁移路径与 `reindex` 开关。
* `tools/chaos/`：压力与故障注入脚本（小额洪水、reorg 回放、kill-9 重启、磁盘限速）。
* `build/`：

  * `Dockerfile.release`（固定基础镜像与 Go 版本）
  * `release.sh`（三平台构建、`-trimpath`、产出 `SHA256SUMS` 与 `.minisig`）
* 文档：

  * `docs/phase7-validation.md`（压测与指标报告）
  * `docs/release-notes-rc.md`（RC 发布说明）
  * `docs/repro-build.md`（可复现构建步骤）

---

## 🔧 硬化项（代码层，优先顺序）

1. **共识上限**（如果第 4 周仅模板侧截断，本周必须上链）：

   * `Enforce(MaxREAPInputsPerBlock)`：REAP 交易输入数超限 → **区块无效**。
   * `Enforce(MaxReapTaxPerBlock)`：税总额超限 → **区块无效**。
2. **唯一性 & 不可绕过**：

   * 每块**至多 1 笔 REAP**；
   * **非 REAP 交易**花费**过期 UTXO** → **区块无效**；
   * REAP 输入集合必须**等于**本高度选择器产出的前缀（确定性）。
3. **指数升级与再扫**：

   * `indexVersion = 2`（示例），含 `meta: tipHeightIndexed`；
   * 当版本不匹配或元数据异常：

     * 若 `--reindex-expiry`：全量重建；
     * 否则安全退出并提示参数。
4. **参数冻结点**：

   * 在 `chaincfg.Params` 标注**冻结**的共识参数清单（主网与测试网分别定义）：

     * `TaxRate{num,den}`、`MaxREAPInputsPerBlock`、`MaxReapTaxPerBlock`、`BurnPolicy`、`REAP_VERSION`。
   * 对默认日志输出这些参数值，启动时打印一次（便于审计）。

---

## 🧪 压力/模糊测试矩阵（4 组，合计 \~6–7h）

> 用 Testnet 上的 3 种子 + 本地 2 节点；必要时在私有子网复现。

### A. 小额 UTXO 洪水 + DUST 边界（\~2h）

* 生成 5–10 万笔接近 DUST 的 UTXO；推进高度至到期窗口。
* 观察：

  * 模板/验证不超时；REAP 每块处理量稳定；
  * **REAP 积压 < 3×MaxInputs**；
  * **拒绝**任何普通交易花费已过期 UTXO。
* 产出：处理速率曲线、积压曲线、CPU/内存峰值。

### B. 深度 reorg 回放（\~1.5h）

* 在私有矿工节点上构造 `+N` 区块的替代链（N=5..10），其中包含/不包含 REAP 的不同组合；
* 切换主分支，验证：

  * ExpiryIndex `Connect/Disconnect` 后与主链一致；
  * 无 “双 REAP”/“漏 REAP”；
  * coinbase 费用/税总额在最终链一致。

### C. 节点崩溃恢复 & I/O 干扰（\~1.5h）

* 在写入索引/打包模板期间 `kill -9`；重启后：

  * 不损坏 DB；`tipHeightIndexed` 正确；
  * 若有未完成批，能在下一个区块处理。
* 使用 `tc`/`ionice`/`cgroups` 限速磁盘与网络，确保**不崩溃**。

### D. P2P 扰动 & mempool 对抗（\~1h）

* 模拟大量无效 REAP 交易注入（应被 mempool 拒收，不传播）；
* 普通交易尝试花费过期 UTXO（节点直接拒绝）；
* 观察对同步速度与资源占用的影响。

---

## 📊 指标门槛（通过标准）

* **出块间隔中位数**：600s ±20%（近 288 块）
* **孤块率**：≤ 3%（短窗峰值≤5%）
* **REAP 覆盖率**：≥ 95%（到期后 N 块内被处理）
* **REAP 积压**：稳态 **< 3 × MaxREAPInputsPerBlock**
* **同步时长**：外部新节点 **< 2 小时** 同步到头
* **稳定性**：压力/故障注入中**无崩溃**、**无错误分叉**

---

## 🔐 可复现构建 & 签名（\~3–4h）

1. **锁依赖**：`go.mod` 固定版本到 commit SHA；`go env -w GOFLAGS=-trimpath`。
2. **容器化编译**：

   * 基础镜像：`golang:1.22.x`（显式 tag）+ `debian:bookworm-slim@sha256:…`（固定 digest）
   * 构建：`CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -buildid="`
3. **产物**：`btcd`、`btcwallet`、`obtc-status`（Linux/amd64、darwin/arm64、windows/amd64）。
4. **校验**：生成 `SHA256SUMS` 与 `SHA256SUMS.minisig`（`minisign -S -m SHA256SUMS`）。
5. **文档**：`docs/repro-build.md`：给出**逐步命令**，他人可在容器内重现相同哈希。

---

## 📄 发布说明（RC 草案，\~1h）

* **版本号**：`v0.1.0-rc1`（示例）
* **摘要**：OBTC 协议核心（到期税 30% 入矿工费用、系统交易、共识上限）
* **兼容性**：与 Bitcoin 网络完全隔离（魔数/端口/HRP/WIF/BIP32）
* **如何加入 Testnet**（二进制/Docker/参数）
* **校验**（SHA256 + minisign 步骤）
* **已知问题**与**降级开关**（排序 Simple 模式、禁用 BIP324、固定 DUST 等）

---

## 🕒 时间分配（≤ 20h）

| 任务                                   |        预估 |
| ------------------------------------ | --------: |
| 共识硬化（上限/唯一性/过期限制）                    |      4.0h |
| ExpiryIndex 版本/迁移/`--reindex-expiry` |      2.0h |
| 压力/模糊 A+B（小额洪水 + reorg）              |      3.0h |
| 故障注入 C（崩溃恢复 + I/O 限速）                |      1.5h |
| P2P/mempool 扰动 D                     |      1.0h |
| 可复现构建与签名                             |      3.0h |
| 文档：验证报告、Release Notes、repro-build    |      2.0h |
| 机动                                   |      3.5h |
| **合计**                               | **20.0h** |

---

## ✅ Go/No-Go 门槛（本周必须全部打勾）

* [ ] 共识硬化代码合入，单测/集成测试覆盖并通过
* [ ] 压测各项指标达标（见“指标门槛”）
* [ ] 崩溃/重启/重组用例均一致且无损
* [ ] `go.mod` 锁定、容器构建稳定、三平台产物哈希固定
* [ ] 生成并签名 `SHA256SUMS`，校验步骤文档齐全
* [ ] `release-notes-rc.md` & `testnet-join.md` 更新完毕

---

## 🧱 常见坑 & 规避

* **只在模板侧做上限** ⇒ 升级到共识检查，避免矿工“选择性截断”导致实现差异。
* **REAP 识别过宽** ⇒ 仅允许**固定版本 + 固定输出形态**；OP\_RETURN 长度与前缀严格校验。
* **指数升级破坏** ⇒ 默认安全退出，需显式 `--reindex-expiry` 参数才重建。
* **构建不可复现** ⇒ 未锁基础镜像/Go 版本/`-trimpath`；务必固定。
* **日志过噪/隐私** ⇒ 默认 INFO，禁止外向遥测，状态页仅汇总指标不含敏感数据。

---
