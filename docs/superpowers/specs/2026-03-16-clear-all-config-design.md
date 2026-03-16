# 清除所有配置功能设计

## 概述

为 dmxapi-claude-code 工具新增"清除所有配置"功能，作为配置模式菜单的第 6 个选项，允许用户一键清除该工具写入的所有环境变量配置。

## 背景

当前工具将配置以环境变量形式写入多个位置：

| 位置 | 平台 | 写入方式 |
|------|------|----------|
| Shell 配置文件 (`~/.zshrc` 等) | macOS / Linux | `export KEY='VALUE'` |
| fish 配置文件 (`~/.config/fish/config.fish`) | macOS / Linux (fish) | `set -Ux KEY 'VALUE'` |
| Windows 注册表 (`HKCU\Environment`) | Windows | `setx` / `REG ADD` |
| VSCode settings.json | 全平台 | `claudeCode.environmentVariables` JSON 键 |

涉及的环境变量分为两类：

**始终写入的 7 个变量**（由 `saveConfig()` 写入）：

1. `ANTHROPIC_BASE_URL` — API 服务器地址
2. `ANTHROPIC_AUTH_TOKEN` — 认证令牌
3. `ANTHROPIC_MODEL` — 默认模型
4. `ANTHROPIC_DEFAULT_HAIKU_MODEL` — Haiku 模型
5. `ANTHROPIC_DEFAULT_SONNET_MODEL` — Sonnet 模型
6. `ANTHROPIC_DEFAULT_OPUS_MODEL` — Opus 模型
7. `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` — 禁用实验性 Beta

**条件写入的 1 个变量**（由 `configureAgentTeams()` 单独写入，仅在用户启用时存在）：

8. `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` — Agent Teams 开关

清除时需尝试移除全部 8 个变量，无论其是否存在。

目前没有清除功能，用户需要手动编辑多个文件才能移除配置，体验不佳。

## 用户入口

在现有 5 个配置模式菜单后新增第 6 项：

```
请选择配置模式：
  1. 从头配置（API地址 + 密钥 + 模型）
  2. 仅配置模型
  3. 解决 400 报错
  4. 配置实验性功能
  5. 配置 VSCode 插件
  6. 清除所有配置
```

模式 6 在 `main()` 中使用 **early return**（与模式 4、5 一致），不进入后续的 `loadExistingConfig()` / `saveConfig()` 流程。

## 交互流程

```
用户选择 6
    │
    ▼
显示清除摘要：
  "将从以下位置清除所有 dmxapi 相关配置："
  "  • Shell 配置文件 (~/.zshrc)"
  "  • VSCode settings.json (如果存在)"
  "  • 当前进程环境变量"
  "涉及的环境变量：ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, ..."
    │
    ▼
二次确认："确定要清除所有配置吗？此操作不可撤销。(y/n)"
    │
    ├─ n → 返回主菜单
    │
    ▼ y
按位置逐一执行清除：
  1. Shell 配置文件（移除 export / set -Ux 行）
  2. VSCode settings.json（移除相关键，保留其他用户配置）
  3. Windows 注册表（如适用，删除环境变量）
  4. 当前进程环境变量（os.Unsetenv）
    │
    ▼
显示结果报告（实际移除数量动态统计）：
  "✓ 已从 ~/.zshrc 中移除 7 个环境变量"
  "✓ 已从 VSCode settings.json 中移除配置"
  "✓ 已清除当前进程环境变量"
  ""
  "配置已全部清除。重新打开终端后生效。"
```

## 实现策略：复用现有函数

代码库中已有单个变量的移除函数，**必须复用**而非重新实现：

- `removeEnvVarUnix(key string)` — 从 shell 配置文件中移除单个环境变量
- `removeEnvVarWindows(key string)` — 从 Windows 注册表中移除单个环境变量

清除功能通过循环调用这些函数实现，确保行为一致性。

## 待清除的变量列表

定义一个包级别的变量列表，供清除函数使用：

