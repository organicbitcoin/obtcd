# Week 2 计划（到期 UTXO 索引：ExpiryIndex）— Go + btcd + Cursor

---

## 🎯 本周目标（Definition of Done）

* 在 `btcd/blockchain/expiryindex/` 实现 **ExpiryIndex**（含持久化、重启恢复、重组一致性）。
* 新增最小 RPC：`obtc.listExpiring`（节点侧）可按**区块高度/时间窗**列出即将到期的 UTXO。
* 端到端：在 **simnet** 上生成一批 UTXO，推进到“将到期”窗口，`obtc.listExpiring` 输出与预期一致。
* 单元测试（包含 **reorg 用例**）与基准测试通过；CI 绿。

---

## 🧭 设计要点（先拍板）

1. **到期语义**（本周实现两套、默认用高度，便于确定性）：

   * **高度模式（默认）**：`expiryHeight = createHeight + ExpiryWindowBlocks`。
   * **时间模式（可开关）**：`expiryMTP = createMTP + ExpiryWindowSeconds`，再映射到最接近高度用于调度。

   > Testnet/Devnet 通过 `Params` 设置**加速窗口**（例如“7 年 → 7 天”）。

2. **双向索引**（保证可删、可重放）：

   * **Fwd：OutPoint → ExpiryKey**（便于花费/续期时快速删除）
   * **Rev：ExpiryKey → *compressed list of OutPoints***（便于“按到期顺序”扫描）
   * **Meta：进度/版本**（`tipHeightIndexed`, `indexVersion`）

3. **确定性与轻量**：

   * `ExpiryKey` 采用 **8 字节 big-endian**（到期高度/时间戳），自然排序即为扫描顺序。
   * Rev 值内按 **(txid, vout)** 排序，保证多实现一致。
   * **不在索引里存金额**（避免双写/一致性复杂度）；需要金额时再查 UTXO 集（第 3–4 周优化如有必要再加缓存）。

4. **重组一致性**：

   * `ConnectBlock`：新生 UTXO → 插入；被花费 UTXO → 删除。
   * `DisconnectBlock`：反向操作。
   * 全部写入在单个数据库 **原子批**内完成。

---

## 🗂️ 目录/文件规划

```
btcd/
  blockchain/
    expiryindex/
      expiryindex.go        # 对外接口（Init、ConnectBlock、DisconnectBlock、ScanRange）
      buckets.go            # DB bucket 常量与升级
      encode.go             # OutPoint 编解码、ExpiryKey 序列化
      params.go             # Expiry 模式/窗口参数（从 chaincfg 读取）
      reorg_test.go         # 重组一致性测试
      expiryindex_test.go   # 单测/基准
  rpc/
    rpcserver.go            # 注册 obtc.listExpiring
    rpcwebsocket.go         # （如需）
  chaincfg/
    params_obtc.go          # Week1 骨架已建：补上 Expiry 参数占位（不切网络）
```

---

## 🔧 数据结构与编码（简化规范）

* **Buckets**

  * `bktExpiryMeta`：`tipHeightIndexed (u32 LE)`、`indexVersion (u16)`
  * `bktOutpoint2Expiry`：`key = outpoint(36B)` → `value = expiryKey(8B)`
  * `bktExpiry2Outpoints`：`key = expiryKey(8B)` → `value = varbytes{ (outpoint 36B) * N }`（内部按 `(txid asc, vout asc)` 排序）

* **Key 编码**

  ```text
  outpoint = txHash(32LE) || vout(u32 LE)
  expiryKey = u64 BE   // 高度或时间戳（模式由参数决定）
  ```

* **参数（从 chaincfg 读取）**

  ```go
  type ExpiryMode int { ByHeight, ByMTP }
  type ExpiryParams struct {
      Mode                 ExpiryMode
      WindowBlocks         uint64 // Dev/Test 默认使用
      WindowSeconds        uint64 // 主网可切换为时间
      ListBatchLimit       int    // 每次扫描上限，默认 10k
      StartScanHeight      int32  // 初始扫描起点
  }
  ```

---

## 📡 RPC 规格（节点侧，最小可用）

`obtc.listExpiring` —— 列出“在 \[fromKey, toKey] 范围内将到期”的 UTXO

* **请求**

  ```json
  {
    "from": 123450,           // 起始到期高度（或秒），为空则默认 tipHeight+1
    "horizon": 1440,          // 扫描窗口（高度/秒）
    "limit": 1000,            // 返回上限（硬上限<= ListBatchLimit）
    "mode": "height|time",    // 可选；默认跟随 ExpiryParams.Mode
    "includeSpent": false     // 可选；默认 false
  }
  ```
