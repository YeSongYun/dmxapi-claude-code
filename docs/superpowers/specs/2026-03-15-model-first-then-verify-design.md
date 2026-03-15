# 先配模型再验证 API 设计文档

**日期**: 2026-03-15
**状态**: 待实现
**范围**: `dmxapi-claude-code.go`，仅修改 `main()` 函数中 `configMode == 1` 的流程逻辑

---

## 问题描述

当前 `configMode == 1`（从头配置）流程：

```
配置 BaseURL → 配置 Token → API 验证(用 defaultModel 兜底) → 配置模型 → 保存
```

**问题：** API 连接验证在模型配置之前，验证所用模型是代码内硬编码的 `defaultModel` 常量（兜底），而非用户实际选择的模型。这导致验证结果与用户真实使用场景不完全一致。

---

## 设计方案

### 新流程（configMode == 1）

```
配置 BaseURL
  ↓
配置 Auth Token
  ↓
configureModels(&cfg)          ← 提前到 API 验证之前
  ↓
API 连接验证循环（使用 cfg.Model）
  ↓
styledConfirm Teams / VSCode
  ↓
saveConfig(cfg)
  ↓
if wantTeams  → configureAgentTeams(false)
if wantVSCode → configureVSCode(cfg, false)
  ↓
printSummary(cfg)
```

### 其他模式不受影响

- **configMode == 2**（仅配置模型）：保持原有行为，模型配置在 if/else 分支块之后。全新安装时 `cfg` 各字段为空，`configureModels` 内部会用 `defaultModel` 填充 `cfg.Model`，行为正常。
- **configMode == 3/4/5**：行为完全不变。

---

## 改动范围

> **⚠️ 重要：以下三处改动必须原子性实施。若只应用改动 3 而遗漏改动 1，mode 1 将完全跳过模型配置。**

### 改动 1：在 `configMode == 1` 分支中，Token 配置之后新增模型配置调用

位置：`main()` → `if configMode == 1` 分支，`getNewAuthToken` 调用之后

```go
// 配置 Auth Token
cfg.AuthToken = getNewAuthToken(cfg.AuthToken, hostname)

// [新增] 配置模型（在 API 验证前）
fmt.Println()
configureModels(&cfg)
```

### 改动 2：删除 `configMode == 1` 分支内部的 `cfg.Model` 兜底赋值块

位置：`main()` → `if configMode == 1` 分支内部，当前约第 2241-2243 行（`validateAPIConnection` 循环之前）

```go
// 删除这段（位于 if configMode == 1 { ... } 内部）：
if cfg.Model == "" {
    cfg.Model = defaultModel
}
```

**删除理由**：改动 1 已在此之前调用 `configureModels`，其内部（第 1857-1858 行）在调用 `runL1Menu` 前已用 `defaultModel` 填充非空值；且 `runL2Menu` 在自定义留空或 Esc 取消时均返回 `currentValue` 原值而非空字符串（第 1759-1760、1768 行）。因此 `configureModels` 返回后 `cfg.Model` 必然非空。

### 改动 3：将分支块之后的模型配置条件从 `!= 3` 改为 `== 2`

位置：`main()` 中大 if/else 分支块结束之后，当前约第 2308-2310 行

```go
// 旧：
if configMode != 3 {
    configureModels(&cfg)
}

// 新：
if configMode == 2 {
    configureModels(&cfg)
}
```

**完整覆盖说明**：
- mode 1：改动 1 已在分支内部配置，此处跳过，正确
- mode 2：此处执行 `configureModels`，正确
- mode 3：不配置模型，正确
- mode 4/5：在各自分支末尾 `return`，不到达此处，正确

---

## 验证失败时的修复菜单

验证失败时的 `selectFixOption()` 菜单中，"修改模型名"（case 4）**保留**。

**状态说明（预期行为）**：case 4 调用 `runL2Menu("默认模型", cfg.Model)`，仅修改 `cfg.Model` 单个字段，`cfg.HaikuModel`、`cfg.SonnetModel`、`cfg.OpusModel` 保持不变。这是预期行为：用户已在验证前完成了完整的模型配置，验证失败时只需临时换一个主模型重试。

验证循环结束后，新流程**不会再次调用** `configureModels`（改动 3 的效果）。用户在 case 4 中修改的主模型值将直接写入保存，这是有意为之的简化操作路径。

---

## 验证标准

1. **configMode == 1 流程**：Token 配置完成后立即显示模型配置菜单，模型配置完成后再进行 API 连接验证
2. **API 验证所用模型**：使用 `cfg.Model`（用户刚配置的值），不再出现空值兜底
3. **configMode == 2 流程**：仅执行模型配置 → 保存 → 退出，不受影响（含全新安装 cfg 全空的场景）
4. **configMode == 3/4/5**：流程完全不受影响
5. **验证失败修复菜单**：URL、Token、模型名三项修复选项全部正常工作；case 4 修改的仅为主模型字段
