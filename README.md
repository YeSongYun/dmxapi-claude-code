# DMXAPI Claude Code 配置工具

一键配置 Anthropic Claude Code CLI 环境变量的跨平台工具。

## 功能特性

- **dmxapi 推荐配置**：只需输入 API Key，一键完成全部配置
- **多配置管理**：保存多套命名配置，随时切换、编辑、删除
- 交互式配置 API 地址、认证令牌与模型，内置 Claude 5 系列等预设模型，支持自定义模型名
- 自动验证 API 连接有效性，失败时可现场修改或强制保存
- 环境变量自动持久化，同步写入 `~/.claude/settings.json`，可选配置 VSCode Claude Code 插件
- 自动启用思考等级（Effort Level = max），可选启用 Agent Teams 实验性功能
- 可配置 Git 提交 / PR 署名（默认 "Generated with dmxapi"）
- 内置「解决 400 报错」模式（禁用实验性请求头）
- 支持仅清除当前生效配置（保留已保存配置），也可一键清除全部配置
- 启动时自动检测 Claude Code 是否安装，并检查本工具新版本
- 支持 Windows / Linux / macOS / WSL，兼容 bash / zsh / fish

## ⚡ 快速安装（推荐）

无需手动下载，一行命令自动下载并运行配置工具（临时运行，结束后自动清理，不常驻安装）。

### Linux / macOS

```bash
curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/git/raw/main/install.sh | bash
```

### Windows PowerShell

```powershell
iwr -useb https://cnb.cool/dmxapi/dmxapi_claude_code/-/git/raw/main/install.ps1 | iex
```

### Windows CMD

```cmd
curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/git/raw/main/install.cmd -o "%TEMP%\install.cmd" && call "%TEMP%\install.cmd"
```

> **macOS 说明**：安装脚本已自动移除 Gatekeeper 隔离标记（`xattr -cr`），无需手动处理。
>
> **Windows 说明**：CMD 方案需要 Windows 10 版本 1803 或更高（内置 curl）。Windows ARM64 设备同样支持，通过 x64 模拟层运行。

---

## 下载

> **说明**：`[版本]` 替换为实际下载的版本号，如 `v1.9.4`

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Windows | x64 | `dmxapi-claude-code-[版本]-windows-amd64.exe` |
| Windows | ARM64 | 同样使用 `dmxapi-claude-code-[版本]-windows-amd64.exe`（经 x64 模拟层运行）|
| Linux | x64 | `dmxapi-claude-code-[版本]-linux-amd64` |
| Linux | ARM64 | `dmxapi-claude-code-[版本]-linux-arm64` |
| macOS | Intel | `dmxapi-claude-code-[版本]-macos-amd64` |
| macOS | Apple Silicon (M1/M2/M3/M4) | `dmxapi-claude-code-[版本]-macos-arm64` |

## 快速选择版本

不确定自己的系统架构？运行以下命令确认：

| 系统 | 检测命令 | 结果 → 对应文件后缀 |
|------|----------|---------------------|
| Windows | `echo %PROCESSOR_ARCHITECTURE%` | `AMD64` 或 `ARM64` → `windows-amd64.exe` |
| Linux | `uname -m` | `x86_64` → `linux-amd64` / `aarch64` → `linux-arm64` |
| macOS | `uname -m` | `x86_64` → `macos-amd64` / `arm64` → `macos-arm64` |

## 使用方法

> **说明**：以下示例文件名中的 `v1.9.4` 为版本号示例，请替换为实际下载的版本号。

### Windows x64

```powershell
# 下载后直接运行
.\dmxapi-claude-code-v1.9.4-windows-amd64.exe
```

### Linux

#### Linux x64 (amd64)

适用于普通 PC 服务器、云主机（x86_64 架构）。

```bash
# 确认架构
uname -m  # 应输出 x86_64

# 添加执行权限
chmod +x dmxapi-claude-code-v1.9.4-linux-amd64

# 运行
./dmxapi-claude-code-v1.9.4-linux-amd64
```

#### Linux ARM64

适用于树莓派（64 位系统）、AWS Graviton、Oracle Ampere 等 ARM64 架构服务器。

```bash
# 确认架构
uname -m  # 应输出 aarch64

# 添加执行权限
chmod +x dmxapi-claude-code-v1.9.4-linux-arm64

# 运行
./dmxapi-claude-code-v1.9.4-linux-arm64
```

### macOS

#### macOS Apple Silicon (M1/M2/M3/M4，arm64)

适用于 2020 年末及之后发布的 Mac（搭载 Apple Silicon 芯片）。

```bash
# 确认架构
uname -m  # 应输出 arm64

# 添加执行权限并移除 macOS 安全隔离标记
chmod +x dmxapi-claude-code-v1.9.4-macos-arm64
xattr -cr dmxapi-claude-code-v1.9.4-macos-arm64

# 运行
./dmxapi-claude-code-v1.9.4-macos-arm64
```

#### macOS Intel (amd64)

适用于 2020 年前发布的 Mac（搭载 Intel 处理器）。

```bash
# 确认架构
uname -m  # 应输出 x86_64

# 添加执行权限并移除 macOS 安全隔离标记
chmod +x dmxapi-claude-code-v1.9.4-macos-amd64
xattr -cr dmxapi-claude-code-v1.9.4-macos-amd64

# 运行
./dmxapi-claude-code-v1.9.4-macos-amd64
```

