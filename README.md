# DMXAPI Claude Code 配置工具

一键配置 Anthropic Claude Code CLI 环境变量的跨平台工具。

## 功能特性

- 交互式配置 API 地址和认证令牌
- 自动验证 API 连接有效性
- 配置默认模型设置
- 支持 Windows / Linux / macOS
- 环境变量自动持久化

## 下载

> **说明**：`[版本]` 替换为实际下载的版本号，如 `v1.3.3`

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Windows | x64 | `dmxapi-claude-code-[版本]-windows-amd64.exe` |
| Linux | x64 | `dmxapi-claude-code-[版本]-linux-amd64` |
| Linux | ARM64 | `dmxapi-claude-code-[版本]-linux-arm64` |
| macOS | Intel | `dmxapi-claude-code-[版本]-macos-amd64` |
| macOS | Apple Silicon (M1/M2/M3/M4) | `dmxapi-claude-code-[版本]-macos-arm64` |

## 快速选择版本

不确定自己的系统架构？运行以下命令确认：

| 系统 | 检测命令 | 结果 → 对应文件后缀 |
|------|----------|---------------------|
| Windows | `echo %PROCESSOR_ARCHITECTURE%` | `AMD64` → `windows-amd64.exe` |
| Linux | `uname -m` | `x86_64` → `linux-amd64` / `aarch64` → `linux-arm64` |
| macOS | `uname -m` | `x86_64` → `macos-amd64` / `arm64` → `macos-arm64` |

## 使用方法

> **说明**：以下示例文件名中的 `v1.3.3` 为版本号示例，请替换为实际下载的版本号。

### Windows x64

```powershell
# 下载后直接运行
.\dmxapi-claude-code-v1.3.3-windows-amd64.exe
```

### Linux

#### Linux x64 (amd64)

适用于普通 PC 服务器、云主机（x86_64 架构）。

```bash
# 确认架构
uname -m  # 应输出 x86_64

# 添加执行权限
chmod +x dmxapi-claude-code-v1.3.3-linux-amd64

# 运行
./dmxapi-claude-code-v1.3.3-linux-amd64
```

#### Linux ARM64

适用于树莓派（64 位系统）、AWS Graviton、Oracle Ampere 等 ARM64 架构服务器。

```bash
# 确认架构
uname -m  # 应输出 aarch64

# 添加执行权限
chmod +x dmxapi-claude-code-v1.3.3-linux-arm64

# 运行
./dmxapi-claude-code-v1.3.3-linux-arm64
```

### macOS

#### macOS Apple Silicon (M1/M2/M3/M4，arm64)

适用于 2020 年末及之后发布的 Mac（搭载 Apple Silicon 芯片）。

```bash
# 确认架构
uname -m  # 应输出 arm64

# 添加执行权限
chmod +x dmxapi-claude-code-v1.3.3-macos-arm64

# 运行
./dmxapi-claude-code-v1.3.3-macos-arm64
```

#### macOS Intel (amd64)

适用于 2020 年前发布的 Mac（搭载 Intel 处理器）。

```bash
# 确认架构
uname -m  # 应输出 x86_64

# 添加执行权限
chmod +x dmxapi-claude-code-v1.3.3-macos-amd64

# 运行
./dmxapi-claude-code-v1.3.3-macos-amd64
```

#### macOS Gatekeeper 安全限制处理

首次运行时 macOS 可能提示"无法验证开发者"或"已损坏"，以下三种方式任选其一：

**方式一（推荐）：命令行移除隔离标记**

```bash
# 将 <文件名> 替换为实际文件名，如 dmxapi-claude-code-v1.3.3-macos-arm64
xattr -cr <文件名>
# 然后正常运行
./<文件名>
```

**方式二：通过系统设置允许运行**

1. 尝试运行程序，出现安全提示后点击"完成"（不要点"移到废纸篓"）
2. 打开 **系统设置 → 隐私与安全性**
3. 向下滚动，找到"已阻止使用……"的提示，点击**仍要打开**
4. 在弹出的确认对话框中再次点击**打开**

> macOS Sequoia (15.x) 注意：系统设置路径相同，但界面可能稍有不同。

**方式三：Finder 右键打开（临时，仅当次有效）**

1. 在 Finder 中找到下载的文件
2. 按住 **Control** 键并单击文件（或右键单击）
3. 选择**打开**
4. 在弹出的对话框中点击**打开**

## 配置的环境变量

| 环境变量 | 说明 |
|----------|------|
| `ANTHROPIC_BASE_URL` | API 服务器地址 |
| `ANTHROPIC_AUTH_TOKEN` | API 认证令牌 |
| `ANTHROPIC_MODEL` | 默认模型 |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Haiku 模型 |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Sonnet 模型 |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Opus 模型 |

## 配置生效

| 系统 | 立即生效命令 | 说明 |
|------|-------------|------|
| Windows | 重新打开终端 | 已通过 `setx` 写入用户注册表 |
| Linux | `source ~/.bashrc` | 已写入 `~/.bashrc` 和 `~/.profile` |
| macOS (zsh) | `source ~/.zshrc` | 已写入 `~/.zshrc` 和 `~/.bash_profile` |
| macOS (bash) | `source ~/.bash_profile` | 已写入 `~/.zshrc` 和 `~/.bash_profile` |

**验证环境变量已生效：**

```bash
# Linux / macOS
echo $ANTHROPIC_BASE_URL

# Windows PowerShell
echo $env:ANTHROPIC_BASE_URL
```

## 常见问题

**Q：macOS 提示"无法验证开发者"或"已损坏，无法打开"**

A：这是 macOS Gatekeeper 的安全机制，并非文件损坏。请参考上方 [macOS Gatekeeper 安全限制处理](#macos-gatekeeper-安全限制处理) 章节，推荐使用方式一的命令行方式解决。

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

**Q：树莓派用哪个版本？**

A：使用 `linux-arm64` 版本（需确保系统为 64 位，运行 `uname -m` 应输出 `aarch64`）。32 位系统暂不支持。

## 从源码编译

```bash
# 安装 Go 1.21+
# https://go.dev/dl/

# 下载依赖
go mod tidy

# 编译当前平台
go build -o dmxapi-claude-code dmxapi-claude-code.go

# 交叉编译其他平台
GOOS=linux GOARCH=amd64 go build -o dmxapi-claude-code-linux-amd64 dmxapi-claude-code.go
GOOS=linux GOARCH=arm64 go build -o dmxapi-claude-code-linux-arm64 dmxapi-claude-code.go
GOOS=darwin GOARCH=amd64 go build -o dmxapi-claude-code-macos-amd64 dmxapi-claude-code.go
GOOS=darwin GOARCH=arm64 go build -o dmxapi-claude-code-macos-arm64 dmxapi-claude-code.go
```

## 获取 Token

访问 [https://www.dmxapi.cn/token](https://www.dmxapi.cn/token) 获取您的 API Token。

## 许可证

MIT License