* **响应**

  ```json
  {
    "mode": "height",
    "from": 123450,
    "to": 124450,
    "count": 42,
    "items": [
      {"txid":"…","vout":0,"expiry":123456},
      ...
    ]
  }
  ```

> **说明**：不返回金额；第 3–4 周在矿工模板中再查 UTXO 集获取金额与脚本类型。

---

## ⛓️ 区块路径集成（本周仅索引，不改共识）

* 在 `blockchain.ProcessBlock` 流水线中（或对应钩子）调用：

  * `expiryIndex.ConnectBlock(block, view)`
  * `expiryIndex.DisconnectBlock(block, view)`
* **注意**：使用和 `txindex` 一样的数据库事务/批处理风格，确保**崩溃后一致**。

---

## ✅ 本周交付物（Deliverables）

* `expiryindex/` 目录与实现、单测 & 基准、`obtc.listExpiring` RPC。
* `docs/week2-validation.md`：测试步骤与输出（含 reorg 用例）。
* CI 增加 `./blockchain/expiryindex` 路径的测试与 `-race`。

---

## 🧪 测试计划（可直接跑）

1. **功能单测**

   * 新增 UTXO → 正确记录到期键；
   * 花费 UTXO → Outpoint 映射与 Expiry 桶均被清理；
   * 同一高度多笔 → 输出顺序按 `(txid, vout)` 升序；
   * 边界：空区块、同 tx 创建与花费（coinbase 除外）混合。

2. **重组测试（`reorg_test.go`）**

   * 构造 `A-B-C` 和 `A-B’-C’` 分叉，保证 **Connect/Disconnect** 成对；
   * 断言两侧收敛后索引一致（`tipHeightIndexed` 与桶内容完全相等）。

3. **性能基准（`BenchmarkConnectBlock_ExpiryIndex`）**

   * 10k/50k 假 UTXO 创建区块，`ConnectBlock` 耗时与内存占用；
   * 目标：10k 插入批 < 150ms（本地）。

4. **RPC 集成测试（simnet）**

   * 链上快速造一批 UTXO；推进到“将到期窗口”；
   * `obtc.listExpiring` 返回数量与 outpoint 集合符合预期；
   * `limit`、`horizon` 生效，分页行为正确。

---

## 🕒 时间分配（≤ 20h）

| 任务                                                      |        预估 |
| ------------------------------------------------------- | --------: |
| 设计落稿（模式/键空间/桶/重组策略）                                     |      1.5h |
| 代码：桶与编码（`buckets.go` / `encode.go`）                     |      2.0h |
| 代码：核心索引器（`expiryindex.go`：Init/Connect/Disconnect/Scan） |      6.0h |
| 代码：RPC（注册/参数校验/分页）                                      |      2.0h |
| 单测：功能/边界/重组                                             |      4.0h |
| 基准：10k/50k 场景                                           |      1.0h |
| 集成：接入 `ProcessBlock` 钩子 + 冒烟（simnet）                    |      2.0h |
| 文档：`docs/week2-validation.md`                           |      0.5h |
| **合计**                                                  | **19.0h** |

> **缓冲**：如实现顺利，预留 1h 做一次“崩溃恢复测试”（中途 `kill -9` 后重启校验一致性）。

---

## 🧱 常见坑 & 规避

* **崩溃一致性**：所有写入放进**单个 DB 批**；出错回滚。
* **重组缺口**：`DisconnectBlock` 必须使用原块内容“反向操作”，不要试图“从现状推导”。
* **键空间膨胀**：`Expiry2Outpoints` 的 value 使用**压缩 varbytes** 存储；对“大键”分页存储（>N个 outpoints 时按子页拆分，必要时第 3 周再做）。
* **时间/高度混用**：把两种模式封装在 `ExpiryParams.Mode` 下，RPC 返回 `mode` 字段，避免客户端误解。
* **金额诉求**：第 2 周先不存金额；第 3–4 周在挖矿模板处查询 UTXO 集，再决定是否为热路径加只读缓存。

---

## 🧰 Cursor 助攻清单（提效点）

* **批量骨架生成**：目录与文件头、接口注释、错误码枚举。
* **测试用例扩展**：基于 3–4 个核心场景自动扩展到边界组合。
* **基准脚手架**：快速生成 `testing.B` 基准模板与假数据生成器。
* **跨文件重构**：发生命名/签名调整时，自动替换/修正引用。
