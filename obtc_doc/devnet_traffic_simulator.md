# DevNet 流量模拟器与 Web Dashboard 说明

本文档基于当前仓库里的最新实现，说明本地 OBTC DevNet、流量模拟器和 `obtc-status --devnet` Web Dashboard 的实际能力与使用方式。

核心组件包括：

- `scripts/devnet-up.sh`
- `cmd/devnetsim`
- `scripts/validation/devnet_sim_smoke.sh`
- `cmd/obtc-status`

---

## 一、当前这套 DevNet 是什么

现在这套 DevNet 默认不是旧的 2 节点 `simnet`，而是：

- **网络**：`obtcregtest`
- **节点数**：默认 `3` 个节点，可配置为 `2` 到 `5`
- **节点角色**：
  - `node1`：`miner`
  - `node2`：`peer`
  - `node3+`：`relay`
- **数据目录**：`./devnet-data`
- **manifest**：`./devnet-data/manifest.json`

目标不是只做“发交易、看 mempool 数量”的简单压测，而是让本地流量场景在运行时也顺手覆盖 OBTC 特有逻辑，包括：

- `expiryindex`
- `getexpirycommitment`
- `getreapplan`
- REAP marker transaction 实际入块
- replay-protected sighash 激活后的交易签名要求

---

## 二、现在能做什么

当前 DevNet 已经支持：

- **空块 / 稀疏块 / 密集块**
- **mempool backlog**
- **fee market 分层流量**
- **冲突交易 / 双花尝试**
- **归集交易**
- **父子依赖链交易**
- **primary / peer 双钱包、多节点流量**
- **stop / restart 后恢复节点和 simulator 状态**
- **跨多个区块连续喂流量**
- **动态混合场景**
- **2 到 5 节点的本地拓扑**
- **Web Dashboard 直接触发常用动作**
- **按节点浏览最近区块和单块 JSON**

并且：

- `start` / `restart` 后会自动把链推进到 OBTC 相关逻辑已激活的高度
- `status`、`mine`、`prepare-peer` 等路径会串上 OBTC 状态验证
- Dashboard 不只是只读页，在 `--devnet` 模式下已经是一个本地操作台

---

## 三、快速上手

### 1. 编译二进制

```bash
cd /Users/pengyu/src/obtcd
go build -o btcd
go build ./cmd/obtc-status
```

### 2. 启动 DevNet

```bash
./scripts/devnet-up.sh start
```

启动时会自动：

- 构建 `btcctl` 和 `devnetsim`
- 启动 `3` 个本地节点
- 生成 `devnet-data/manifest.json`
- 将链推进到 `OBTC_BOOTSTRAP_HEIGHT=145`
- 检查每个节点的 OBTC 专有状态

### 3. 启动 Web Dashboard

```bash
./cmd/obtc-status/obtc-status --devnet --listen 127.0.0.1:9680
```

然后访问：

- `http://127.0.0.1:9680/`

在 `--devnet` 模式下，`obtc-status` 会默认：

- 使用 `obtcregtest`
- 使用 `--notls`
- 默认 RPC 账号密码为 `obtc / obtcpass`
- 读取 `./devnet-data/manifest.json`
- 调用 `./scripts/devnet-up.sh`

### 4. 最短验证路径

```bash
./scripts/devnet-up.sh scenario multisource
./scripts/devnet-up.sh status
```

### 5. 完整 smoke

```bash
./scripts/validation/devnet_sim_smoke.sh
```

---

## 四、环境变量与默认端口

`scripts/devnet-up.sh` 目前暴露的关键环境变量：

- `DEVNET_NODE_COUNT`
  - 默认 `3`
  - 范围 `2..5`
- `DEVNET_NETWORK`
  - 默认 `obtcregtest`
- `DEVNET_RPC_BASE_PORT`
  - 默认 `18556`
- `DEVNET_P2P_BASE_PORT`
  - 默认 `19555`

因此默认端口布局会是：

- `node1 RPC`：`127.0.0.1:18556`
- `node2 RPC`：`127.0.0.1:18557`
- `node3 RPC`：`127.0.0.1:18558`
- `node1 P2P`：`127.0.0.1:19555`
- `node2 P2P`：`127.0.0.1:19556`
- `node3 P2P`：`127.0.0.1:19557`

例如：

```bash
DEVNET_NODE_COUNT=5 ./scripts/devnet-up.sh start
```

---

## 五、CLI 核心命令

### 1. 生命周期

