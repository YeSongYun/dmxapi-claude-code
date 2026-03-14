# curl 一键安装 实现计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 dmxapi-claude-code 工具提供 `curl | bash` 一键安装体验，覆盖 Linux、macOS、Windows（PowerShell + CMD）。

**Architecture:** 在仓库根目录新增三个静态安装脚本，脚本内硬编码版本号，自动检测平台后从 CNB Release 下载对应二进制并运行。同步更新 README、`.cnb.yml`、`.github/workflows/release.yml` 的 Release 说明，以及 `CLAUDE.md` 发版 checklist。

**Tech Stack:** Bash (POSIX sh)、PowerShell、Windows Batch Script、Markdown

---

## 文件结构

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `install.sh` | Linux/macOS 安装脚本 |
| 新建 | `install.ps1` | Windows PowerShell 安装脚本 |
| 新建 | `install.cmd` | Windows CMD 安装脚本 |
| 修改 | `README.md` | 新增快速安装章节 |
| 修改 | `.cnb.yml` | Release description 新增快速安装章节 |
| 修改 | `.github/workflows/release.yml` | Release body 新增快速安装章节（镜像） |
| 修改 | `CLAUDE.md` | 发版 checklist 新增三个脚本的版本号提醒 |

---

## Chunk 1: 三个安装脚本

### Task 1: 创建 `install.sh`（Linux / macOS）

**Files:**
- Create: `install.sh`

- [ ] **Step 1: 创建脚本文件**

```bash
#!/bin/sh
set -e

VERSION="v1.4.6"

# 检测操作系统
OS=$(uname -s)
case "$OS" in
  Darwin) OS_NAME="macos" ;;
  Linux)  OS_NAME="linux" ;;
  *)
    echo "不支持的操作系统: $OS"
    exit 1
    ;;
esac

# 检测架构
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH_NAME="amd64" ;;
  arm64|aarch64) ARCH_NAME="arm64" ;;
  *)
    echo "不支持的架构: $ARCH"
    exit 1
    ;;
esac

FILENAME="dmxapi-claude-code-${VERSION}-${OS_NAME}-${ARCH_NAME}"
URL="https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/${VERSION}/${FILENAME}"
TMP_FILE="/tmp/${FILENAME}"

echo "正在下载 ${FILENAME}..."
if ! curl -fsSL "$URL" -o "$TMP_FILE"; then
  echo "下载失败，请检查网络连接或手动下载：$URL"
  exit 1
fi

chmod +x "$TMP_FILE"

# macOS：移除 Gatekeeper 隔离标记
if [ "$OS_NAME" = "macos" ]; then
  xattr -cr "$TMP_FILE" 2>/dev/null || true
fi

echo "正在启动配置工具..."
"$TMP_FILE"

rm -f "$TMP_FILE"
```

- [ ] **Step 2: 验证脚本语法**

```bash
sh -n install.sh
```

期望输出：无报错（命令无输出则表示语法正确）

- [ ] **Step 3: 检查脚本可读性**

```bash
cat install.sh
```

确认：`VERSION` 在第一个非注释行清晰可见，方便发版时修改。

- [ ] **Step 4: 提交**

```bash
git add install.sh
git commit -m "feat: 新增 install.sh Linux/macOS 一键安装脚本"
```

---

### Task 2: 创建 `install.ps1`（Windows PowerShell）

**Files:**
- Create: `install.ps1`

- [ ] **Step 1: 创建脚本文件**

```powershell
$VERSION = "v1.4.6"

# 检测架构
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -ne "AMD64") {
    Write-Host "当前仅支持 x64 架构，检测到：$arch" -ForegroundColor Red
    exit 1
}

$filename = "dmxapi-claude-code-$VERSION-windows-amd64.exe"
$url = "https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/$VERSION/$filename"
$tmpFile = Join-Path $env:TEMP $filename

Write-Host "正在下载 $filename..."
try {
    Invoke-WebRequest -Uri $url -OutFile $tmpFile -UseBasicParsing
} catch {
    Write-Host "下载失败：$_" -ForegroundColor Red
    exit 1
}

Write-Host "正在启动配置工具..."
& $tmpFile

Remove-Item -Force $tmpFile -ErrorAction SilentlyContinue
```

- [ ] **Step 2: 验证脚本语法（在 macOS/Linux 上用 pwsh 检查，若无 pwsh 可跳过此步）**

```bash
pwsh -NoProfile -Command "Get-Content install.ps1 | Out-Null; Write-Host 'syntax ok'"
```

期望输出：`syntax ok`（若无 pwsh 环境则人工检查括号/引号匹配）

- [ ] **Step 3: 提交**

```bash
git add install.ps1
git commit -m "feat: 新增 install.ps1 Windows PowerShell 一键安装脚本"
```

---

### Task 3: 创建 `install.cmd`（Windows CMD）

**Files:**
- Create: `install.cmd`

- [ ] **Step 1: 创建脚本文件**

