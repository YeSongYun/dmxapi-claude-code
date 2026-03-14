# 配置流程 UX 优化设计文档

**日期**: 2026-03-15
**状态**: 已批准
**范围**: `dmxapi-claude-code.go`，仅修改 `main()` 函数中 configMode == 1 的流程逻辑

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
[新增] styledConfirm("是否同时配置 Agent Teams 功能") → wantTeams bool
[新增] styledConfirm("是否同时配置 VSCode 插件")      → wantVSCode bool
  ↓
configureModels(&cfg)
  按 ESC/q → 接受当前模型值，退出模型菜单
  ↓
saveConfig(cfg)
  ↓
if wantTeams  → configureAgentTeams(false)
if wantVSCode → configureVSCode(cfg, false)
  ↓
printSummary(cfg)
styledInput("按回车键退出")
```

### ESC/q 行为修复原理

不需要修改任何菜单函数。通过流程重排，模型配置之后不再有任何追加问询，ESC/q 退出模型菜单后程序直接走到 `saveConfig` 然后退出，与"保存退出"提示完全一致。

---

## 改动范围

| 文件 | 行范围 | 改动类型 |
|------|--------|---------|
| `dmxapi-claude-code.go` | ~2238（API验证成功后） | 插入两个 styledConfirm，结果存入 wantTeams/wantVSCode |
| `dmxapi-claude-code.go` | ~2279–2289（saveConfig 后的追问） | 删除原追问，改为用 wantTeams/wantVSCode 条件判断 |

**不涉及改动**：
- 任何菜单渲染函数（renderL1Menu、renderL2Menu 等）
- 任何按键处理函数（readRawKey）
- configureAgentTeams / configureVSCode 函数签名或实现
- configMode 2/3/4/5 的流程

---

## 验证标准

1. configMode == 1 流程：API 验证成功后立即显示 Agent Teams 和 VSCode 的问询
2. 在模型菜单按 ESC 或 q：程序保存配置并退出（不再追问任何问题）
3. configMode 2/4/5 等其他模式：流程不受影响
