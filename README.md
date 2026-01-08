# DMXAPI Claude Code 配置工具

一键配置 Anthropic Claude Code CLI 环境变量的跨平台工具。

## 功能特性

- 交互式配置 API 地址和认证令牌
- 自动验证 API 连接有效性
- 配置默认模型设置
- 支持 Windows / Linux / macOS
- 环境变量自动持久化

## 下载

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Windows | x64 | `dmxapi-claude-code.exe` |
| Linux | x64 | `dmxapi-claude-code-linux-amd64` |
| Linux | ARM64 | `dmxapi-claude-code-linux-arm64` |
| macOS | Intel | `dmxapi-claude-code-macos-amd64` |
| macOS | Apple Silicon (M1/M2/M3) | `dmxapi-claude-code-macos-arm64` |

## 使用方法

### Windows

```powershell
.\dmxapi-claude-code.exe
```

### Linux

```bash
# 添加执行权限
chmod +x dmxapi-claude-code-linux-amd64

# 运行
./dmxapi-claude-code-linux-amd64
```

### macOS

```bash
# 添加执行权限
chmod +x dmxapi-claude-code-macos-arm64

# 运行（首次可能需要在"系统设置 > 隐私与安全性"中允许）
./dmxapi-claude-code-macos-arm64
```

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

- **Windows**: 重新打开终端窗口
- **Linux**: 运行 `source ~/.bashrc` 或重新打开终端
- **macOS**: 运行 `source ~/.zshrc` 或重新打开终端

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
