# 设计文档：四项功能改进

**日期**：2026-03-13
**状态**：已确认，待实现

---

## 功能1：API 检测使用用户配置的默认模型

### 背景

`validateAPIConnection`（dmxapi-claude-code.go:603）当前硬编码 `"claude-haiku-4-5-20251001"` 作为测试模型，完全忽略用户实际配置的默认模型（`ANTHROPIC_MODEL`）。当用户使用三方平台专属模型时，检测结果可能不准确。

### 改动范围

**修改 `validateAPIConnection` 函数签名**：
```go
// 旧
func validateAPIConnection(baseURL, authToken string) error
// 新
func validateAPIConnection(baseURL, authToken, model string) error
```
请求体中的 `"model"` 字段改为使用传入的 `model` 参数。
调用处改为：`validateAPIConnection(cfg.BaseURL, cfg.AuthToken, cfg.Model)`

**错误处理说明**：
- HTTP 404 → 提示"API 端点不存在或模型名称不正确"（因为模型名无效也可能返回404）
- HTTP 401 → "认证失败：API Token 无效"
- HTTP 403 → "权限被拒绝"
- 其余错误 → 原有逻辑不变

**修改 `selectFixOption()` 函数**：增加第4个选项"修改模型名"：
```
[1] 修改 URL    Base URL 有问题
[2] 修改 Key   API Key 有问题
[3] 都修改     URL 和 Key 都有问题
[4] 修改模型名  模型名称可能不正确
```

**修改 `main()` 验证循环**：
- 选择4时，调用 `runL2Menu("默认模型", cfg.Model)` 让用户重新选择模型，更新 `cfg.Model`
- 修改模型后 `continue` 进入下一次循环，立即用新模型重新验证
- 验证成功后跳出循环（行为与现有逻辑一致）

---

## 功能2：启动时检测 Claude Code 是否已安装

### 背景

工具的目的是配置 Claude Code 的环境变量，若 Claude Code 本身未安装则配置无意义。

### 改动范围

**新增函数 `checkClaudeCodeInstalled() bool`**：
```go
func checkClaudeCodeInstalled() bool {
    _, err := exec.LookPath("claude")
    return err == nil
}
```

**在 `main()` 的 `printLogo()` 之后、`selectConfigMode()` 之前调用**：
```go
if !checkClaudeCodeInstalled() {
    printError("未检测到 Claude Code，请先安装后再运行此工具")
    fmt.Println()
    // 根据 runtime.GOOS 显示对应平台的安装命令
    printInfo("安装命令:")
    // macOS/Linux 显示 curl 命令
    // Windows 显示 PowerShell 命令
    fmt.Println()
    styledInput("按回车键退出")
    os.Exit(1)
}
```

**平台特定安装命令**（根据 `runtime.GOOS` 判断）：
- **macOS / Linux / WSL**：`curl -fsSL https://claude.ai/install.sh | bash`
- **Windows PowerShell**：`irm https://claude.ai/install.ps1 | iex`
- **Windows CMD**：`curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd`

Windows 判断逻辑：`runtime.GOOS == "windows"` 时同时打印 PowerShell 和 CMD 两种命令，其余平台只打印 curl 命令。

流程：打印错误 → 打印安装命令 → `styledInput("按回车键退出")` 等待用户 → `os.Exit(1)`。
此时尚未进入配置流程，无需清理任何状态。

---

## 功能3：实验性 Agent Teams 环境变量配置（主菜单选项4）

### 背景

`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` 是实验性功能开关，不应默认开启，需要用户主动选择。

### 改动范围

**新增常量**：
```go
envAgentTeams = "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"
```

**修改 `selectConfigMode()`**：增加第4项，返回值增加 `case "4": return 4`：
```
[1] 从头配置          配置 URL、Token 和模型
[2] 仅配置模型        跳过 URL 和 Token 配置
[3] 解决 400 报错     禁用实验性请求头
[4] 配置实验性功能    启用/禁用 Agent Teams
```

**新增 `configureAgentTeams()` 函数**，选4时调用（独立流程，不走其他配置）：
1. 读取当前变量状态：`currentVal := getEnvVar(envAgentTeams)`，显示"已启用"或"未设置"
2. 用 `runConfirmMenu("是否启用 Agent Teams 功能")` 询问用户
3. 启用 → 调用 `setEnvVarsUnix/Windows` 写入 `{"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"}`
4. 禁用 → 调用 `removeEnvVarUnix/Windows` 删除该变量（若本已未设置，直接提示"无需操作"）
5. 打印结果摘要，`styledInput("按回车键退出")` 后退出

