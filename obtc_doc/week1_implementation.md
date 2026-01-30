# Week 1 计划（基线 & 链参数）— Go + btcd + Cursor

## 🎯 本周目标（Definition of Done）

* 本机 **2 个节点（simnet）** 启动、互连、**能连续出块并完成一笔转账**（记录 txid）。
* `go test ./...` 通过（允许临时跳过超长用例，并标注 TODO）。
* 建好 **OBTC 专用链参数骨架**（`params_obtc.go` + `Register()` + `IsOBTC()`），暂不切换，**第 3 周再启用**。
* 最小 CI（Linux）跑 **`go vet`**、**`go test -race`**、**构建**。
* README 一页：**快速起两节点**、**转账示例**、**参数表占位**。

---

## 📦 本周可交付物（Deliverables）

* `scripts/devnet-up.sh`：一键起 2 节点（simnet）脚本
* `chaincfg/params_obtc.go`：OBTC 参数骨架（含 `Register()`、`IsOBTC()`、TODO: 唯一常量占位）
* `.github/workflows/ci.yml`：最小 CI（Linux + vet + -race）
* `scripts/rebase-upstream.sh`：上游同步脚本
* `README.md`：一页快速上手
* 端到端验证记录：高度、出块日志、转账 **txid**、测试通过截图（可放到 `docs/`）

---

## 🗂️ 仓库结构建议

```
btcd/ (fork)
  chaincfg/
    params_obtc.go      # 本周创建骨架（不启用）
scripts/
  devnet-up.sh
  rebase-upstream.sh
.github/
  workflows/ci.yml
docs/
  week1-validation.md   # 出块/转账/测试的记录与截图
```

---

## 🧩 任务拆解与时间分配（≤ 20h）

### 1) Fork & 远端设置（2h）

* 添加 upstream、创建 `obtc-main`、开启分支保护（禁止直接推 main）。
* 写 `scripts/rebase-upstream.sh`（仅开发分支可 rebase；发布分支用 merge）。

```bash
git remote add upstream https://github.com/btcsuite/btcd.git
git fetch upstream
git checkout -b obtc-main
git push -u origin obtc-main
```

**验收**：`git remote -v` 正确、`obtc-main` 已受保护、脚本可运行。

---

### 2) OBTC 参数骨架（4–5h）

> 本周**不切换到新网络**，只**落骨架**，避免超时；第 3 周再替换创世与端口。

* 在 `chaincfg/params_obtc.go` 新建 **占位参数**、`init() { Register(&OBTCMainNetParams) }`。
* 定义 **唯一的** `wire.OBTCNet`（四字节魔数）**占位常量**（暂不启用），并补充 **WIF/BIP32/HRP** **TODO 注释**：**不得复用**比特币的版本字节与 xpub/xprv。
* 提供 `func IsOBTC(p *Params) bool { return p.Net == wire.OBTCNet }`。

> **注意**：仅写骨架，不影响现有网络行为；同时在 README 放一张“参数表占位”，下周/第 3 周填实并冻结。

**验收**：`go build` 通过；新增的 `Register()` 单测通过。

---

### 3) Devnet 启动脚本（simnet）（2–3h）

* 采用 **simnet**（btcd 自带回归网络），确保**最快起链**。
* 启动 2 节点，分开 datadir 与端口，强制一个连接另一个。

`scripts/devnet-up.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

# 清理旧数据
rm -rf ./devnet
mkdir -p ./devnet/node1 ./devnet/node2

# 启动节点1
./btcd --simnet --datadir=./devnet/node1 \
  --listen=127.0.0.1:18555 --rpclisten=127.0.0.1:18556 \
  --txindex --rpcuser=u --rpcpass=p &

sleep 1

# 启动节点2（连接节点1）
./btcd --simnet --datadir=./devnet/node2 \
  --listen=127.0.0.1:18557 --rpclisten=127.0.0.1:18558 \
  --connect=127.0.0.1:18555 --txindex --rpcuser=u --rpcpass=p &
```

**验收**：两个进程正常运行，日志显示握手与区块产生。

---

### 4) 端到端冒烟（出块 + 转账）（3–4h）

* 使用 `btcctl`（或 curl RPC）生成区块 & 转账，记录 **txid**。
* 收敛流程：

  1. 生成地址 `getnewaddress`（节点2）
  2. 在节点1 `sendtoaddress <addr> <amount>`
  3. `generate 1` 或等待出块
  4. 在节点2 `gettransaction <txid>` 确认

将命令与结果记录到 `docs/week1-validation.md`（高度、hash、txid）。

**验收**：出现至少 1 笔**已确认**转账，截图/日志齐全。

---

### 5) 最小 CI（Linux + vet + -race）（3–4h）

`.github/workflows/ci.yml`

```yaml
name: CI
on: [push, pull_request]
jobs:
  linux:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: { go-version: '1.22' }
    - run: go vet ./...
    - run: go test -race ./...
    - run: go build ./...
```

> Mac/Windows 矩阵留到后续周，避免本周被环境问题卡住。

**验收**：CI 绿；提交失败能在 PR 上看到原因。

---

### 6) 禁用非必需项（1h）

* **不引入编译标签**，保持默认 **P2P v1**；如仓库默认尝试 v2（BIP324），则在配置上 **关闭**。
* 仅当日志显示因 v2 导致握手失败时再落小改（保持最小化变动）。

**验收**：在默认设置下，节点能稳定互连与出块。

---

### 7) README（1–2h）

一页即可，包含：

* 项目简介（OBTC = btcd fork，**本周只在 simnet 验证**）
* 构建：`go 1.22+`，`go build`
* 快速开始：运行 `scripts/devnet-up.sh`、`btcctl` 转账示例
* **参数表占位**（Net 魔数、端口、HRP、WIF、BIP32 前缀 —— 标注“第 3 周冻结”）

**验收**：新人按文档可 10–15 分钟内复现本周验收步骤。

---

## ✅ 本周验收清单（可勾选）

* [ ] 两节点（simnet）启动与握手成功
* [ ] 连续出块 ≥ 10 个
* [ ] 完成 1 笔从节点1 → 节点2 的已确认转账（记录 txid）
* [ ] `go vet`、`go test -race`、`go build` 均通过（本地与 CI）
* [ ] `params_obtc.go` 骨架提交并通过单测（含 `Register()`、`IsOBTC()`）
* [ ] `docs/week1-validation.md` 填好高度、hash、txid、截图

---

## 🧭 风险与规避

* **花时间在自定义创世** → 本周避免；先用 **simnet** 起链达成目标。
* **WIF/BIP32/HRP 复用 BTC** → 本周只做占位，下周/第 3 周**生成唯一常量并冻结**。
* **CI 环境卡住** → 先跑 Linux；Mac/Win 推后。
* **P2P v2（BIP324）扰动** → 默认 v1 保守设置，后续再跟进。

---

## 🔜 Week 2 预告（到期索引 ExpiryIndex）

* 设计键空间与推进策略（按高度/时间窗）
* 持久化与恢复（含重组一致性）
* RPC：`listexpiring`
* 单元测试 + 基准（10k/100k 假 UTXO）

---

### 附：上游同步脚本（可选）

`scripts/rebase-upstream.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail
git fetch upstream
git checkout obtc-main
git rebase upstream/master
```

> 仅用于开发分支。若未来打 `release/*`，请改用 **merge**，保留历史。
