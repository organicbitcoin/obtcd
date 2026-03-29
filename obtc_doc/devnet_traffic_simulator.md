# DevNet 流量模拟器说明

本文档说明升级后的 OBTC DevNet 流量模拟器，核心组件包括：

- `scripts/devnet-up.sh`
- `cmd/devnetsim`
- `scripts/validation/devnet_sim_smoke.sh`

现在这套 devnet **默认跑的是 2 节点 `obtcregtest`**，不再只是普通 Bitcoin `simnet`。这样做的目的很明确：

> 让当前所有流量场景，在运行时都能顺手验证 **OBTC 特有逻辑**，而不是只验证交易数量和 mempool 压力。

这次纳入场景验证的 OBTC 特有逻辑包括：

- `expiryindex`
- `getexpirycommitment`
- `getreapplan`
- REAP 区块交易实际入块
- OBTC replay-protected sighash 激活后的签名要求

---

## 一、现在能模拟什么

当前 DevNet 已支持：

- **空块**
- **稀疏区块**：少量普通交易
- **密集区块**：大量独立交易
- **积压场景**：挖完一个块后 mempool 仍有 backlog
- **费率分层市场**：不同 fee band 的交易同时存在
- **冲突交易 / 双花尝试**：验证 mempool reject 行为
- **归集交易**：多输入 consolidation / sweep
- **依赖链交易**：未确认父子链
- **多钱包 / 多节点流量**：两个 deterministic wallet 分别经 node1 / node2 发流量
- **重启持久化验证**：stop / restart 后继续发流量
- **持续多块流量**：跨多个区块稳定注入交易
- **动态混合场景**：多种场景串起来一次跑

并且：

- **每个场景都运行在 OBTC 激活高度之后**
- **每个场景都会验证 OBTC 专有状态**

---

## 二、快速上手

### 1）先编译主节点

```bash
cd /Users/pengyu/src/obtcd
go build -o btcd
```

### 2）启动 OBTC DevNet

```bash
./scripts/devnet-up.sh start
```

启动时会自动：

- 启两个本地节点
- 启用 `--expiryindex`
- 把链高度推进到 OBTC expiry / REAP / replay protection 已激活的位置
- 自动检查最新区块里是否真的出现 REAP tx

### 3）查看状态

```bash
./scripts/devnet-up.sh status
```

状态输出现在会包含：

- 普通节点状态
- mempool 状态
- primary / peer simulator 状态
- `getexpiryindexstats`
- `getexpirycommitment`
- `getreapplan`

### 4）最短跑法

如果你只想最快验证我这版新增能力：

```bash
./scripts/devnet-up.sh start
./scripts/devnet-up.sh scenario multisource
./scripts/devnet-up.sh stop
```

### 5）完整 smoke

```bash
./scripts/validation/devnet_sim_smoke.sh
```

---

## 三、核心命令

### 生命周期

```bash
./scripts/devnet-up.sh start
./scripts/devnet-up.sh restart
./scripts/devnet-up.sh stop
./scripts/devnet-up.sh status
./scripts/devnet-up.sh logs
./scripts/devnet-up.sh clean
```

### 挖矿与 OBTC 校验

```bash
./scripts/devnet-up.sh mine 1
./scripts/devnet-up.sh mine 10
./scripts/devnet-up.sh miner on
./scripts/devnet-up.sh miner off
./scripts/devnet-up.sh validate-obtc
```

`validate-obtc` 会显式验证：

- expiry index 已启用
- expiry commitment 已 active
- reap plan 已 active
- 如果挖块前的 REAP 计划确实选中了输入，则当前最新区块里存在 REAP marker transaction

### UTXO 准备

#### primary wallet

```bash
./scripts/devnet-up.sh prepare 4000 300000
```

#### peer wallet

```bash
./scripts/devnet-up.sh prepare-peer 512 240000
```

`prepare-peer` 会：

1. 用 primary wallet 给 peer wallet 注资
2. 确认资金进链
3. 让 peer wallet 可通过 node2 发交易
4. 准备完成后顺手做一轮 OBTC 状态校验

### 流量注入

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

新增随机化参数：

- `--value-min` / `--value-max`：按区间随机每笔交易的收款金额
- `--randomize-inputs`：随机挑选可花 UTXO，而不是总按同一套顺序花
- `--rand-seed`：让随机流量可复现

---

## 四、支持的流量模式

### `simple`
最简单的单输出独立交易。

### `mixed`
更像真实钱包行为的混合模式，包含：

- 单输出支付
- 双输出 / 三输出拆分
- 自转账

### `chain`
构造依赖未确认输出的父子链交易。

### `consolidate`
构造多输入归集交易，用来模拟：

- 钱包清理 UTXO
- 更重的交易权重
- 多输入打包行为

### `feemarket`
按不同费率层注入交易，适合模拟：

- fee market 分层
- 矿工打包偏好
- mempool 压力行为

### `conflict`
每次尝试：

1. 先广播一笔有效交易
2. 再广播一笔花同一输入的冲突交易