```go
var allEnvVarKeys = []string{
    envBaseURL,      // ANTHROPIC_BASE_URL
    envAuthToken,    // ANTHROPIC_AUTH_TOKEN
    envModel,        // ANTHROPIC_MODEL
    envHaikuModel,   // ANTHROPIC_DEFAULT_HAIKU_MODEL
    envSonnetModel,  // ANTHROPIC_DEFAULT_SONNET_MODEL
    envOpusModel,    // ANTHROPIC_DEFAULT_OPUS_MODEL
    envDisableExperimentalBetas, // CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS
    envAgentTeams,   // CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS
}
```

使用精确的变量名匹配，**不使用前缀通配**（如 `ANTHROPIC_*`），避免误删用户自定义的环境变量。

## 核心函数设计

### `clearAllConfig()`

主入口函数，协调整个清除流程：

1. 调用 `showClearSummary()` 显示摘要
2. 等待用户输入 y/n 确认
3. 如果确认，依次调用各平台清除函数
4. 调用 `showClearReport()` 显示结果

### Shell 配置清除

**复用 `removeEnvVarUnix(key)`**，在循环中对 8 个变量逐一调用。该函数已经：
- 通过 `detectShellProfile()` 获取当前 shell 类型和配置文件列表
- 正确处理 bash/zsh (`export KEY=`) 和 fish (`set -Ux KEY`) 两种格式
- 处理回退文件列表
- 文件不存在时静默跳过

### VSCode 配置清除 — `clearVSCodeConfig() (clearResult)`

从 VSCode settings.json 中移除相关配置键。

逻辑：
1. 调用 `getVSCodeSettingsPath()` 获取路径
2. 如果文件不存在，返回"跳过"
3. 读取并解析 JSONC（复用 `stripJSONC()`）
4. 删除 `claudeCode.environmentVariables` 键
5. 删除旧的 `claude-code.environmentVariables` 键（兼容）
6. **保留所有其他用户配置不变**
7. 写回格式化的 JSON（2 空格缩进）

### Windows 注册表清除

**复用 `removeEnvVarWindows(key)`**，在循环中对 8 个变量逐一顺序调用。该函数已经：
- 执行 `REG DELETE "HKCU\Environment" /V <KEY> /F`
- 失败时回退到 `setx <KEY> ""`
- 处理变量不存在的情况

顺序执行（与现有实现一致），不使用并行。

### `clearProcessEnvVars()`

清除当前进程中的环境变量：
- 对 8 个变量调用 `os.Unsetenv()`

### `showClearSummary()`

在确认前显示即将清除的内容摘要，包括：
- 将被清除的位置列表（根据当前平台动态显示）
- 涉及的环境变量名列表

### `showClearReport(results []clearResult)`

清除完成后显示结果报告，每个位置标注状态：
- `✓` 成功（显示实际移除数量，动态统计）
- `—` 跳过（文件不存在等）
- `✗` 失败（附带错误信息）

## 数据结构

```go
type clearResult struct {
    Location string // 位置描述，如 "~/.zshrc"
    Status   string // "success" | "skipped" | "failed"
    Message  string // 详细信息，如 "移除了 7 个环境变量"
    Err      error  // 错误信息（如有）
}
```

## 错误处理

- 每个位置的清除独立进行，一个失败不影响其他位置
- 文件不存在视为"跳过"，不报错
- Windows 注册表中变量不存在视为"跳过"
- 所有错误在最终报告中汇总展示

## 平台行为差异

| 操作 | macOS/Linux | Windows | WSL |
|------|-------------|---------|-----|
| Shell 清除 | 清除 ~/.zshrc 等 | 不适用 | 清除 ~/.zshrc 等 |
| VSCode 清除 | 清除 macOS 路径 | 清除 Windows 路径 | 清除 Windows 侧路径 |
| 注册表清除 | 不适用 | 删除 HKCU\Environment | 不适用 |
| 进程变量清除 | os.Unsetenv | os.Unsetenv | os.Unsetenv |

## 影响范围

- **新增代码**：约 150-200 行（复用现有函数减少了代码量）
- **修改点**：
  - 主菜单选项列表（新增第 6 项）
  - `main()` 函数 switch 分支（新增 mode 6，early return）
- **复用**：`removeEnvVarUnix()`、`removeEnvVarWindows()`、`detectShellProfile()`、`getVSCodeSettingsPath()`、`stripJSONC()`
- **不涉及**：安装脚本、测试文件、现有配置保存逻辑