```bash
./scripts/devnet-up.sh start
./scripts/devnet-up.sh restart
./scripts/devnet-up.sh stop
./scripts/devnet-up.sh clean
./scripts/devnet-up.sh status
./scripts/devnet-up.sh logs
./scripts/devnet-up.sh mempool
./scripts/devnet-up.sh help
```

说明：

- `start`：创建全新的 `devnet-data/`
- `restart`：基于已有节点数据和 simulator 状态恢复
- `stop`：停进程，但保留数据
- `clean`：停进程并删除全部本地 DevNet 数据
- `status`：列出全部节点、钱包、mempool 与 OBTC 状态
- `logs`：输出每个节点最近日志
- `mempool`：逐节点查看 mempool 信息

### 2. 挖矿与校验

```bash
./scripts/devnet-up.sh mine 1
./scripts/devnet-up.sh mine 10
./scripts/devnet-up.sh miner on
./scripts/devnet-up.sh miner off
./scripts/devnet-up.sh validate-obtc
./scripts/devnet-up.sh audit-replay
./scripts/devnet-up.sh demo
```

说明：

- `mine n`：在 `node1` 上挖 `n` 个区块
- `miner on|off`：切换 `node1` 连续 CPU 挖矿
- `validate-obtc`：手工触发一次全节点 OBTC 状态校验
- `audit-replay`：逐块重放当前链并校验 REAP / expiry / tax / refund 等共识条件
- `demo`：`scenario dynamic` 的别名

`validate-obtc` 关注的重点包括：

- `expiryindex` 是否启用
- `expiry commitment` 是否 active
- `REAP plan` 是否 active
- 每个节点高度是否已达到 bootstrap 高度之后

### 3. UTXO 准备

#### primary wallet

```bash
./scripts/devnet-up.sh prepare 512 300000
```

#### peer wallet

```bash
./scripts/devnet-up.sh prepare-peer 256 300000
```

注意：

- CLI 中 `prepare-peer` 的默认参数是 `256 300000`
- Dashboard 的快捷按钮当前预设是 `256 240000`

`prepare-peer` 的流程是：

1. primary wallet 给 peer wallet 注资
2. 资金确认进链
3. peer wallet 可以通过 `node2` 继续发交易
4. 末尾自动补一轮 OBTC 状态校验

### 4. 流量注入

#### primary wallet

```bash
./scripts/devnet-up.sh spam --count 500 --mode mixed --value 150000
./scripts/devnet-up.sh spam --count 800 --mode feemarket --value 150000 --pace-ms 10
./scripts/devnet-up.sh spam --count 600 --mode mixed --value-min 80000 --value-max 180000 --randomize-inputs --rand-seed 42
./scripts/devnet-up.sh spam --count 60 --mode conflict
./scripts/devnet-up.sh spam --count 40 --mode consolidate
```

#### peer wallet

```bash
./scripts/devnet-up.sh spam-peer --count 120 --mode mixed --value 110000
./scripts/devnet-up.sh spam-peer --count 60 --mode chain --value 90000
```

常用参数：

- `--count`
- `--mode`
- `--value`
- `--fee-rate`
- `--prepare`
- `--prepare-value`
- `--pace-ms`
- `--value-min`
- `--value-max`
- `--randomize-inputs`
- `--rand-seed`

新增随机化能力：

- `--value-min` / `--value-max`
  - 在区间内随机生成每笔交易金额
- `--randomize-inputs`
  - 不按固定顺序挑 UTXO
- `--rand-seed`
  - 保持随机流量可复现

---

## 六、支持的流量模式

### `simple`

最直接的普通转账流量，适合快速堆交易数量。

### `mixed`

更接近日常钱包行为的混合流量，包含：

- 单输出支付
- 双输出 / 三输出拆分
- 自转账

### `chain`

构造父子依赖链，观察未确认祖先/后代行为。

### `consolidate`

模拟钱包归集零散 UTXO 的多输入交易。

### `feemarket`

制造不同费率层，观察拥堵下的打包偏好。

### `conflict`

每次会尝试：

1. 先广播一笔有效交易
2. 再广播一笔花同一输入的冲突交易

预期结果：

- 第一笔进入 mempool
- 第二笔被拒绝

---

## 七、预置场景

```bash
./scripts/devnet-up.sh scenario empty
./scripts/devnet-up.sh scenario sparse
./scripts/devnet-up.sh scenario dense
./scripts/devnet-up.sh scenario backlog
./scripts/devnet-up.sh scenario feemarket
./scripts/devnet-up.sh scenario conflict
./scripts/devnet-up.sh scenario consolidation
./scripts/devnet-up.sh scenario multisource
./scripts/devnet-up.sh scenario steady
./scripts/devnet-up.sh scenario dynamic
```

