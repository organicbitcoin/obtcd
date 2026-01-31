# 北极星 & 约束

* **北极星**：主网候选（Mainnet-Candidate）发布，连续出块≥72h、REAP 正常、钱包可续期、≥3 个地域种子节点可同步。
* **不做**：GUI、复杂 Explorer、策略层微调（除 DUST 最小改动）、花哨 DevOps。
* **分支**：`main`(上游镜像)、`obtc-main`(开发)、`release/*`(冻结)；共识改动全部参数化到 `IsOBTC()` 路径。
* **目录建议**：

  ```
  btcd/ (fork)
    blockchain/expiryindex/
    mining/reap/
    chaincfg/params_obtc.go
    cmd/obtc-status/     # 200行只读状态HTTP
  btcwallet/ (fork)
    rpc/obtc/            # getexpiry/renew
  scripts/ (build/run/release)
  docs/
  ```

---

# 周度计划（每周 ≤20h，含 10–15% 机动）

## 第 1 周｜基线与链参数（18–20h）

**产出**：能起 Devnet、能打包、能同步。

* Fork & 远端：2h（加 `upstream`、保护分支）
* `chaincfg/params_obtc.go`：4h（魔数、端口、HRP、WIF、BIP32 版本、默认种子占位）
* 创世占位 & 启动脚本：4h（`scripts/devnet-up.sh`、创世生成器草案）
* 关掉非必需：2h（先禁用 v2 传输/BIP324，可后续再开）
* CI & 多平台构建：4h（linux/amd64、darwin/arm64、win/amd64）
* 文档 README(一页)：2h
  **Cursor 用法**：跨仓全局重命名参数、脚手架生成 README/脚本。
  **验收**：两台节点本地互连出块、转账成功、`go test ./...` 绿。

## 第 2 周｜到期索引 ExpiryIndex（18–20h）

**产出**：索引能滚动维护、可查询。

* 设计与数据结构：2h（高度推进+键空间）
* 实现 `blockchain/expiryindex/`：8h（持久化、再扫、恢复）
* RPC：`listexpiring`：3h
* 单测+基准：5h（10k/100k UTXO 假数据；启动/恢复用例）
  **Cursor**：生成表驱动测试、错误路径枚举。
  **验收**：假 UTXO 能被正确索引；重启后索引不丢失。

## 第 3 周｜REAP 选择器 & 系统交易构造（19–20h）

**产出**：从索引选择到期 UTXO，构造 REAP 交易（未接入验证）。

* 确定性排序：6h（键=到期高度→金额→txid；每块上限）
* 交易构造器：8h（系统交易输出：30% 税入 coinbase 账户；余款丢弃/不再可用）
* 策略/共识边界：2h（排序&上限进共识；阈值参数化）
* 单测/属性测：3–4h（排序稳定性、去重）
  **Cursor**：跨文件抽取共识常量、生成属性测试骨架。
  **验收**：喂入索引→产出系统交易，哈希与排序可复现。

## 第 4 周｜验证规则 & 挖矿模板集成（18–20h）

**产出**：端到端 Devnet 自动 REAP 生效、税入 coinbase。

* `validation` 路径：8h（校验REAP结构/排序/上限、coinbase 增税不越界）
* `mining` 模板：6h（优先打包 REAP；与普通 tx 权衡最简）
* 端到端流：4h（构造将到期UTXO→自动被 REAP→coinbase 记账）
  **Go/No-Go 门槛 A**：本周末**必须**端到端 REAP 成功；否则启用**简化 REAP**（排序键去掉金额，只用 高度→txid），下周继续。

## 第 5 周｜钱包续期 RPC（16–18h）

**产出**：可查询到期、可一键续期（无 GUI）。

* `btcwallet`：`obtc.getexpiry`：4h（基于索引/查询层）
* `btcwallet`：`obtc.renew <outpoint|addr> [period]`：8h（新 UTXO 发送到自身；6–12 月随机窗）
* CLI：`renew-all --before 180d`：2h
* 测试：2–4h（批量续期、失败回退）
  **Cursor**：生成 RPC handler 模板、错误码对齐。
  **验收**：本地钱包批量续期成功率≥99%。

## 第 6 周｜加速 Testnet & 观测（18–20h）

**产出**：7年→7天加速网络上线，3 个地域种子。

* 参数开关（加速仅用于 Testnet）：2h
* 种子部署：8–10h（3 台云主机：美/欧/亚；`systemd`、UFW、日志轮转）
* 迷你状态页 `cmd/obtc-status/`：4h（区块头高度、mempool 计数、节点数）
* 稳定性冒烟：4h（同步<2h到头、间隔合理、REAP出现）
  **Go/No-Go 门槛 B**：新节点 2 小时内能全同步；否则先仅内部灰度、限制对外公告。

## 第 7 周｜硬化/压力/封版（18–20h）

**产出**：可发布候选。

* 压力/模糊：8h（小额洪水、DUST 边界、索引恢复、断电/重启）
* 资源限额：3h（REAP 每块硬上限、coinbase 税 cap）
* 依赖与构建锁定：3h（`go.mod` pin commits、容器化构建）
* 发布资产：4–6h（签名、校验步骤、快速接入文档、一键 `docker compose up obtc-fullnode`）
  **Cursor**：脚手架出 Dockerfile/compose、生成发布说明模板。
  **验收**：重复构建通过，外部按文档可起全节点并同步。

## 第 8 周｜Mainnet-Candidate（16–18h）

**产出**：主网候选发布 & 72h 观察。

* 冻结参数：4h（魔数/端口/HRP/WIF/BIP32/创世哈希、DNSSeeds）
* 生成&校验创世：4h（生成器+校验器；双人复核流程脚本）
* 发布二进制 & 指南：4h（下载、校验、最小运维、故障排查）
* 观察与热修：4–6h（72h 内仅接入必要修复，避免大改）
  **Go/No-Go 门槛 C**：连续出块≥72h、REAP 正常、三地域可同步、无>1 深度重组 → 宣布“主网候选上线”。

---

# 横向清单（全期并行的小任务）

* **安全/隔离**：确保地址 HRP、WIF、BIP32 版本不同于比特币主网，防重放与误存。
* **上限保护**：REAP 数量/体积硬上限；coinbase 税额 cap 防溢出。
* **日志最小化**：默认 INFO，无外向遥测；只保留必要指标。
* **差分测试**：固定一组区块/交易在上游 btcd 与分叉上跑对比（确保除你改动外一致）。
* **发布签名**：`minisign`/`age` 均可；文档写明校验步骤。

---

# 降级策略（提前写好）

* **A 点失败**（周4 REAP 未打通）：立即切换“简化 REAP 排序”（高度→txid），把复杂性留到 M2。
* **B 点失败**（周6 同步>2h）：先缩小 Testnet 目标区块大小/出块间隔，限制外部接入，优先稳定再公开。
* **C 点失败**（周8 观察不稳）：保持“候选”状态，不切参数；仅发热修并延长观察窗口。

---

# Issue 模板（复制即用）

* **Title**：\[模块] 简述
* **Context**：背景/场景
* **Acceptance**：明确可验证条件（如“新节点<2h全同步”“REAP 每块≤N 笔”）
* **Scope**：内外边界
* **Test**：单测/集成/冒烟步骤
* **Rollback**：回滚或关闭开关的方式