> **说明**：`xattr -cr` 用于移除 macOS 对从网络下载文件添加的隔离标记（`com.apple.quarantine`），是 macOS 运行未签名可执行文件的必要步骤。若跳过此步骤，系统可能提示"无法验证开发者"或"已损坏，无法打开"。

## 配置的环境变量

| 环境变量 | 说明 |
|----------|------|
| `ANTHROPIC_BASE_URL` | API 服务器地址 |
| `ANTHROPIC_AUTH_TOKEN` | API 认证令牌 |
| `ANTHROPIC_MODEL` | 默认模型 |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Haiku 模型 |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Sonnet 模型 |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Opus 模型 |
| `ANTHROPIC_DEFAULT_FABLE_MODEL` | Fable 模型 |
| `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` | 禁用实验性 Beta 功能（固定为 1）|
| `CLAUDE_CODE_EFFORT_LEVEL` | 思考等级（启用时写入 `max`，配置流程默认自动启用）|
| `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` | Agent Teams 实验性功能（启用时写入 1，未启用不写入）|

> 除系统环境变量外，以上配置会同步写入 `~/.claude/settings.json`；选择配置 VSCode 插件时还会写入 VSCode 的 `claudeCode.environmentVariables` 设置。已保存的命名配置存放在 `~/.DMXAPI/claude_code/` 目录。

## 配置生效

| 系统 / Shell | 写入位置 | 立即生效方式 |
|------|------|-------------|
| Windows | 用户环境变量（`setx`，超长值经注册表写入）| 重新打开终端 |
| Linux (bash) | `~/.bashrc` | `source ~/.bashrc` |
| Linux (zsh) | `~/.zshrc` | `source ~/.zshrc` |
| macOS (zsh) | `~/.zshrc` 和 `~/.zprofile` | `source ~/.zshrc` |
| macOS (bash) | `~/.bash_profile` | `source ~/.bash_profile` |
| fish | `~/.config/fish/config.fish`（`set -Ux` universal 变量）| 新开 fish 会话后生效 |

> 未能识别的 shell 会回退写入多个常见配置文件（macOS：`.zshrc` / `.zprofile` / `.bash_profile`；Linux：`.bashrc` / `.profile`）。

**验证环境变量已生效：**

```bash
# Linux / macOS
echo $ANTHROPIC_BASE_URL

# Windows PowerShell
echo $env:ANTHROPIC_BASE_URL
```

## 常见问题

**Q：macOS 提示"无法验证开发者"或"已损坏，无法打开"**

A：这是 macOS Gatekeeper 的安全机制，并非文件损坏。按照上方安装步骤执行 `xattr -cr <文件名>` 移除隔离标记后重新运行即可（使用一键安装脚本时已自动处理）。若已错过此步骤，单独执行以下命令：

```bash
xattr -cr dmxapi-claude-code-v1.9.4-macos-arm64  # Apple Silicon
# 或
xattr -cr dmxapi-claude-code-v1.9.4-macos-amd64  # Intel
```

---

**Q：Linux 运行时提示 `Permission denied`**

A：缺少执行权限，运行以下命令后再重试：

```bash
chmod +x <文件名>
```

---

**Q：运行 `echo $ANTHROPIC_BASE_URL` 后输出为空**

A：当前终端尚未加载新配置，执行对应的 `source` 命令后再验证：

- Linux：`source ~/.bashrc`
- macOS (zsh)：`source ~/.zshrc`
- macOS (bash)：`source ~/.bash_profile`
- fish：新开一个 fish 会话（或执行 `source ~/.config/fish/config.fish`）

---

**Q：如何确认我的 Mac 是 Intel 还是 Apple Silicon？**

A：运行 `uname -m`，输出 `arm64` 为 Apple Silicon，输出 `x86_64` 为 Intel。也可点击苹果菜单 → **关于本机**，在"芯片"或"处理器"行查看。

---

**Q：Windows 配置后环境变量不生效**

A：确认已重新打开终端（不是刷新当前终端）。可用以下命令验证是否已写入注册表：

```powershell
[System.Environment]::GetEnvironmentVariable("ANTHROPIC_BASE_URL", "User")
```

若有输出则说明写入成功，重新打开终端即可生效。

---

**Q：Claude Code 请求报 400 错误（invalid request headers）怎么办？**

A：这是实验性 Beta 请求头导致的。本工具配置时会自动写入 `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` 解决；已保存配置的用户也可在配置管理的编辑菜单中选择「解决 400 报错」单独修复。

---

**Q：树莓派用哪个版本？**

A：使用 `linux-arm64` 版本（需确保系统为 64 位，运行 `uname -m` 应输出 `aarch64`）。32 位系统暂不支持。

## 从源码编译

```bash
# 安装 Go 1.21+
# https://go.dev/dl/

# 下载依赖
go mod tidy

# 编译当前平台（仓库含多个 .go 文件，需按包编译，勿指定单个文件）
go build -o dmxapi-claude-code .

# 交叉编译其他平台
GOOS=windows GOARCH=amd64 go build -o dmxapi-claude-code.exe .
GOOS=linux GOARCH=amd64 go build -o dmxapi-claude-code-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dmxapi-claude-code-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -o dmxapi-claude-code-macos-amd64 .
GOOS=darwin GOARCH=arm64 go build -o dmxapi-claude-code-macos-arm64 .
```

## 获取 Token

访问 [https://www.dmxapi.cn/keys](https://www.dmxapi.cn/keys) 获取您的 API Token。

## 许可证

MIT License