场景说明：

- `empty`：挖一个空块
- `sparse`：少量普通交易后挖块
- `dense`：大量普通交易后挖块
- `backlog`：制造挖完块仍然残留的 backlog
- `feemarket`：制造 fee band 分层
- `conflict`：制造双花冲突尝试
- `consolidation`：制造多输入归集交易
- `multisource`：primary + peer 两个钱包同时发流量
- `steady`：跨多个区块持续注入流量
- `dynamic`：空块、稀疏块、混合流量、fee market、冲突、链交易按阶段串起来

其中：

- `multisource` 目前是最直观的双钱包、多节点演示
- `dynamic` 是最接近“端到端功能巡检”的一键场景
- `demo` 直接等价于 `scenario dynamic`

---

## 八、Web Dashboard 现在提供什么

### 1. 启动方式

```bash
./cmd/obtc-status/obtc-status --devnet --listen 127.0.0.1:9680
```

默认端点：

- `/`
- `/status`
- `/healthz`
- `/blocks`
- `/block`

### 2. 首页 Dashboard

首页会展示：

- `Network`
- `Healthy Nodes / Configured Nodes`
- `Best Height`
- `Total Mempool`
- 每个节点的链高、peer 数、mempool 数量
- `ExpiryIndex / ExpiryCommitment / REAP Picked`
- 最近一次 Dashboard 动作及命令输出

### 3. 快捷动作按钮

当前首页内置按钮包括：

- `Start`
- `Stop`
- `Restart`
- `Mine 1`
- `Validate`
- `Prepare`
- `Prepare Peer`
- `Scenario Dynamic`
- `Scenario Multisource`

这些按钮会直接调用本地 `scripts/devnet-up.sh`。

限制：

- 只允许 **loopback client** 触发动作
- 也就是说，操作按钮只给本机浏览器或本机反代后的回环访问开放

### 4. 自定义挖块与流量注入

Dashboard 还支持两个自定义表单：

#### 自定义挖块

- 输入区块数量
- 调用 `mine <n>`

#### 自定义流量注入

基础字段：

- `target`
  - `primary`
  - `peer`
- `mode`
- `count`
- `value`

高级字段：

- `fee_rate`
- `prepare`
- `prepare_value`
- `pace_ms`
- `value_min`
- `value_max`
- `rand_seed`
- `randomize_inputs`

这意味着现在很多常见本地调试已经不必手敲整条 `spam` / `spam-peer` 命令。

### 5. 区块列表页与单块查看器

#### `/blocks`

区块列表页支持：

- 选择节点
- 指定最近区块数量
- 在 `Raw` / `RPC` 两种视图之间切换

默认：

- 默认展示最近 `20` 个区块
- 最大支持 `50` 个区块

#### `/block`

单块查看器支持：

- 指定节点
- 按区块高度查
- 按区块 hash 查
- 切换 `Raw` / `RPC` 视图

典型用途：

- 快速查看最新 best block
- 直接检查 REAP marker 所在块的 JSON
- 对比 raw block 解码视图和 RPC 派生字段视图

---

## 九、Smoke 当前覆盖什么

运行：

```bash
./scripts/validation/devnet_sim_smoke.sh
```

当前 smoke 的核心关注点包括：

1. DevNet 正常启动
2. `validate-obtc` 成功
3. `expiryindex / expirycommitment / reap plan` 激活
4. primary wallet 预热
5. peer wallet 注资
6. 双节点 multi-source mempool
7. conflict reject
8. consolidation
9. 挖出带 REAP marker 的区块
10. `stop / restart` 后继续发流量
11. 末尾追加一次逐块 replay audit，避免只靠页面或单次 RPC 结果判断

---

## 十、工程实现上的几个关键点

### 1. Web Dashboard 依赖 manifest

`start` 和 `restart` 会生成：

- `./devnet-data/manifest.json`

Dashboard 会优先读这个文件来了解：

- 当前网络
- 节点数量
- 节点名字与角色
- 每个节点的 RPC / P2P 地址

### 2. simulator 状态是持久化的

当前会保存：

- `./devnet-data/devnetsim/state.json`
- `./devnet-data/devnetsim/peer-state.json`

因此 `restart` 不只是重启节点，也会把 primary / peer wallet 的模拟状态一起续上。