预期：

- 第一笔 accepted
- 第二笔 rejected

---

## 五、预置场景（scenario）

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

### 场景说明

- `empty`：挖空块，但仍验证 OBTC block 状态
- `sparse`：少量普通交易后挖块
- `dense`：大量普通交易后挖块
- `backlog`：制造 backlog
- `feemarket`：制造 fee market 分层
- `conflict`：制造双花冲突尝试
- `consolidation`：制造多输入归集交易
- `multisource`：两个独立钱包经两个节点同时发流量
- `steady`：跨多个区块持续喂流量
- `dynamic`：多阶段混合场景

注意：

- 这些场景现在不只是“跑交易”
- **每次挖块后都会自动做 OBTC-specific validation**

---

## 六、为什么一定要切到 `obtcregtest`

因为 `expiryindex / REAP / expiry commitment / replay protection` 这些逻辑，本来就是挂在 **OBTC 网络参数** 上的。

如果还是普通 Bitcoin `simnet`：

- 可以模拟交易流量
- 但 **OBTC 特有逻辑不会真正激活**

所以现在这版 devnet 的核心变化是：

> 保留原来的流量场景集合，但把底层网络切到 **OBTC regtest**，这样现有所有场景都能真实带上 OBTC 逻辑验证。

---

## 七、这次修掉的一个关键问题：Replay Protection 签名

在把 devnet 切到 OBTC 激活高度之后，我实际跑 smoke 抓到了一个问题：

- `devnetsim` 生成的 legacy spend 仍然使用普通 `SigHashAll`
- 但 OBTC replay protection 激活后，mempool 要求交易使用 replay-protected sighash
- 所以交易会被拒绝

报错大意是：

- `missing OBTC replay-protected sighash bit`

修复方式：

- 当 replay protection 已激活时，simulator 自动改用：

```text
SigHashOBTCReplayProtection | SigHashAll
```

这样 DevNet 在 OBTC 激活高度之后仍然可以继续正常发流量。

---

## 八、关于 mirrored broadcast

实现多钱包双节点流量时，还发现一个本地 devnet 现象：

- 经 RPC 本地注入的交易，不总能稳定依赖原生 relay 自动汇合成同一个共享 mempool

为了让两边节点都能看到多源流量，并让矿工节点稳定打包 peer wallet 的交易，当前 `devnetsim` 使用了：

- **mirrored broadcast**

也就是：

- 经 node1 注入的交易，会镜像到 node2
- 经 node2 注入的交易，会镜像到 node1

这是当前本地 devnet 的工程化 workaround，不代表生产网络真实 relay 行为已经被完整模拟。

---

## 九、Smoke 现在验证什么

运行：

```bash
./scripts/validation/devnet_sim_smoke.sh
```

它会验证：

1. OBTC devnet 启动
2. 显式 `validate-obtc`
3. expiryindex / expirycommitment / reap plan 激活
4. primary wallet 准备
5. peer wallet 注资
6. 双节点 multi-source mempool
7. conflict reject
8. consolidation
9. 挖出包含 REAP tx 的非空区块
10. stop / restart 后继续发 peer 交易

当前通过时的典型结果：

```text
[smoke] PASS mempool_size=20 node2_mempool_size=20 mined_block_txs=28 reap_picked=1
```

---

## 十、你本地最短命令清单

### 最短跑完整新版能力

```bash
cd /Users/pengyu/src/obtcd
go build -o btcd

./scripts/devnet-up.sh start
./scripts/devnet-up.sh prepare 512 300000
./scripts/devnet-up.sh prepare-peer 256 240000
./scripts/devnet-up.sh spam --count 200 --mode feemarket --value 150000
./scripts/devnet-up.sh spam-peer --count 80 --mode mixed --value 110000
./scripts/devnet-up.sh mine 1
./scripts/devnet-up.sh status
```

### 直接跑多源场景

```bash
./scripts/devnet-up.sh scenario multisource
```

### 直接跑动态混合场景

```bash
./scripts/devnet-up.sh scenario dynamic
```

### 显式校验 OBTC 状态

```bash
./scripts/devnet-up.sh validate-obtc
```

### 验证 restart 能力

```bash
./scripts/devnet-up.sh stop
./scripts/devnet-up.sh restart
./scripts/devnet-up.sh spam-peer --count 3 --mode simple --value 90000
```

---

## 十一、当前仍未覆盖的限制

虽然已经比之前强很多，但还不是完整生产环境模拟。当前仍缺：

- network partition / reconnect
- 延迟传播 / 丢包 / 更复杂网络拓扑
- 更复杂 package relay / ancestor policy
- 更多钱包画像和 fee preference
- 长时间 soak / restart regression
- 不依赖 mirrored broadcast 的原生 relay 收敛验证

---

## 十二、建议的下一步

如果继续增强，我建议优先做：

1. `network partition / reconnect`
2. 更多钱包画像 + fee preference
3. long-running soak / restart regression

这三项对“更接近真实环境”的提升最大。
