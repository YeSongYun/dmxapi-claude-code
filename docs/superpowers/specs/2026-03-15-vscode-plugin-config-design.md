# VSCode 插件配置支持设计文档

**日期**: 2026-03-15
**项目**: dmxapi-claude-code
**状态**: 已确认

---

## 背景

Claude Code VSCode 插件无法读取 shell 环境变量（如 `.zshrc` 中的 `export ANTHROPIC_BASE_URL=...`），需要将配置写入 VSCode 的 `settings.json`，以 `claude-code.environmentVariables` 数组的形式存储。

当前工具已能配置 shell/Windows 注册表环境变量，本次改动在此基础上增加对 VSCode 插件的支持。

---

## 变更范围

### 1. 主菜单新增模式5

当前 4 项菜单扩展为 5 项：

```
[1]  从头配置         配置 URL、Token 和模型
[2]  仅配置模型       跳过 URL 和 Token 配置
[3]  解决 400 报错    禁用实验性请求头
[4]  配置实验性功能    启用/禁用 Agent Teams
[5]  配置 VSCode 插件  写入 VSCode settings.json
```

### 2. 模式1（从头配置）后置步骤追加

完成主配置保存后，追加两个可选步骤：

```
保存完成
  → 询问："是否同时配置 Agent Teams？"   （用户可跳过）
  → 询问："是否同时配置 VSCode 插件？"   （用户可跳过）
  → 打印摘要 → 退出
```

### 3. 新增模式5核心逻辑

复用已保存的环境变量配置，写入 VSCode `settings.json`。

---

## 跨平台路径检测

| 平台 | settings.json 路径 | 获取方式 |
|------|-------------------|---------|
| macOS | `~/Library/Application Support/Code/User/settings.json` | `os.UserHomeDir()` |
| Linux | `~/.config/Code/User/settings.json` | `os.UserHomeDir()` |
| Windows | `%APPDATA%\Code\User\settings.json` | `os.Getenv("APPDATA")` |
| WSL | `/mnt/c/Users/<用户名>/AppData/Roaming/Code/User/settings.json` | 调用 `cmd.exe /c echo %USERPROFILE%` 获取 Windows 用户目录，再拼接 `AppData/Roaming/Code/User/settings.json` |

路径拼接全部使用 `filepath.Join()`。WSL 下通过执行 `cmd.exe /c echo %USERPROFILE%` 获取 Windows 侧用户目录，若执行失败则回退到 `/mnt/c/Users/` 下匹配第一个非系统用户目录。

---

## 写入格式

在 `settings.json` 中仅更新 `claude-code.environmentVariables` 键，保留文件中所有其他设置：

```json
{
  "claude-code.environmentVariables": [
    { "name": "ANTHROPIC_BASE_URL",                    "value": "https://..." },
    { "name": "ANTHROPIC_AUTH_TOKEN",                  "value": "sk-..." },
    { "name": "ANTHROPIC_MODEL",                       "value": "claude-sonnet-4-6-cc" },
    { "name": "ANTHROPIC_DEFAULT_HAIKU_MODEL",         "value": "claude-haiku-4-5-20251001-cc" },
    { "name": "ANTHROPIC_DEFAULT_SONNET_MODEL",        "value": "claude-sonnet-4-6-cc" },
    { "name": "ANTHROPIC_DEFAULT_OPUS_MODEL",          "value": "claude-opus-4-6-cc" },
    { "name": "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS","value": "1" }
  ]
}
```

**环境变量写入策略说明**：
- `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS`：始终写入，固定值为 `"1"`，与 `saveConfig()` 行为一致
- `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`：若当前进程环境变量中已设置（值为 `"1"`），则追加写入；否则不写入
- 空值变量（如 BaseURL/Token 为空）仍写入，行为与 CLI 一致，由用户确认后决定是否继续

---

## 模式5 执行流程

**数据来源**：模式5在 `main()` 中通过 `loadExistingConfig()` 从当前进程环境变量读取已有配置，再传入 `configureVSCode(cfg Config)`。模式1结束后复用已保存至内存的 `cfg` 直接传入。

```
1. 检测平台，确定 settings.json 路径（调用 getVSCodeSettingsPath()）
2. 使用传入的 cfg 展示将写入的变量列表
3. 若关键配置（BaseURL/Token）为空 → 警告并询问是否继续
4. 用户确认后调用 saveVSCodeConfig(cfg)
5.   读取 settings.json（不存在则自动创建目录和空文件 {}）
6.   JSON 解析（失败见边界处理）→ 仅替换 claude-code.environmentVariables 键
7.   序列化写回文件（2空格缩进，保留其他键）
8. 根据 saveVSCodeConfig 返回值：
     成功 → printSuccess + 打印写入路径
     失败 → printError（与 configureAgentTeams 风格一致，不 os.Exit）
9. 模式5独立运行时：显示"按回车键退出"；模式1内嵌调用时：不显示，由 main 统一处理退出
```

**模式1后置步骤调用方式**：
```
// 保存完成后
if styledConfirm("是否同时配置 Agent Teams 功能") {
    configureAgentTeams()
}
if styledConfirm("是否同时配置 VSCode 插件") {
    configureVSCode(cfg)   // 传入已保存的 cfg，不再询问退出
}
printSummary(cfg)
styledInput("按回车键退出")   // 仅此一处，不重复
```

---

## 边界处理

| 情形 | 处理方式 |
|------|---------|
| `settings.json` 不存在 | `os.MkdirAll` 创建目录，写入新文件 `{}` 再合并 |
| JSON 解析失败 | 打印错误，询问"是否备份原文件并重新创建？"；用户确认后将原文件重命名为 `settings.json.bak`，再写入新文件；用户拒绝则跳过，不破坏原文件 |
| `APPDATA` 环境变量为空（Windows） | 回退到 `os.UserHomeDir() + AppData/Roaming` |
| WSL 环境 | 写入 Windows 侧路径，额外提示用户 |
| 关键配置为空 | 警告但不强制阻止（用户可继续写入空值或跳过） |

---

## 新增函数清单

| 函数名 | 签名 | 职责 |
|--------|------|------|
| `getVSCodeSettingsPath` | `() (string, error)` | 跨平台检测并返回 settings.json 绝对路径；WSL 下调用 cmd.exe 获取 Windows 用户目录 |
| `saveVSCodeConfig` | `(cfg Config) error` | 读取→JSON解析→合并→写回 settings.json；解析失败时提示备份并重建 |
| `configureVSCode` | `(cfg Config, exitOnDone bool)` | 模式5 完整交互流程；`exitOnDone=true` 时末尾显示"按回车键退出"，嵌入模式1时传 `false` |

**错误处理约定**：`configureVSCode` 内部调用 `saveVSCodeConfig` 后，成功调用 `printSuccess`，失败调用 `printError`，均不调用 `os.Exit`，与 `configureAgentTeams` 保持一致。

---

## 不在本次范围内

- Cursor、Windsurf、VSCode Insiders 等其他编辑器（仅支持 VS Code）
- 独立的 VSCode URL/Token 输入（复用已有配置）
- 从 VSCode settings.json 反向读取配置到 shell 环境变量
