# Phase 5 计划（钱包续期 RPC & 自动续期）— btcwallet + btcd

> 修订（2026-02-11）：建议把 Week5 拆为 5A/5B，优先 5A（`getexpiry` + 手动 `renew` + 批量续期），自动续期与随机窗口放到 5B（可并入 Week6），降低跨仓联调风险。

**时间预算：16–18 小时** ｜ **目标**：实现钱包侧“到期感知 + 一键续期 + 自动续期”的 RPC 与 CLI，形成可验证的续期闭环（无 GUI）。

---

## 🎯 本周目标（Definition of Done）

* 钱包可查询每个 UTXO 的**到期高度/剩余区块/预计天数**与**风险提示**（DUST/即将到期）。
* 钱包提供 **`obtc.getexpiry`** 与 **`obtc.renew`** RPC；支持 **批量续期**。
* CLI 提供 `renew-all --before 180d` 一键续期入口。
* 自动续期（可选）支持：**到期前 6–12 个月随机窗口**内执行，支持“最大费率阈值”。
* 续期交易**默认转到新地址/新脚本**（隐私卫生），并记录映射/审计信息。
* 本地 regtest/simnet 端到端：**UTXO → 续期 → 新 UTXO** 成功率 ≥ 99%。

---

## 🧭 白皮书要点（本周落地）

来自《obtc_whitepaper_cn_v0》第 6 章（钱包 UX）：

1. **到期感知**：展示倒计时；距到期 < 180 天提示加重。
2. **一键续期**：把将到期 UTXO 发送到**新脚本**，重置 7 年计时。
3. **自动续期**：在**到期前 6–12 个月**随机窗口执行，避免同步行为暴露。
4. **尘埃预警**：若预测 70% 返还 < DUST，强提示用户续期或合并。
5. **返还脚本**：共识层返还到原脚本；钱包应自动“转发到新脚本”。

---

## 🗂️ 代码组织（建议）

> 说明：Week5 主要在 **btcwallet fork** 中实现；btcd 侧仅提供必要 RPC 或参数读取。

```
btcwallet/
  rpc/obtc/
    commands.go            # RPC 请求/响应结构
    server.go              # obtc.getexpiry / obtc.renew
  wallet/
    expiry.go              # UTXO 到期计算与标记
    renew.go               # 续期交易构造与提交
    policy.go              # 自动续期策略（随机窗/费率阈值）
  cmd/btcwallet/
    flags.go               # --autorenew / --renew-before / --maxfeerate
  cmd/btcctl/
    obtc_renew.go           # renew-all CLI
  docs/
    phase5-validation.md
```

---

## 🔧 核心功能设计

### 1) 到期感知（getexpiry）

**目标**：对钱包内 UTXO 输出“到期高度 + 倒计时 + 风险提示”。

**关键字段**：
- `outpoint` (txid:vout)
- `amount` (sats)
- `create_height`
- `expiry_height = create_height + WindowBlocks`
- `blocks_to_expiry`
- `days_to_expiry ≈ blocks_to_expiry / 144`
- `status`: `ok | expiring | expired`
- `dust_risk`: `true/false`（若 `floor(value * 0.7) < DUST`）

**DUST 预警**：
- 若未续期，未来 REAP 返还可能不足 DUST → `dust_risk=true`。
- 提示建议“合并/续期”。
- 注意当前 REAP dust 逻辑按**单输入**执行，存在阈值 cliff（如 778/779）与“TaxNum=0 仍可能因 dust 折叠产生有效损失”的语义；钱包侧展示请按实现口径解释。
- 详见：`obtc_doc/reap_dust_behavior.md`。

### 2) 一键续期（renew）

**目标**：把选定 UTXO 花费到**新地址/新脚本**，刷新计时。

**策略**：
- 输入：用户指定 `outpoint` / `addr` / `label` / `account` / `before_height`。
- 输出：默认 **新地址**（`getnewaddress`），可选“保持原脚本”。
- 费率：用户指定或使用钱包默认费率；支持 `max_feerate` 阈值。
- 聚合：允许“多输入合并为 1–N 输出”，减少 UTXO 碎片。
- 安全限制：
  - **拒绝**续期已过期 UTXO（避免无效交易）。
  - **锁定**选中的 UTXO，避免并发双花。

### 3) 自动续期（可选开关）

**逻辑**：
- 对每个 UTXO，计算 `expiry_height`。
- 当进入 **随机窗口** `[expiry - 12mo, expiry - 6mo]` 内，触发候选。
- 若当前费率 `<= max_feerate`，自动提交；否则延后。
- 自动续期可设置**每日预算**或**批量上限**。

---

## 📡 RPC 草案（建议）

### `obtc.getexpiry`

**请求参数**（可选）：
- `before_height` / `before_days`
- `min_amount` / `max_results`
- `include_change` / `include_locked`

**响应示例**：
```json
{
  "tip_height": 123456,
  "window_blocks": 368880,
  "items": [
    {
      "outpoint": "txid:vout",
      "amount": 100000,
      "create_height": 100000,
      "expiry_height": 3779200,
      "blocks_to_expiry": 1200,
      "days_to_expiry": 8,
      "status": "expiring",
      "dust_risk": false
    }
  ]
}
```

### `obtc.renew`

**请求**：
```json
{
  "outpoints": ["txid:vout"],
  "before_days": 180,
  "target_addr": "newaddr(optional)",
  "max_feerate": 5,
  "dry_run": false
}
```

**响应**：
```json
{
  "txid": "...",
  "inputs": 3,
  "outputs": 1,
  "fee": 1200,
  "renewed_total": 5200000
}
```

### CLI

```
btcctl wallet obtc.renew --before 180d
btcctl wallet obtc.getexpiry --before 90d
btcctl renew-all --before 180d --maxfeerate 5
```

---

## 🧪 测试计划

### 单元测试
- 到期计算（高度/剩余区块/天数）；
- DUST 预警边界；
- 续期选择逻辑（过滤已过期/锁定/太小输出）。

### 集成测试（regtest/simnet）
1. 造 UTXO → `getexpiry` 显示正确；
2. 推进高度进入 `expiring` → `renew` 成功生成新 UTXO；
3. 批量续期（100+ 输出），成功率 ≥ 99%；
4. 自动续期模拟（随机窗口 + 费率阈值）稳定触发。

---

## 📑 本周交付物（Deliverables）

* `btcwallet`：`obtc.getexpiry` / `obtc.renew` RPC
* `btcctl`：`renew-all` CLI
* 自动续期开关（`--autorenew`、`--renew-before`、`--maxfeerate`）
* `docs/phase5-validation.md`：续期批量测试报告（含 txid/高度）

---

## 🕒 时间分配（≤ 18h）

| 任务 | 预估 |
| --- | ---: |
| RPC 设计与结构体 | 2.0h |
| getexpiry 实现 | 3.0h |
| renew 构造与提交 | 5.0h |
| CLI + 自动续期策略 | 3.0h |
| 测试与验证 | 4.0h |
| **合计** | **17.0h** |

---

## 🧱 常见坑 & 规避

* **不记录 create_height** ⇒ 无法可靠计算到期高度；必须在钱包 DB 保存。
* **续期已过期 UTXO** ⇒ 交易无效；应强制拒绝。
* **批量续期费率过高** ⇒ 增加 `max_feerate` 限制。
* **输出仍回到原脚本** ⇒ 隐私退化；默认转发到新地址。
* **自动续期过于集中** ⇒ 随机窗口 + 每日上限。

