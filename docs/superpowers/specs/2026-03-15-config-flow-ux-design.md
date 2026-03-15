# 配置流程 UX 优化设计文档

**日期**: 2026-03-15
**状态**: 已批准（已修正 spec-review 问题）
**范围**: `dmxapi-claude-code.go`，仅修改 `main()` 函数中 configMode == 1/2 的流程逻辑

---

## 问题描述

### 问题 1：Team / VSCode 问询时机不合理

当前流程在**保存配置之后**才追问是否配置 Agent Teams 和 VSCode，属于"事后追问"模式，用户体验割裂。用户期望在进入模型配置前就决定好所有配置项。

### 问题 2：ESC / q 无法"保存退出"

模型选择 L1 菜单底部提示"q/Esc 保存退出"，但实际行为是：
- ESC/q 退出模型菜单
- 程序继续追问 Agent Teams 和 VSCode 配置

导致提示文字与实际行为不一致，用户无法快速完成并退出。

---

## 设计方案（方案 A：最小改动）

### 新流程（configMode == 1）

```
URL 配置
  ↓
Token 配置
  ↓
API 连接验证（循环直到成功）
  ↓
[新增] styledConfirm("是否同时配置 Agent Teams 功能") → 写入 wantTeams
[新增] styledConfirm("是否同时配置 VSCode 插件")      → 写入 wantVSCode
  ↓
configureModels(&cfg)
  按 ESC/q → 接受当前模型值，退出模型菜单（返回 main）
  ↓
saveConfig(cfg)
  ↓
if wantTeams  → configureAgentTeams(false)
if wantVSCode → configureVSCode(cfg, false)
  ↓
printSummary(cfg)
styledInput("按回车键退出")
```

### configMode == 2 变更说明

configMode == 2 为"仅配置模型"，本次**有意移除**其末尾的 Teams/VSCode 问询：

- **原行为**：模型配置 → 保存 → 问 Teams → 问 VSCode
- **新行为**：模型配置 → 保存 → 直接退出

理由：mode 2 语义为"仅配置模型"；Teams 和 VSCode 已有独立的 mode 4 和 mode 5，无需在 mode 2 中重复提供。

### ESC/q 行为修复原理

不需要修改任何菜单函数。通过流程重排，模型配置之后不再有任何追加问询，ESC/q 退出模型菜单后程序直接走到 `saveConfig` 然后退出，与"保存退出"提示完全一致。

**注意**：ESC/q 在 L2 菜单（二级模型选择菜单）中行为是"返回 L1"，不退出程序——这是正确行为，不受本次改动影响。

---

## 改动范围

### 变量声明（关键：必须在 main() 第一层作用域声明）

```go
func main() {
    ...
    cfg := loadExistingConfig()

    // 在所有 if-else 分支之前声明，确保后续代码可访问
    var wantTeams, wantVSCode bool

    if configMode == 1 {
        ...
        // API 验证成功后赋值
        wantTeams = styledConfirm("是否同时配置 Agent Teams 功能")
        wantVSCode = styledConfirm("是否同时配置 VSCode 插件")
        ...
    } else if ...
```

### 删除行

| 行号（约） | 内容 | 操作 |
|-----------|------|------|
| ~2279–2289 | 两个 `styledConfirm` 的追加问询及其 if 块 | 删除 |

### 新增行

| 位置 | 内容 |
|------|------|
| loadExistingConfig 调用之后 | `var wantTeams, wantVSCode bool` |
| API 验证成功后（约第 2238 行后） | 两个 styledConfirm，结果赋给 wantTeams/wantVSCode |
| saveConfig 调用之后 | `if wantTeams { configureAgentTeams(false) }` 和 `if wantVSCode { configureVSCode(cfg, false) }` |

---

## 验证标准

1. **configMode == 1 流程**：API 验证成功后立即显示 Agent Teams 和 VSCode 的问询，然后进入模型配置
2. **ESC/q 行为**：在模型 L1 菜单按 ESC 或 q，程序保存配置并退出（不再追问任何问题）
3. **ESC 在 L2 菜单**：在模型 L2 菜单按 ESC 或 q，返回到 L1 菜单（不退出程序）——现有行为保持不变
4. **configMode == 2**：仅执行模型配置 → 保存 → 退出，不再追问 Teams/VSCode
5. **configMode == 3/4/5**：流程完全不受影响
6. **wantTeams=true 时**：保存完成后自动进入 Agent Teams 配置流程
7. **wantVSCode=true 时**：保存完成后自动进入 VSCode 插件配置流程
