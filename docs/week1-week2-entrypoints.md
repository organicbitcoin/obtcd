# Week1/Week2 关键入口点索引

本文档用于快速定位 Week1/Week2 新增或核心逻辑入口，方便后续回顾与维护。

## Week1（基线 & 链参数）

### 1) OBTC 网络参数
- `chaincfg/params_obtc.go`
  - OBTC 网络参数（主网/测试网/回归网）与 Expiry 参数骨架
  - Register/IsOBTC/ForkHeight 相关逻辑

### 2) 开发与上游同步脚本
- `scripts/devnet-up.sh`
  - 本地双节点启动脚本（simnet）
- `scripts/rebase-upstream.sh`
  - 上游同步脚本

### 3) 验证记录
- `docs/week1-validation.md`
  - Week1 端到端验证记录（出块/转账等）

---

## Week2（ExpiryIndex + RPC + 验证工具）

### 1) ExpiryIndex 核心实现
- `blockchain/expiryindex/expiryindex.go`
  - 索引器入口与核心逻辑（Connect/Disconnect/Scan）
- `blockchain/expiryindex/buckets.go`
  - DB bucket 定义与版本元数据
- `blockchain/expiryindex/encode.go`
  - OutPoint/ExpiryKey 编解码
- `blockchain/expiryindex/params.go`
  - Expiry 参数装配
- `blockchain/expiryindex/*_test.go`
  - 单测/重组/基准等

### 2) RPC 接口入口
- `rpcserver.go`
  - `handleListExpiring`
  - `handleGetExpiryIndexStats`
- `btcjson/obtcextcmds.go`
  - RPC 命令定义（listexpiring/getexpiryindexstats）
- `btcjson/obtcextresults.go`
  - RPC 返回结构定义
- `rpcserverhelp.go`
  - RPC 帮助文本

### 3) OBTC 网络启动入口（新增 flags）
- `config.go`
  - 新增 `--obtcmainnet/--obtctestnet/--obtcregtest`
- `params.go`
  - 新增 `obtcMainNetParams/obtcTestNetParams/obtcRegTestParams`
- `cmd/btcctl/config.go`
  - btcctl 对应网络 flag 与默认 RPC 端口

### 4) 验证工具与脚本
- `scripts/validation/utxo_expiry_validator.go`
  - 完整验证工具（参数校验/分页/边界/压力/基准）
- `scripts/validation/quick_validate.sh`
  - 一键验证入口
- `scripts/validation/demo.sh`
  - 演示脚本
- `scripts/validation/README.md`
  - 使用说明
- `scripts/validation/config_examples.conf`
  - 配置样例

### 5) Week2 验证记录
- `docs/week2-validation.md`
  - 本地验证记录
- `docs/week2-summary.md`
  - Week2 设计与实现总结

---

## 推荐阅读顺序
1. `chaincfg/params_obtc.go`
2. `blockchain/expiryindex/expiryindex.go`
3. `rpcserver.go`（listexpiring/getexpiryindexstats）
4. `scripts/validation/README.md` + `utxo_expiry_validator.go`