**新增 `removeEnvVarUnix(key string) error` 函数**：
- 读取对应配置文件（macOS：`.zshrc` + `.bash_profile`；Linux：`.bashrc` + `.profile`）
- 过滤掉以 `export KEY=` 开头的行（幂等：变量不存在时返回 nil，不报错）
- 压缩连续空行为单个空行，确保文件末尾保留换行符
- 写回文件

**新增 `removeEnvVarWindows(key string) error` 函数**：
1. 优先用 `REG DELETE "HKCU\Environment" /V KEY /F` 删除注册表用户变量
2. 若 REG DELETE 失败（权限不足或其他错误），降级为 `setx KEY " "` 并打印警告提示用户手动处理
3. 向用户说明需要重启终端才能生效

**`main()` 中处理选项4**：
```go
} else if configMode == 4 {
    configureAgentTeams()
    return // 不走后续配置流程
}
```

---

---

## 功能4：启动时检查版本更新

### 背景

程序当前 `appVersion = "1.0.0"` 与实际发布版本 `v1.4.4` 不符，且没有更新提示机制。需要在启动时自动检测是否有新版本，提示用户下载。

### 版本获取方案

CNB 没有公开的无认证 JSON API（`api.cnb.cool` 需要 Bearer Token）。但 releases 页面为服务端渲染，版本数据嵌入在 HTML 的 `initialState` JSON 中：

- **获取 URL**：`https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases`
- **提取正则**：`"tagRef":"refs/tags/(v\d+\.\d+\.\d+)"` 严格匹配 `vMAJOR.MINOR.PATCH` 格式，第一条匹配即最新版（页面按发布时间倒序渲染）
- **HTTP 超时**：5 秒（`client.Timeout = 5 * time.Second`），失败时静默跳过，不阻塞启动

### 改动范围

**修正 `appVersion` 常量**：
```go
// 旧
appVersion = "1.0.0"
// 新
appVersion = "1.4.4"
```

**新增函数 `fetchLatestVersion() string`**：
- 创建带 5 秒超时的 `http.Client`，GET releases 页面 HTML
- 正则 `"tagRef":"refs/tags/(v\d+\.\d+\.\d+)"` 提取第一条匹配，去掉前缀 `v` 返回如 `"1.4.5"`
- 任何错误（网络超时、正则不匹配、版本格式不符）均返回 `""` 表示跳过

**新增函数 `compareVersions(a, b string) int`**：
- 将版本字符串按 `.` 分割为数字段，段数不足 3 段时补 0（如 `1.0` 视为 `1.0.0`）
- 逐段转 int 比较（避免 `"1.10" < "1.9"` 的字符串陷阱）
- 任何段解析失败返回 0（视为相等，不触发更新提示）
- 返回 -1 / 0 / 1（a 小于 / 等于 / 大于 b）

**新增函数 `checkForUpdates()`**：
1. 调用 `fetchLatestVersion()`，若返回 `""` 则直接 return
2. 若 `compareVersions(appVersion, latest) < 0`，说明有新版本
3. 复用 `runConfirmMenu("发现新版本 vX.X.X，是否立即前往下载页？")` 展示更新提示（是=立即下载，否=跳过继续），ESC 键等同于"跳过继续"
4. 选"是" → 用系统命令打开下载页，然后 `os.Exit(0)` 退出
5. 选"否"或 ESC → 直接 return，继续正常流程

**打开浏览器的系统命令**（根据 `runtime.GOOS`）：
- macOS：`exec.Command("open", url)`
- Linux：`exec.Command("xdg-open", url)`（失败则打印链接让用户手动访问）
- Windows：`exec.Command("cmd", "/c", "start", "", url)`（`start` 命令第一个参数为空字符串窗口标题）

若打开浏览器失败，降级为 `printInfo("请手动访问: " + url)` 后正常退出。

**在 `main()` 中的调用位置**：
```
initWindowsConsole()
printLogo()
checkClaudeCodeInstalled()  // 功能2：未安装则退出
checkForUpdates()           // 功能4：有新版本则提示（失败则静默跳过）
selectConfigMode()          // 原有流程
```

---

## 影响范围汇总

| 改动 | 涉及函数/常量 | 类型 |
|------|-------------|------|
| 功能1 | `validateAPIConnection`、`selectFixOption`、`main` | 修改 |
| 功能2 | `main`、新增 `checkClaudeCodeInstalled` | 新增+修改 |
| 功能3 | `selectConfigMode`、`main`、新增 `configureAgentTeams`、`removeEnvVarUnix`、`removeEnvVarWindows`、常量 `envAgentTeams` | 新增+修改 |
| 功能4 | 常量 `appVersion`、新增 `fetchLatestVersion`、`compareVersions`、`checkForUpdates`、`main` | 新增+修改 |

所有改动均在 `dmxapi-claude-code.go` 单文件内完成，无需创建新文件。
