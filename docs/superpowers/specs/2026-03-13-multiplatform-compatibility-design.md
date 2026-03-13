# 多平台兼容性优化设计文档

**日期**：2026-03-13
**项目**：dmxapi_claude_code
**版本**：v1.4.4 → v1.5.0
**状态**：已批准，待实现

---

## 背景

`dmxapi_claude_code` 是一个 Go 编写的 Claude Code CLI 跨平台配置工具，支持 Windows / Linux / macOS 三大平台，编译目标覆盖 5 种平台+架构组合。

本次优化针对代码审查中发现的 5 类多平台兼容性问题，进行精准修复，不引入新架构。

---

## 问题清单

| # | 问题描述 | 严重度 | 影响平台 |
|---|----------|--------|---------|
| 1 | Shell 检测硬编码：Linux 只写 `.bashrc`/`.profile`，zsh/fish 用户的配置无法写入 | 高 | Linux |
| 2 | `setx` 1024 字节上限：Windows 下 API Token 过长时被静默截断 | 高 | Windows |
| 3 | WSL 无区分：运行在 WSL 下按普通 Linux 处理，提示信息与实际不符 | 中 | WSL |
| 4 | `visibleLength` ANSI 解析缺陷：只识别 SGR（`m` 结尾）序列，其他控制码计入可见宽度导致对齐错乱 | 中 | 全平台 |
| 5 | `source` 提示固化：macOS 固定提示 `source ~/.zshrc`，bash 用户应提示 `source ~/.bash_profile` | 低 | macOS/Linux |

---

## 设计

### Fix 1：Shell 自动检测（解决问题 1 + 5）

**改动文件**：`dmxapi-claude-code.go`

#### 新增函数 `detectShellProfile()`

通过 `$SHELL` 环境变量判断用户当前 shell，返回：
- 要写入的配置文件路径列表
- 对应的 `source` 提示命令字符串

```
$SHELL 包含 "zsh"  →  ["~/.zshrc"]                          + "source ~/.zshrc"
$SHELL 包含 "fish" →  ["~/.config/fish/config.fish"]        + "source ~/.config/fish/config.fish"
$SHELL 包含 "bash" →  macOS: ["~/.bash_profile"]
                       Linux: ["~/.bashrc"]                  + "source ~/.bashrc / ~/.bash_profile"
$SHELL 为空或未知  →  回退：写全部常见文件（现有逻辑）
```

**返回结构**：
```go
type shellProfile struct {
    configFiles []string  // 相对于 HomeDir 的文件路径列表
    sourceCmd   string    // 提示用户执行的 source 命令
    isFish      bool      // 是否为 fish shell（写法不同）
}
```

#### fish shell 特殊处理

fish 不使用 `export KEY=VALUE`，而使用：
- 设置：`set -Ux KEY VALUE`
- 删除：`set -e KEY`
- 配置文件：`~/.config/fish/config.fish`

`setEnvVarsUnix` 和 `removeEnvVarUnix` 均需在 `isFish == true` 时切换到 fish 语法。

#### 影响函数

- `setEnvVarsUnix`：改为调用 `detectShellProfile()` 获取文件列表和格式
- `removeEnvVarUnix`：同步更新
- `configureAgentTeams`：末尾 `source` 提示改为动态文本（来自 `detectShellProfile().sourceCmd`）
- `printSummary`：同上

---

### Fix 2：Windows setx 长度限制（解决问题 2）

**改动文件**：`dmxapi-claude-code.go`

**改动函数**：`setEnvVarsWindows`

#### 逻辑

对每个值写入前检查字节长度：

```
len(value) > 900
  → REG ADD "HKCU\Environment" /V KEY /T REG_SZ /D VALUE /F
  → 成功：继续
  → 失败：返回明确错误，提示用户手动配置
否则
  → setx KEY "VALUE"（现有逻辑不变）
```

阈值设为 900 字节（而非 1024），留有余量应对编码膨胀。

注册表写入（`REG ADD HKCU\Environment`）与 `setx` 效果等价，且无长度限制。

---

### Fix 3：WSL 检测与提示（解决问题 3）

**改动文件**：`dmxapi-claude-code.go`

#### 新增函数 `isWSL()`

```go
func isWSL() bool {
    data, err := os.ReadFile("/proc/version")
    if err != nil { return false }
    lower := strings.ToLower(string(data))
    return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}
```

#### 使用位置

`printSummary` 和 `configureAgentTeams` 末尾，WSL 环境下额外显示：

```
◆ 注意：WSL 环境下，环境变量仅在当前 WSL 会话有效
  若需要 Windows 侧程序读取，请在 Windows 侧单独配置
```

---

### Fix 4：ANSI 解析修复（解决问题 4）

**改动文件**：`dmxapi-claude-code.go`
**改动函数**：`visibleLength`

#### 现状

状态机仅在遇到 `m` 时退出转义状态，导致 `\033[2K`（清行）、`\033[A`（光标上移）等序列的终止字母被计入可见宽度。

#### 修复

ANSI CSI 序列终止字节范围为 `0x40–0x7E`（标准定义），修改判断条件：

```go
// 修改前
if r == 'm' { inEscape = false }

// 修改后
if r >= 0x40 && r <= 0x7E { inEscape = false }
```

---

## 影响评估

| 改动 | 涉及函数 | 新增函数 | 风险 |
|------|---------|---------|------|
| Fix 1 Shell 检测 | `setEnvVarsUnix`, `removeEnvVarUnix`, `configureAgentTeams`, `printSummary` | `detectShellProfile()` | 低（有回退逻辑） |
| Fix 2 setx 限制 | `setEnvVarsWindows` | 无 | 低（仅添加分支） |
| Fix 3 WSL 检测 | `printSummary`, `configureAgentTeams` | `isWSL()` | 极低（只读文件） |
| Fix 4 ANSI 修复 | `visibleLength` | 无 | 极低（1 行改动） |

---

## 不在范围内

- fish shell 支持：设计中已包含架构，但需用户明确要求才实现（当前先实现检测+提示，写入改用标准 `export` 或提示用户手动）
- macOS Gatekeeper 提示：已在 CI release notes 中处理，无需在工具内重复
- `build.sh` 补充：独立任务，不在本次范围
- Windows ARM64 支持：当前无此编译目标，不在本次范围

---

## 测试要点

- [ ] macOS zsh 用户：写入 `~/.zshrc`，提示 `source ~/.zshrc`
- [ ] macOS bash 用户（`$SHELL=/bin/bash`）：写入 `~/.bash_profile`，提示 `source ~/.bash_profile`
- [ ] Linux bash 用户：写入 `~/.bashrc`
- [ ] Linux zsh 用户：写入 `~/.zshrc`
- [ ] 未知 shell（`$SHELL` 为空）：回退写全部文件
- [ ] Windows：短 token（<900字节）用 `setx`，长 token 用 `REG ADD`
- [ ] WSL：`isWSL()` 返回 true，显示额外提示
- [ ] 非 WSL Linux：`isWSL()` 返回 false，不显示额外提示
- [ ] `visibleLength`：含 `\033[2K` 的字符串宽度计算正确