### 3. mirrored broadcast 仍然存在

当前 `devnetsim` 在多钱包 / 多节点模式下仍然保留了 mirrored broadcast 的工程化处理：

- primary 方向发出的交易会镜像到 peer 侧
- peer 方向发出的交易会镜像到 primary 侧

目的是让本地 DevNet 更稳定地收敛到预期的共享交易视图，便于验证场景和挖矿结果。它是本地测试工程手段，不等于生产网络的原生 relay 已被完整模拟。

### 4. 共识层正确，不代表本地工具层已经适配完成

这次排查里需要特别区分两类问题：

- 共识层负责拒绝违规区块和违规交易
- 本地钱包 / 流量工具负责别去构造明知会被拒绝的交易

以 OBTC expiry 为例：

- 链上规则本来就会拒绝 `non-REAP transaction spends expired utxo`
- 但如果本地工具的选币逻辑没有把 expired UTXO 排除掉，`prepare` 或 `spam` 仍然会自己构造坏交易，然后在 mempool 侧失败

所以网页里“节点状态正常”并不等于“本地测试工具没有偏离共识规则”。

### 5. 本地钱包现在也必须显式适配 expiry 和 replay protection

当前仓库里两条已经修过的本地钱包路径是：

- `cmd/devnetsim`
- `integration/rpctest/memwallet`

它们现在都遵循同一组约束：

- expired 但尚未被 REAP 清走的 confirmed UTXO，不再计入 spendable balance
- 选币时不会再把这类 UTXO 当成候选输入
- OBTC replay protection 激活后，签名会自动带 `SigHashOBTCReplayProtection | SigHashAll`

这也是为什么现在大规模 `prepare`、`prepare-peer` 和 rpctest 本地钱包场景都能继续跑通。

### 6. Dashboard 适合观察，Replay Audit 适合做强校验

`obtc-status --devnet` 的网页适合：

- 看链高、mempool、REAP picked、最近区块
- 直接触发 `prepare`、`mine`、`scenario`
- 人工检查 REAP block、expiryindex 页面和 block JSON

但它不是完整的逐块审计器。更强的校验路径是：

```bash
./scripts/devnet-up.sh audit-replay
```

这条命令会重放整条当前链，并额外检查：

- 区块链接关系和 coinbase 位置
- coinbase maturity
- 非 REAP 交易不能花 expired UTXO
- REAP 交易不能花 live UTXO
- REAP marker 的 `height/count/digest`
- refund / tax 与链上输出是否一致
- deterministic REAP selection 是否符合当前实现

如果你要验证“网页看起来没问题，但底层是否真的满足共识要求”，优先跑 replay audit，而不是只刷新 Dashboard。

---

## 十一、推荐的本地组合

### 1. 最常用的 CLI + Dashboard 组合

```bash
cd /Users/pengyu/src/obtcd
go build -o btcd
go build ./cmd/obtc-status

./scripts/devnet-up.sh start
./cmd/obtc-status/obtc-status --devnet --listen 127.0.0.1:9680
```

随后：

1. 浏览器打开 `http://127.0.0.1:9680/`
2. 先点 `Scenario Multisource` 或 `Scenario Dynamic`
3. 再去 `/blocks` 或 `/block` 看最新区块内容

### 2. 最短的手工命令集

```bash
./scripts/devnet-up.sh prepare 512 300000
./scripts/devnet-up.sh prepare-peer 256 300000
./scripts/devnet-up.sh spam --count 200 --mode feemarket --value 150000
./scripts/devnet-up.sh spam-peer --count 80 --mode mixed --value 110000
./scripts/devnet-up.sh mine 1
./scripts/devnet-up.sh status
```

### 3. 五节点拓扑验证

```bash
DEVNET_NODE_COUNT=5 ./scripts/devnet-up.sh start
./cmd/obtc-status/obtc-status --devnet --devnet-nodes 5
```

---

## 十二、当前仍未覆盖的限制

虽然这套本地环境已经比早期版本强很多，但依然不是完整生产网络模拟。当前仍未覆盖或只做了近似覆盖的点包括：

- network partition / reconnect
- 更真实的网络延迟、丢包、乱序传播
- 不依赖 mirrored broadcast 的完整原生 relay 收敛
- 更复杂的 package relay / ancestor policy
- 更多钱包画像与 fee preference
- 更长时间的 soak / restart regression

如果继续增强，优先级最高的方向仍然是：

1. network partition / reconnect
2. 更多钱包画像和 fee preference
3. 长时间 soak / restart regression
