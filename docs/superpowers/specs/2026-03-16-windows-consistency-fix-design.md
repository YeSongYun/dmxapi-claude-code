# Windows 版本一致性与 Bug 修复设计文档

**日期**: 2026-03-16
**状态**: 已审批

---

## 背景

用户在 Windows PowerShell 7.5.4 上通过 `iwr | iex` 安装运行时，发现三类问题：

1. 界面文字显示乱码（install.ps1 中文 `?????`）
2. 功能 Bug（VSCode settings.json 解析失败）
3. 与 Mac 版本不统一（Logo 无 ASCII art、无版本号；模型菜单按 Esc 不清除）

---

## 变更清单（4 处修改）

### Fix 1：install.ps1 中文乱码

**文件**: `install.ps1`
**根因**: `Write-Host "正在下载..."` 在 exe 启动前执行，此时 PowerShell 输出编码为系统默认（GBK/OEM），而非 UTF-8。Go exe 内部调用 `SetConsoleOutputCP(65001)` 只影响 exe 自身进程，不影响 PowerShell 父进程。
**修复**: 在 install.ps1 顶部（第 1 行之后）添加：
```powershell
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
```
- `[Console]::OutputEncoding` 控制 `Write-Host` 的控制台输出编码
- `$OutputEncoding` 控制管道输出编码（防御性设置）

---

### Fix 2：Windows Logo 无 ASCII art 和版本号

**文件**: `dmxapi-claude-code.go`，函数 `printLogo()`，约第 229 行
**根因**: 代码中有 `if runtime.GOOS == "windows"` 早返回分支，显示简洁文字并跳过 ASCII art。该分支遗漏了版本号 `appVersion`，且无法与 Mac 版本保持视觉一致。
**修复**: 删除 Windows 专用早返回分支。Windows Terminal / PowerShell 7+ 完全支持 Unicode 方块字符（`█ ╗ ╔` 等），直接复用 Mac 的 ASCII art + 版本号显示逻辑即可。

删除以下代码块（约 230-235 行）：
```go
if runtime.GOOS == "windows" {
    fmt.Println()
    fmt.Println(colorCyan + styleBold + "  === DMXAPI ===" + colorReset)
    fmt.Println(styleDim + "  Claude Code CLI 配置工具" + colorReset)
    fmt.Printf("  %s%s/%s%s\n\n", colorMagenta, runtime.GOOS, runtime.GOARCH, colorReset)
    return
}
```

修复后，Windows 和 Mac 统一显示：
```
  ██████╗ ███╗   ███╗██╗...
  ...
  Claude Code CLI 配置工具  ·  让 AI 触手可及
  v1.4.9  windows/amd64
```

---

### Fix 3：VSCode settings.json JSONC 解析失败

**文件**: `dmxapi-claude-code.go`，函数 `mergeVSCodeSettings()`，约第 748 行
**根因**: VS Code 的 settings.json 是 JSONC 格式，允许：
- `// 单行注释`
- `/* 块注释 */`
- 尾随逗号（`{"key": "val",}`）

Go 标准库 `json.Unmarshal` 遇到这些格式报错，导致 VSCode 配置无法写入。
**错误信息**: `invalid character '}' looking for beginning of object key string`（尾随逗号典型报错）

**修复**: 新增 `stripJSONC(data []byte) []byte` 函数，使用字符级状态机解析，正确处理字符串内容（不误删字符串中的 `//` 或 `,`）：

状态机逻辑：
1. 遍历字节，追踪 `inString` 状态（处理 `\"` 转义）
2. 在字符串外，识别 `//`（跳到行尾）和 `/*`（跳到 `*/`）
3. 注释剥离后，用正则清除尾随逗号：`,(\s*[}\]])`

调用位置：在 `mergeVSCodeSettings` 中，`json.Unmarshal(existingJSON, ...)` 前先调用 `stripJSONC`：
```go
func mergeVSCodeSettings(existingJSON []byte, envVars []map[string]string) ([]byte, error) {
    cleaned := stripJSONC(existingJSON)  // ← 新增
    var settings map[string]interface{}
    if err := json.Unmarshal(cleaned, &settings); err != nil {
        return nil, fmt.Errorf("解析 settings.json 失败: %v", err)
    }
    ...
}
```

---

### Fix 4：L1 模型菜单按 Esc 不清除

**文件**: `dmxapi-claude-code.go`，函数 `runL1Menu()`，约第 1812 行
**根因**: `runItemMenu`（其他所有菜单）在 `KeyEnter` 时调用了 `clearMenuLines(linesPrinted)`，但 `runL1Menu` 的 `KeyEsc` 分支只调用了 `restore()` 和 `return`，遗漏了 `clearMenuLines`。导致模型配置菜单按 Esc 退出后残留在终端屏幕上（Mac 和 Windows 均有此问题）。

**修复**: 在 `KeyEsc` 的 `return` 前添加 `clearMenuLines(linesPrinted)`：
```go
case KeyEsc:
    restore()
    clearMenuLines(linesPrinted)  // ← 新增
    return
```

---

## 不修改的内容

- `checkClaudeCodeInstalled()` 和 `checkForUpdates()` 在 Windows 上工作正常（已通过用户实际运行验证），无需修改
- `install.cmd` 无需修改（CMD 默认支持中文输出，且已有对应的 Go exe 来处理）
- 模型默认值（`-cc` 后缀）不修改——Windows 用户显示的非 `-cc` 模型是从既有环境变量加载的，不是 bug

---

## 测试要点

1. Windows PowerShell 7+ 运行 install.ps1，确认下载/启动提示为中文而非 `?????`
2. Windows Terminal 中运行 exe，确认 ASCII art Logo 正确显示（方块字符不乱码）
3. 对包含 `//注释` 和尾随逗号的 settings.json 运行 VSCode 配置，确认写入成功
4. 在 L1 模型菜单按 Esc，确认菜单消失不残留

---

## 版本影响

修复后发布新版本时，需同步更新 `dmxapi-claude-code.go`、`install.sh`、`install.ps1`、`install.cmd` 中的版本号。
