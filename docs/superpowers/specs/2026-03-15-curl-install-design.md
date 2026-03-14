# curl 一键安装方案设计文档

**日期**：2026-03-15
**状态**：已审批

---

## 背景

当前用户需要手动下载二进制文件、赋权并运行，步骤繁琐，面向非技术用户体验不佳。本方案通过一行 `curl` 命令实现"下载 + 赋权 + 运行"的完整安装流程。

---

## 目标

- 用户只需一行命令即可完成安装并进入交互式配置
- 覆盖 Linux、macOS、Windows（PowerShell + CMD）
- 不改动任何 Go 业务代码

---

## 方案选择

采用**方案 A：静态脚本**。在仓库中维护三个安装脚本，脚本内硬编码版本号，每次发版时同步更新。

理由：实现最简单，无外部依赖，对用户透明。

---

## 用户安装命令

### Linux / macOS
```bash
curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.sh | bash
```

### Windows PowerShell
```powershell
iwr -useb https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.ps1 | iex
```

### Windows CMD
```cmd
curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.cmd -o %TEMP%\install.cmd && %TEMP%\install.cmd
```

---

## 新增文件

### `install.sh`（Linux / macOS）

**流程：**
1. 定义版本常量 `VERSION`
2. 检测 OS：`uname -s` 输出值映射规则：
   - `Darwin` → 文件名中使用 `macos`
   - `Linux` → 文件名中使用 `linux`
   - 其他 → 打印"不支持的操作系统"并退出
3. 检测架构：`uname -m` 输出值映射规则：
   - `x86_64` → `amd64`
   - `arm64` / `aarch64` → `arm64`
   - 其他 → 打印"不支持的架构"并退出
4. 拼接文件名：`dmxapi-claude-code-{VERSION}-{映射后OS}-{映射后ARCH}`
5. 拼接下载 URL：`https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/{VERSION}/{文件名}`
6. `curl` 下载到 `/tmp/`；若下载失败（非 0 退出码）则打印错误并退出
7. `chmod +x`
8. macOS 额外执行 `xattr -cr`（处理 Gatekeeper）
9. 直接运行工具（交互式配置启动）
10. 运行完毕后删除 `/tmp/` 临时文件

### `install.ps1`（Windows PowerShell）

**流程：**
1. 定义版本常量 `$VERSION`
2. 检测架构：`$env:PROCESSOR_ARCHITECTURE`：
   - `AMD64` → 文件名使用 `windows-amd64.exe`
   - 其他（含 ARM64）→ 打印"当前仅支持 x64 架构"并退出
3. 拼接文件名：`dmxapi-claude-code-{VERSION}-windows-amd64.exe`
4. 拼接下载 URL（同上规则）
5. `Invoke-WebRequest` 下载到 `$env:TEMP\`；若失败则打印错误并退出
6. 直接运行工具
7. 运行完毕后删除临时文件

### `install.cmd`（Windows CMD）

**流程：**
1. 定义版本变量 `VERSION`
2. 检测架构：`%PROCESSOR_ARCHITECTURE%`：
   - `AMD64` → 文件名使用 `windows-amd64.exe`
   - 其他 → 打印"当前仅支持 x64 架构"并退出（`exit /b 1`）
3. 拼接文件名：`dmxapi-claude-code-{VERSION}-windows-amd64.exe`
4. `curl`（Windows 10+ 内置）下载到 `%TEMP%\`；`errorlevel` 非零则打印错误并退出
5. 直接运行工具
6. 运行完毕后删除临时文件

---

## 修改文件

### `README.md`

在"下载"章节前新增"⚡ 快速安装（推荐）"章节，包含三平台安装命令。

### `.cnb.yml`

在 Release description 的 `🛠️ 安装说明` 章节**之前**插入 `⚡ 快速安装（推荐）` 章节，变量使用 `${CNB_BRANCH}`。

### `.github/workflows/release.yml`

镜像同步文件（CNB 为主发布平台，GitHub 为镜像）。与 `.cnb.yml` 保持一致，在 Release body 的安装说明前插入快速安装章节，变量使用 `${{ github.ref_name }}`。此文件为次要目标，优先级低于 `.cnb.yml`。

---

## 下载 URL 格式

```
https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/{VERSION}/{文件名}
```

其中 `{文件名}` = `dmxapi-claude-code-{VERSION}-{映射后OS}-{映射后ARCH}[.exe]`，OS 映射规则见"新增文件"章节。

示例：
```
# macOS Apple Silicon
https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/v1.4.6/dmxapi-claude-code-v1.4.6-macos-arm64

# Linux x64
https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/v1.4.6/dmxapi-claude-code-v1.4.6-linux-amd64

# Windows x64
https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/v1.4.6/dmxapi-claude-code-v1.4.6-windows-amd64.exe
```

---

## 发版 Checklist 新增项

每次发新版时，需同步更新三个脚本文件中的版本号常量：
- `install.sh` 中的 `VERSION="vX.X.X"`
- `install.ps1` 中的 `$VERSION = "vX.X.X"`
- `install.cmd` 中的 `set VERSION=vX.X.X`

---

## 约束

- 不修改任何 Go 业务代码
- 脚本运行后工具自动进入交互式配置，不接受命令行参数
- Windows CMD 方案依赖 Windows 10+ 内置 curl（版本 1803+）