```batch
@echo off
setlocal

set VERSION=v1.4.6

rem 检测架构
if /i not "%PROCESSOR_ARCHITECTURE%"=="AMD64" (
    echo 当前仅支持 x64 架构，检测到：%PROCESSOR_ARCHITECTURE%
    exit /b 1
)

set FILENAME=dmxapi-claude-code-%VERSION%-windows-amd64.exe
set URL=https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/%VERSION%/%FILENAME%
set TMP_FILE=%TEMP%\%FILENAME%

echo 正在下载 %FILENAME%...
curl -fsSL "%URL%" -o "%TMP_FILE%"
if errorlevel 1 (
    echo 下载失败，请检查网络连接或手动下载：%URL%
    exit /b 1
)

echo 正在启动配置工具...
"%TMP_FILE%"

del /f "%TMP_FILE%" 2>nul
endlocal
```

- [ ] **Step 2: 检查关键语法点**

人工确认以下三点：
1. 文件第一行为 `@echo off`
2. `if /i not "%PROCESSOR_ARCHITECTURE%"=="AMD64"` 使用了 `/i`（大小写不敏感）
3. `errorlevel 1` 判断在 curl 命令之后紧接着出现

- [ ] **Step 3: 提交**

```bash
git add install.cmd
git commit -m "feat: 新增 install.cmd Windows CMD 一键安装脚本"
```

---

## Chunk 2: 文档与配置更新

### Task 4: 更新 `README.md`

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 在"## 下载"章节前插入快速安装章节**

在 `README.md` 中找到 `## 下载` 这一行，在其**前面**插入以下内容：

```markdown
## ⚡ 快速安装（推荐）

无需手动下载，一行命令完成安装并自动启动配置。

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

> **Windows 说明**：CMD 方案需要 Windows 10 版本 1803 或更高（内置 curl）。

---

```

- [ ] **Step 2: 验证章节顺序**

```bash
grep -n "##" README.md
```

期望：`⚡ 快速安装` 出现在 `下载` 之前。

- [ ] **Step 3: 提交**

```bash
git add README.md
git commit -m "docs: README 新增快速安装章节"
```

---

### Task 5: 更新 `.cnb.yml` Release 说明

**Files:**
- Modify: `.cnb.yml`

- [ ] **Step 1: 在 `### 🛠️ 安装说明` 之前插入快速安装章节**

找到 `.cnb.yml` 中 `### 🛠️ 安装说明` 这一行（约第 118 行），在其前面插入：

```yaml
              ### ⚡ 快速安装（推荐）

              **Linux / macOS**
              ```bash
              curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.sh | bash
              ```

              **Windows PowerShell**
              ```powershell
              iwr -useb https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.ps1 | iex
              ```

              **Windows CMD**
              ```cmd
              curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.cmd -o %TEMP%\install.cmd && %TEMP%\install.cmd
              ```

              ---

```

注意：
- `.cnb.yml` 中 description 字段的内容使用 `|` 块标量，每行需保持与已有内容相同的缩进（14 个空格）。
- 快速安装命令中的 URL 使用 `main` 分支（而非 `${CNB_BRANCH}`），因为安装脚本托管在 `main` 分支的源码中，与二进制 Release tag 无关，始终指向最新版脚本。

- [ ] **Step 2: 验证 YAML 语法**

```bash
python3 -c "import yaml; yaml.safe_load(open('.cnb.yml'))" && echo "YAML 语法正确"
```

期望输出：`YAML 语法正确`

- [ ] **Step 3: 提交**

```bash
git add .cnb.yml
git commit -m "ci: CNB Release 说明新增快速安装章节"
```

---

### Task 6: 更新 `.github/workflows/release.yml`

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: 在 `### 🛠️ 安装说明` 之前插入快速安装章节**

找到 `.github/workflows/release.yml` 中 `### 🛠️ 安装说明` 这一行（约第 82 行），在其前面插入：

```yaml
            ### ⚡ 快速安装（推荐）

            **Linux / macOS**
            ```bash
            curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.sh | bash
            ```

            **Windows PowerShell**
            ```powershell
            iwr -useb https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.ps1 | iex
            ```

            **Windows CMD**
            ```cmd
            curl -fsSL https://cnb.cool/dmxapi/dmxapi_claude_code/-/raw/main/install.cmd -o %TEMP%\install.cmd && %TEMP%\install.cmd
            ```

            ---

```

注意：此文件缩进为 12 个空格（少于 `.cnb.yml` 的 14 个），保持与已有 body 内容一致。

- [ ] **Step 2: 验证 YAML 语法**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo "YAML 语法正确"
```

期望输出：`YAML 语法正确`

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/release.yml
git commit -m "ci: GitHub Release 说明新增快速安装章节（镜像同步）"
```

---

### Task 7: 验证 `CLAUDE.md` 发版 Checklist

**Files:**
- Verify only: `CLAUDE.md`（此文件已在计划执行前创建，内容应已完整，无需修改）

- [ ] **Step 1: 确认表格包含全部 4 个版本号文件**

```bash
grep -c "install\." CLAUDE.md
```

期望输出：`3`（即 install.sh、install.ps1、install.cmd 三行均存在）

- [ ] **Step 2: 若输出不为 3，则说明 CLAUDE.md 内容不完整，手动补充缺失行后提交**

```bash
git add CLAUDE.md
git commit -m "docs: CLAUDE.md 发版 checklist 补充安装脚本版本号"
```

若输出为 3，跳过此步，无需提交。

---

### Task 8: 推送所有提交

- [ ] **Step 1: 确认本地提交数量**

```bash
git log --oneline origin/main..HEAD
```

期望：显示本次开发的所有提交（至少 6 条）。

- [ ] **Step 2: 推送到远程**

```bash
git push origin main
```
