# 配置摘要完善设计文档

**日期**: 2026-03-15
**状态**: 已批准（已修正 spec-review 问题）
**范围**: `dmxapi-claude-code.go`，仅修改 `printSummary` 函数

---

## 问题描述

当前配置摘要仅展示 `Config` struct 中的字段（BaseURL / AuthToken / 三个模型）以及固定常量 Disable Betas，共 7 行。

以下两项配置在流程中可被设置，但**未出现在摘要中**：

1. **Agent Teams**（`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`）：用户在 mode 1 或 mode 4 中可启用/禁用
2. **VSCode 插件**（`claude-code.environmentVariables` 写入 settings.json）：用户在 mode 1 或 mode 5 中可配置

---

## 设计方案

### 核心原则

- 显示**真实系统状态**，而非仅反映"本次流程选项"
- 不修改函数签名，不增加调用方负担
- 利用已有函数（`getEnvVar`、`getVSCodeSettingsPath`）保持一致性

### 函数签名

```go
// 不变
func printSummary(cfg Config)
```

### Agent Teams 状态检测

```go
agentTeamsVal := getEnvVar(envAgentTeams)
agentTeamsDisplay, agentTeamsColor := "未启用", colorWhite
if agentTeamsVal == "1" {
    agentTeamsDisplay, agentTeamsColor = "已启用", colorBrightGreen
}
```

- 读取当前进程环境变量（`os.Getenv` 的封装）
- 值约定：本工具写入值始终为 `"1"`（见 `configureAgentTeams` 第 1928 行）
- 时序保证：`configureAgentTeams()` 在写入文件后立即调用 `os.Setenv(envAgentTeams, "1")`（第 1938 行），因此 `printSummary` 读取当前进程 env 可正确反映本次配置结果
- 已启用 → 绿色显示；未启用 → 灰色显示

### VSCode 状态检测

使用 JSON 解析替代原始字节检测，精准检测 `"claude-code.environmentVariables"` 键是否存在：

```go
vscodeDisplay, vscodeColor := "未配置", colorWhite
if path, err := getVSCodeSettingsPath(); err == nil {
    if data, err := os.ReadFile(path); err == nil {
        var jsonSettings map[string]interface{}
        if err := json.Unmarshal(data, &jsonSettings); err == nil {
            if _, ok := jsonSettings["claude-code.environmentVariables"]; ok {
                vscodeDisplay, vscodeColor = "已配置", colorBrightGreen
            }
        }
        // JSON 解析失败：文件存在但格式异常，保留"未配置"
    }
    // 文件不存在：保留"未配置"
}
// getVSCodeSettingsPath 失败（无法确定路径）：保留"未配置"
```

说明：
- `mergeVSCodeSettings` 写入 settings.json 时使用平铺键名 `"claude-code.environmentVariables"`（第 748 行）
- JSON 解析方式比 `bytes.Contains` 精准，不会因注释或其他上下文误判
- 三种不可达场景（路径未知 / 文件不存在 / JSON 损坏）均显示"未配置"，不引入额外状态，避免对普通用户造成困惑

### 摘要表格追加两行

```go
lines := []string{
    makeRow("Base URL", cfg.BaseURL, colorBrightGreen),
    makeRow("Auth Token", maskToken(cfg.AuthToken), colorBrightYellow),
    makeRow("Model", cfg.Model, colorCyan),
    makeRow("Haiku Model", cfg.HaikuModel, colorCyan),
    makeRow("Sonnet Model", cfg.SonnetModel, colorCyan),
    makeRow("Opus Model", cfg.OpusModel, colorCyan),
    makeRow("Disable Betas", fixedDisableExperimentalBetas, colorMagenta),
    makeRow("Agent Teams", agentTeamsDisplay, agentTeamsColor),   // 新增
    makeRow("VSCode Plugin", vscodeDisplay, vscodeColor),         // 新增
}
```

---

## 预期效果

```
╔════════════════════════════════════════════════════════════╗
║                          配置摘要                          ║
╠════════════════════════════════════════════════════════════╣
║  Base URL      │ https://claude.yesongyun.com              ║
║  Auth Token    │ sk-2...199e                               ║
║  Model         │ claude-sonnet-4-6                         ║
║  Haiku Model   │ claude-haiku-4-5-20251001                 ║
║  Sonnet Model  │ claude-sonnet-4-6                         ║
║  Opus Model    │ claude-opus-4-6                           ║
║  Disable Betas │ 1                                         ║
║  Agent Teams   │ 已启用                                    ║
║  VSCode Plugin │ 已配置                                    ║
╚════════════════════════════════════════════════════════════╝
```

| 场景 | Agent Teams | VSCode Plugin |
|------|-------------|---------------|
| 两项都配置 | 已启用（绿） | 已配置（绿） |
| 只启用 Teams | 已启用（绿） | 未配置（灰） |
| 都未配置 | 未启用（灰） | 未配置（灰） |
| 路径/文件/JSON 异常 | — | 未配置（灰） |

---

## 改动范围

### 文件

| 文件 | 改动 |
|------|------|
| `dmxapi-claude-code.go` | 仅修改 `printSummary` 函数体 |

### 修改细节

| 位置 | 操作 |
|------|------|
| `printSummary` 函数体开头 | 新增 Agent Teams 和 VSCode 状态检测变量（约 12 行） |
| `lines := []string{...}` | 追加两个 `makeRow` 调用 |
| 调用点（`main()`） | **不变**，仍为 `printSummary(cfg)` |

### 无需改动

- `import` 中 `bytes`、`encoding/json`、`os` 均已存在
- `getVSCodeSettingsPath()`、`getEnvVar()`、`makeRow()` 均为已有函数
- 调用点（`main()` 第 2298 行）不变

---

## 验证标准

1. **Agent Teams 已启用时**（env `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`）：摘要显示绿色"已启用"
2. **Agent Teams 未启用时**（env 不存在或为空）：摘要显示灰色"未启用"
3. **本次配置了 Agent Teams 后**：由于 `configureAgentTeams()` 调用了 `os.Setenv`，摘要立即反映新状态
4. **VSCode settings.json 含 `claude-code.environmentVariables` 键时**：摘要显示绿色"已配置"
5. **VSCode settings.json 不存在 / 无该键 / JSON 损坏时**：摘要显示灰色"未配置"
6. **调用模式范围**：mode 1/2（`else` 分支）/3 均会走到 `printSummary(cfg)`（第 2298 行）；mode 4/5 在各自函数入口处直接 `return`，不调用
7. **函数签名不变**：调用点无需修改
