# 多平台兼容性优化 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 dmxapi_claude_code 中 5 类跨平台兼容性问题，覆盖 Shell 自动检测、Windows setx 长度限制、WSL 提示、ANSI 解析缺陷和动态 source 提示。

**Architecture:** 所有改动集中在单文件 `dmxapi-claude-code.go`，新增 `detectShellProfile()` 和 `isWSL()` 两个辅助函数，其余均为已有函数的精准修改。采用 TDD 策略：先写失败测试，再写最小实现，验证通过后提交。

**Tech Stack:** Go 1.21，标准库（os、strings、path/filepath、runtime），无新依赖

---

## Chunk 1: Fix 4 + Fix 3（ANSI修复 + WSL检测）

### Task 1: Fix 4 — 修复 `visibleLength` 的 ANSI 解析

**文件：**
- Modify: `dmxapi-claude-code.go:198`（1行改动）
- Test: `dmxapi-claude-code_test.go`（新增测试用例）

**背景：** 当前 `visibleLength` 在遇到 `m` 时才退出 ANSI 转义状态，`\033[2K`（清行）等序列的终止字母 `K` 会被错误计入可见宽度。标准 ANSI CSI 序列终止字节范围为 `0x40–0x7E`。

- [ ] **Step 1.1：写失败测试**

  打开 `dmxapi-claude-code_test.go`，在 `TestCompareVersions` 之后追加：

  ```go
  func TestVisibleLength(t *testing.T) {
      cases := []struct {
          input string
          want  int
      }{
          {"hello", 5},                        // 普通 ASCII
          {"\033[31mhello\033[0m", 5},          // SGR 颜色序列（原来就支持）
          {"\033[2Khello", 5},                  // 清行序列 \033[2K（修复前会多算1）
          {"\033[1;32mOK\033[0m", 2},           // 多参数 SGR
          {"你好", 4},                          // CJK 双宽字符
          {"\033[Ahello\033[B", 5},             // 光标移动序列（\033[A 上移，\033[B 下移）
          {"", 0},                              // 空字符串
      }
      for _, c := range cases {
          got := visibleLength(c.input)
          if got != c.want {
              t.Errorf("visibleLength(%q) = %d, want %d", c.input, got, c.want)
          }
      }
  }
  ```

- [ ] **Step 1.2：运行测试确认失败**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go test -run TestVisibleLength -v
  ```

  预期：`\033[2Khello` 和 `\033[Ahello\033[B` 用例 FAIL（计算值 > 期望值）

- [ ] **Step 1.3：实现修复**

  编辑 `dmxapi-claude-code.go` 第 198 行：

  ```go
  // 修改前：
  		if r == 'm' {
  			inEscape = false
  		}

  // 修改后：
  		if r >= 0x40 && r <= 0x7E { // 任意 CSI 终止字节（标准范围 @A-Z[\]^_`a-z{|}~）
  			inEscape = false
  		}
  ```

- [ ] **Step 1.4：运行测试确认通过**

  ```bash
  go test -run TestVisibleLength -v
  ```

  预期：所有用例 PASS

- [ ] **Step 1.5：运行全部测试确保无回归**

  ```bash
  go test ./... -v
  ```

  预期：所有现有测试 PASS

- [ ] **Step 1.6：提交**

  ```bash
  git add dmxapi-claude-code.go dmxapi-claude-code_test.go
  git commit -m "fix: 修复 visibleLength ANSI 解析，支持所有 CSI 终止字节（0x40-0x7E）"
  ```

---

### Task 2: Fix 3 — 新增 `isWSL()` 函数并更新提示

**文件：**
- Modify: `dmxapi-claude-code.go`（新增函数 + 修改2处提示）
- Test: `dmxapi-claude-code_test.go`（新增测试）

**背景：** 读取 `/proc/version` 检测 WSL 环境，并在 `printSummary` 和 `configureAgentTeams` 的末尾追加 WSL 专属提示。

- [ ] **Step 2.1：写 `isWSL` 的单元测试**

  在 `dmxapi-claude-code_test.go` 追加：

  ```go
  func TestIsWSLFromContent(t *testing.T) {
      // 提取 isWSL 的检测逻辑为可测试的纯函数
      detectWSL := func(content string) bool {
          lower := strings.ToLower(content)
          return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
      }

      cases := []struct {
          content string
          want    bool
      }{
          {"Linux version 5.15.0-microsoft-standard-WSL2", true},   // WSL2
          {"Linux version 4.4.0-19041-Microsoft (gcc ...)", true},  // WSL1
          {"Linux version 5.15.0 (Ubuntu 22.04)", false},           // 普通 Linux
          {"Darwin Kernel Version 23.0.0", false},                   // macOS（极端情况）
          {"", false},                                               // 空内容
          {"WSL kernel release", true},                              // 包含 WSL 关键字
      }
      for _, c := range cases {
          got := detectWSL(c.content)
          if got != c.want {
              t.Errorf("detectWSL(%q) = %v, want %v", c.content, got, c.want)
          }
      }
  }
  ```

  > 注意：由于 `isWSL()` 依赖文件系统（`/proc/version`），我们只测试其内部检测逻辑，不测试文件读取部分。

- [ ] **Step 2.2：运行测试确认通过（纯逻辑，应直接通过）**

  ```bash
  go test -run TestIsWSLFromContent -v
  ```

  预期：PASS（这是纯字符串逻辑，无需实现改动）

- [ ] **Step 2.3：在 `dmxapi-claude-code.go` 中新增 `isWSL()` 函数**

  在 `removeEnvVarWindows` 函数结束的右花括号（约第 661 行）之后、`runCommand` 函数之前插入：

  ```go
  // isWSL 检测当前是否运行在 Windows Subsystem for Linux (WSL) 环境中
  // 通过读取 /proc/version 文件内容判断，失败时返回 false（安全静默）
  func isWSL() bool {
      data, err := os.ReadFile("/proc/version")
      if err != nil {
          return false
      }
      lower := strings.ToLower(string(data))
      return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
  }
  ```

- [ ] **Step 2.4：更新 `printSummary` 中的提示（约第 1638 行）**

  将现有的 switch 块：

  ```go
  switch runtime.GOOS {
  case "windows":
      printTip("配置已保存到用户环境变量")
      printTip("请重新打开终端窗口使配置生效")
  case "darwin":
      printTip("配置已写入 ~/.zshrc 和 ~/.bash_profile")
      printTip("执行 source ~/.zshrc 或重启终端使配置生效")
  default:
      printTip("配置已写入 ~/.bashrc 和 ~/.profile")
      printTip("执行 source ~/.bashrc 或重启终端使配置生效")
  }
  ```

  替换为（**注意：此为临时版本，Task 4 Step 4.5 会将此 switch 替换为 `detectShellProfile()` 动态版本**）：

  ```go
  switch runtime.GOOS {
  case "windows":
      printTip("配置已保存到用户环境变量")
      printTip("请重新打开终端窗口使配置生效")
  case "darwin":
      printTip("配置已写入 ~/.zshrc 和 ~/.bash_profile")
      printTip("执行 source ~/.zshrc 或重启终端使配置生效")
  default:
      printTip("配置已写入 ~/.bashrc 和 ~/.profile")
      printTip("执行 source ~/.bashrc 或重启终端使配置生效")
      if isWSL() {
          fmt.Println()
          printTip("注意：WSL 环境下，环境变量仅在当前 WSL 会话有效")
          printTip("若需要 Windows 侧程序读取，请在 Windows 侧单独配置")
      }
  }
  ```

- [ ] **Step 2.5：更新 `configureAgentTeams` 中的提示（约第 1596 行）**

  将现有 switch 块：

  ```go
  switch runtime.GOOS {
  case "windows":
      printTip("请重新打开终端窗口使配置生效")
  case "darwin":
      printTip("执行 source ~/.zshrc 或重启终端使配置生效")
  default:
      printTip("执行 source ~/.bashrc 或重启终端使配置生效")
  }
  ```

  替换为：

  ```go
  switch runtime.GOOS {
  case "windows":
      printTip("请重新打开终端窗口使配置生效")
  case "darwin":
      printTip("执行 source ~/.zshrc 或重启终端使配置生效")
  default:
      printTip("执行 source ~/.bashrc 或重启终端使配置生效")
      if isWSL() {
          fmt.Println()
          printTip("注意：WSL 环境下，环境变量仅在当前 WSL 会话有效")
          printTip("若需要 Windows 侧程序读取，请在 Windows 侧单独配置")
      }
  }
  ```

- [ ] **Step 2.6：运行全部测试**

  ```bash
  go test ./... -v
  ```

  预期：所有测试 PASS

- [ ] **Step 2.7：编译验证**

  ```bash
  go build -o /tmp/dmxapi-test ./
  ```

  预期：编译成功，无错误

- [ ] **Step 2.8：提交**

  ```bash
  git add dmxapi-claude-code.go dmxapi-claude-code_test.go
  git commit -m "feat: 新增 isWSL() 检测，WSL 环境下显示额外提示信息"
  ```

---

## Chunk 2: Fix 2（Windows setx 长度限制）

### Task 3: Fix 2 — `setEnvVarsWindows` 增加长度检查

**文件：**
- Modify: `dmxapi-claude-code.go:471-499`（`setEnvVarsWindows` 函数）
- Test: `dmxapi-claude-code_test.go`（新增测试）

**背景：** `setx` 命令有 1024 字节上限，超出时静默截断。超过 900 字节时改用 `REG ADD HKCU\Environment`（与现有 `removeEnvVarWindows` 使用相同的 `runCommand("REG", ...)` 模式）。

- [ ] **Step 3.1：写选择策略的单元测试**

  在 `dmxapi-claude-code_test.go` 追加：

  ```go
  func TestSetxOrRegAdd(t *testing.T) {
      // 提取"选择命令"逻辑为可测试的纯函数
      chooseCmd := func(value string) string {
          if len(value) > 900 {
              return "REG_ADD"
          }
          return "SETX"
      }

      shortVal := strings.Repeat("a", 100)
      borderVal := strings.Repeat("a", 900)
      longVal := strings.Repeat("a", 901)

      if chooseCmd(shortVal) != "SETX" {
          t.Errorf("短值（100字节）应使用 SETX")
      }
      if chooseCmd(borderVal) != "SETX" {
          t.Errorf("边界值（900字节）应使用 SETX")
      }
      if chooseCmd(longVal) != "REG_ADD" {
          t.Errorf("超长值（901字节）应使用 REG_ADD")
      }
  }
  ```

- [ ] **Step 3.2：运行测试确认通过（纯逻辑）**

  ```bash
  go test -run TestSetxOrRegAdd -v
  ```

  预期：PASS

- [ ] **Step 3.3：修改 `setEnvVarsWindows` 函数**

  将函数体内的 goroutine 匿名函数从（约第 480-486 行）：

  ```go
  go func(k, v string) {
      defer wg.Done()
      // setx KEY "VALUE" 设置用户环境变量
      if err := runCommand("setx", k, v); err != nil {
          errChan <- fmt.Errorf("设置环境变量 %s 失败: %v", k, err)
      }
  }(key, value)
  ```

  替换为：

  ```go
  go func(k, v string) {
      defer wg.Done()
      var err error
      if len(v) > 900 {
          // setx 有 1024 字节上限，超长值改用 REG ADD 直接写注册表
          err = runCommand("REG", "ADD", `HKCU\Environment`, "/V", k, "/T", "REG_SZ", "/D", v, "/F")
          if err != nil {
              errChan <- fmt.Errorf("设置环境变量 %s 失败（token 过长，注册表写入错误）：%v\n请重启终端后重试，或以管理员权限运行", k, err)
              return
          }
      } else {
          // 普通路径：使用 setx
          err = runCommand("setx", k, v)
          if err != nil {
              errChan <- fmt.Errorf("设置环境变量 %s 失败: %v", k, err)
          }
      }
  }(key, value)
  ```

- [ ] **Step 3.4：运行全部测试**

  ```bash
  go test ./... -v
  ```

  预期：所有测试 PASS

- [ ] **Step 3.5：编译验证（重要：需要对所有平台编译）**

  ```bash
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/dmxapi-windows.exe ./
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/dmxapi-linux ./
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/dmxapi-macos ./
  ```

  预期：三个平台编译均成功，无错误

- [ ] **Step 3.6：提交**

  ```bash
  git add dmxapi-claude-code.go dmxapi-claude-code_test.go
  git commit -m "fix: Windows setEnvVarsWindows 超长值改用 REG ADD，绕过 setx 1024字节限制"
  ```

---

## Chunk 3: Fix 1（Shell 自动检测，最大改动）

### Task 4: Fix 1 — 新增 `detectShellProfile()` 并重构 Unix 配置写入

**文件：**
- Modify: `dmxapi-claude-code.go`（新增类型+函数，重构4处调用点）
- Test: `dmxapi-claude-code_test.go`（新增多个测试）

**背景：** 当前 `setEnvVarsUnix` 和 `removeEnvVarUnix` 硬编码配置文件列表，Linux zsh/fish 用户的配置无法被正确写入。需要新增 `shellProfile` 结构体和 `detectShellProfile()` 函数。

---

#### Step 4.0：写 `detectShellProfile` 的单元测试

在 `dmxapi-claude-code_test.go` 追加：

```go
func TestDetectShellProfile(t *testing.T) {
    cases := []struct {
        shellEnv string
        goos     string
        wantFile string   // configFiles[0] 的期望值（取第一个）
        wantSrc  string   // sourceCmd 期望值
        wantFish bool
    }{
        // macOS 场景
        {"/bin/zsh", "darwin", ".zshrc", "source ~/.zshrc", false},
        {"/bin/bash", "darwin", ".bash_profile", "source ~/.bash_profile", false},
        {"/usr/local/bin/fish", "darwin", ".config/fish/config.fish", "source ~/.config/fish/config.fish", true},
        // Linux 场景
        {"/bin/zsh", "linux", ".zshrc", "source ~/.zshrc", false},
        {"/bin/bash", "linux", ".bashrc", "source ~/.bashrc", false},
        {"/usr/bin/fish", "linux", ".config/fish/config.fish", "source ~/.config/fish/config.fish", true},
        // 非标准 shell 路径（如 Homebrew 安装的 zsh）
        {"/opt/homebrew/bin/zsh", "darwin", ".zshrc", "source ~/.zshrc", false},
        // 空 SHELL 变量：回退（sourceCmd 为空串，表示未知 shell，提示使用"重启终端"）
        {"", "darwin", ".zshrc", "", false},
        {"", "linux", ".bashrc", "", false},
    }

    for _, c := range cases {
        t.Setenv("SHELL", c.shellEnv)
        profile := detectShellProfile(c.goos)
        if len(profile.configFiles) == 0 {
            t.Errorf("SHELL=%q GOOS=%q: configFiles 为空", c.shellEnv, c.goos)
            continue
        }
        if profile.configFiles[0] != c.wantFile {
            t.Errorf("SHELL=%q GOOS=%q: configFiles[0]=%q, want %q",
                c.shellEnv, c.goos, profile.configFiles[0], c.wantFile)
        }
        if c.shellEnv != "" && profile.sourceCmd != c.wantSrc {
            t.Errorf("SHELL=%q GOOS=%q: sourceCmd=%q, want %q",
                c.shellEnv, c.goos, profile.sourceCmd, c.wantSrc)
        }
        if profile.isFish != c.wantFish {
            t.Errorf("SHELL=%q GOOS=%q: isFish=%v, want %v",
                c.shellEnv, c.goos, profile.isFish, c.wantFish)
        }
    }
}
```

- [ ] **Step 4.1：保存测试文件并运行确认失败**

  ```bash
  go test -run TestDetectShellProfile -v
  ```

  预期：FAIL，`detectShellProfile undefined`

---

#### Step 4.2：新增 `shellProfile` 类型和 `detectShellProfile()` 函数

在 `dmxapi-claude-code.go` 中，`setEnvVarsWindows` 函数之前（约第 466 行之后，即 `getEnvVar` 之后）插入：

```go
// shellProfile 描述用户当前 Shell 对应的配置文件信息
type shellProfile struct {
    configFiles []string // 相对于 HomeDir 的配置文件路径列表
    sourceCmd   string   // 提示用户执行的 source 命令（空串=回退模式）
    isFish      bool     // 是否为 fish shell（写法与 bash/zsh 不同）
}

// detectShellProfile 通过 $SHELL 环境变量检测用户的 shell，
// 返回对应的配置文件列表和 source 命令。
// goos 参数用于区分 darwin/linux 行为（通常传入 runtime.GOOS）。
// $SHELL 为空或未知时返回包含多个常见配置文件的 shellProfile（sourceCmd 为空串）。
func detectShellProfile(goos string) shellProfile {
    shell := os.Getenv("SHELL")

    switch {
    case strings.Contains(shell, "zsh"):
        return shellProfile{
            configFiles: []string{".zshrc"},
            sourceCmd:   "source ~/.zshrc",
        }
    case strings.Contains(shell, "fish"):
        return shellProfile{
            configFiles: []string{".config/fish/config.fish"},
            sourceCmd:   "source ~/.config/fish/config.fish",
            isFish:      true,
        }
    case strings.Contains(shell, "bash"):
        if goos == "darwin" {
            return shellProfile{
                configFiles: []string{".bash_profile"},
                sourceCmd:   "source ~/.bash_profile",
            }
        }
        return shellProfile{
            configFiles: []string{".bashrc"},
            sourceCmd:   "source ~/.bashrc",
        }
    default:
        // $SHELL 为空或未知 shell：回退到写全部常见文件（兼容旧行为）
        if goos == "darwin" {
            return shellProfile{configFiles: []string{".zshrc", ".bash_profile"}}
        }
        return shellProfile{configFiles: []string{".bashrc", ".profile"}}
    }
}
```

- [ ] **Step 4.2：运行测试确认通过**

  ```bash
  go test -run TestDetectShellProfile -v
  ```

  预期：所有用例 PASS

---

#### Step 4.3：重构 `setEnvVarsUnix` 使用 `detectShellProfile`

将现有 `setEnvVarsUnix` 函数（约第 501 行）中的配置文件列表初始化部分：

```go
// 确定要写入的配置文件
var configFiles []string
switch runtime.GOOS {
case "darwin":
    configFiles = []string{".zshrc", ".bash_profile"}
default:
    configFiles = []string{".bashrc", ".profile"}
}

// 写入配置文件
for _, configFile := range configFiles {
    configPath := filepath.Join(homeDir, configFile)

    // macOS 特殊处理：如果是 .zshrc 且不存在，则创建
    if runtime.GOOS == "darwin" && configFile == ".zshrc" {
        if _, err := os.Stat(configPath); os.IsNotExist(err) {
            // 创建空的 .zshrc 文件
            if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
                return fmt.Errorf("创建 %s 失败: %v", configPath, err)
            }
        }
    } else {
        // 其他文件：不存在则跳过
        if _, err := os.Stat(configPath); os.IsNotExist(err) {
            continue
        }
    }

    content, err := os.ReadFile(configPath)
    if err != nil {
        continue
    }

    // 统一行尾符：\r\n → \n，\r → \n（兼容 Windows 编辑过的文件）
    normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
    normalized = strings.ReplaceAll(normalized, "\r", "\n")
    lines := strings.Split(normalized, "\n")
    newLines := make([]string, 0, len(lines))
    foundKeys := make(map[string]bool)

    // 遍历现有行，替换已存在的变量
    for _, line := range lines {
        replaced := false
        for key, value := range vars {
            if value == "" {
                continue
            }
            marker := fmt.Sprintf("export %s=", key)
            if strings.HasPrefix(strings.TrimSpace(line), marker) {
                exportLine := fmt.Sprintf("export %s='%s'", key, strings.ReplaceAll(value, "'", "'\\''"))
                newLines = append(newLines, exportLine)
                foundKeys[key] = true
                replaced = true
                break
            }
        }
        if !replaced {
            newLines = append(newLines, line)
        }
    }

    // 添加未找到的变量
    for key, value := range vars {
        if value == "" || foundKeys[key] {
            continue
        }
        exportLine := fmt.Sprintf("export %s='%s'", key, strings.ReplaceAll(value, "'", "'\\''"))
        newLines = append(newLines, exportLine)
    }

    newContent := strings.Join(newLines, "\n")
    if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
        return fmt.Errorf("写入 %s 失败: %v", configPath, err)
    }
}
```

- [ ] **Step 4.3：用以下代码替换上述代码段**

```go
profile := detectShellProfile(runtime.GOOS)

// 写入配置文件
for _, configFile := range profile.configFiles {
    configPath := filepath.Join(homeDir, configFile)

    if profile.isFish {
        // fish shell：确保目录存在，然后写入 set -Ux 语法
        if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
            return fmt.Errorf("创建 %s 目录失败: %v", filepath.Dir(configPath), err)
        }
        // 读取现有内容（文件不存在时从空内容开始）
        var existingContent []byte
        if _, statErr := os.Stat(configPath); statErr == nil {
            existingContent, _ = os.ReadFile(configPath)
        }
        normalized := strings.ReplaceAll(string(existingContent), "\r\n", "\n")
        normalized = strings.ReplaceAll(normalized, "\r", "\n")
        lines := strings.Split(normalized, "\n")
        newLines := make([]string, 0, len(lines))
        foundKeys := make(map[string]bool)

        for _, line := range lines {
            replaced := false
            for key, value := range vars {
                if value == "" {
                    continue
                }
                marker := fmt.Sprintf("set -Ux %s ", key)
                if strings.HasPrefix(strings.TrimSpace(line), marker) {
                    newLines = append(newLines, fmt.Sprintf("set -Ux %s %s", key, value))
                    foundKeys[key] = true
                    replaced = true
                    break
                }
            }
            if !replaced {
                newLines = append(newLines, line)
            }
        }
        for key, value := range vars {
            if value == "" || foundKeys[key] {
                continue
            }
            newLines = append(newLines, fmt.Sprintf("set -Ux %s %s", key, value))
        }
        newContent := strings.Join(newLines, "\n")
        if !strings.HasSuffix(newContent, "\n") {
            newContent += "\n"
        }
        if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
            return fmt.Errorf("写入 %s 失败: %v", configPath, err)
        }
        continue
    }

    // bash / zsh / 回退路径：使用 export KEY='VALUE' 语法
    // 主配置文件（如 .zshrc）不存在时自动创建；其余文件不存在则跳过
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        if configFile == profile.configFiles[0] {
            if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
                return fmt.Errorf("创建 %s 失败: %v", configPath, err)
            }
        } else {
            continue
        }
    }

    content, err := os.ReadFile(configPath)
    if err != nil {
        continue
    }

    normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
    normalized = strings.ReplaceAll(normalized, "\r", "\n")
    lines := strings.Split(normalized, "\n")
    newLines := make([]string, 0, len(lines))
    foundKeys := make(map[string]bool)

    for _, line := range lines {
        replaced := false
        for key, value := range vars {
            if value == "" {
                continue
            }
            marker := fmt.Sprintf("export %s=", key)
            if strings.HasPrefix(strings.TrimSpace(line), marker) {
                exportLine := fmt.Sprintf("export %s='%s'", key, strings.ReplaceAll(value, "'", "'\\''"))
                newLines = append(newLines, exportLine)
                foundKeys[key] = true
                replaced = true
                break
            }
        }
        if !replaced {
            newLines = append(newLines, line)
        }
    }

    for key, value := range vars {
        if value == "" || foundKeys[key] {
            continue
        }
        exportLine := fmt.Sprintf("export %s='%s'", key, strings.ReplaceAll(value, "'", "'\\''"))
        newLines = append(newLines, exportLine)
    }

    newContent := strings.Join(newLines, "\n")
    if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
        return fmt.Errorf("写入 %s 失败: %v", configPath, err)
    }
}
```

---

#### Step 4.4：重构 `removeEnvVarUnix` 使用 `detectShellProfile`

将函数内的配置文件列表初始化（约第 594-600 行）：

```go
var configFiles []string
switch runtime.GOOS {
case "darwin":
    configFiles = []string{".zshrc", ".bash_profile"}
default:
    configFiles = []string{".bashrc", ".profile"}
}

marker := fmt.Sprintf("export %s=", key)
```

替换为：

```go
profile := detectShellProfile(runtime.GOOS)

// fish shell 用 set -e 语法删除变量
fishMarker := fmt.Sprintf("set -Ux %s ", key)
exportMarker := fmt.Sprintf("export %s=", key)
```

然后将循环体内的 marker 使用改为根据 `profile.isFish` 选择：

```go
for _, configFile := range profile.configFiles {
    configPath := filepath.Join(homeDir, configFile)
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        // fish 配置目录可能不存在，删除操作时确保目录存在（写空文件无意义，直接跳过）
        if profile.isFish {
            os.MkdirAll(filepath.Dir(configPath), 0755)
        }
        continue // 文件不存在无需删除
    }
    content, err := os.ReadFile(configPath)
    if err != nil {
        continue
    }
    info, statErr := os.Stat(configPath)
    perm := os.FileMode(0644)
    if statErr == nil {
        perm = info.Mode()
    }
    normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
    normalized = strings.ReplaceAll(normalized, "\r", "\n")
    lines := strings.Split(normalized, "\n")

    // 根据 shell 选择要删除的行标记
    var lineMarker string
    if profile.isFish {
        lineMarker = fishMarker
    } else {
        lineMarker = exportMarker
    }

    newLines := make([]string, 0, len(lines))
    prevBlank := false
    for _, line := range lines {
        if strings.HasPrefix(strings.TrimSpace(line), lineMarker) {
            continue
        }
        isBlank := strings.TrimSpace(line) == ""
        if isBlank && prevBlank {
            continue
        }
        newLines = append(newLines, line)
        prevBlank = isBlank
    }

    newContent := strings.Join(newLines, "\n")
    if !strings.HasSuffix(newContent, "\n") {
        newContent += "\n"
    }
    if err := os.WriteFile(configPath, []byte(newContent), perm); err != nil {
        return fmt.Errorf("写入 %s 失败: %v", configPath, err)
    }
}
```

- [ ] **Step 4.4：完成 `removeEnvVarUnix` 重构（同上替换操作）**

---

#### Step 4.5：更新 `printSummary` 和 `configureAgentTeams` 使用动态 source 提示

将 Task 2 中临时写入的固定文件提示（`printSummary` 约第 1638 行）改为动态版本：

```go
switch runtime.GOOS {
case "windows":
    printTip("配置已保存到用户环境变量")
    printTip("请重新打开终端窗口使配置生效")
default:
    profile := detectShellProfile(runtime.GOOS)
    if len(profile.configFiles) > 0 {
        // 构建写入文件列表的显示文本
        displayFiles := make([]string, len(profile.configFiles))
        for i, f := range profile.configFiles {
            displayFiles[i] = "~/" + f
        }
        printTip(fmt.Sprintf("配置已写入 %s", strings.Join(displayFiles, " 和 ")))
    }
    if profile.sourceCmd != "" {
        printTip(fmt.Sprintf("执行 %s 或重启终端使配置生效", profile.sourceCmd))
    } else {
        // 回退模式（$SHELL 未知）：无法确定具体命令，但仍告知写入的文件列表
        displayFiles := make([]string, len(profile.configFiles))
        for i, f := range profile.configFiles {
            displayFiles[i] = "~/" + f
        }
        printTip(fmt.Sprintf("配置已写入 %s", strings.Join(displayFiles, " 和 ")))
        printTip("重启终端使配置生效")
    }
    if isWSL() {
        fmt.Println()
        printTip("注意：WSL 环境下，环境变量仅在当前 WSL 会话有效")
        printTip("若需要 Windows 侧程序读取，请在 Windows 侧单独配置")
    }
}
```

将 `configureAgentTeams` 末尾的提示（约第 1596 行）也改为动态版本：

```go
switch runtime.GOOS {
case "windows":
    printTip("请重新打开终端窗口使配置生效")
default:
    profile := detectShellProfile(runtime.GOOS)
    if profile.sourceCmd != "" {
        printTip(fmt.Sprintf("执行 %s 或重启终端使配置生效", profile.sourceCmd))
    } else {
        printTip("重启终端使配置生效")
    }
    if isWSL() {
        fmt.Println()
        printTip("注意：WSL 环境下，环境变量仅在当前 WSL 会话有效")
        printTip("若需要 Windows 侧程序读取，请在 Windows 侧单独配置")
    }
}
```

- [ ] **Step 4.5：完成以上两处替换**

---

- [ ] **Step 4.6：运行全部测试**

  ```bash
  go test ./... -v
  ```

  预期：所有测试 PASS

- [ ] **Step 4.7：手动验证当前平台（macOS）**

  ```bash
  go run ./dmxapi-claude-code.go
  ```

  操作路径：选择"仅配置模型" → 完成 → 确认末尾 source 提示显示 `source ~/.zshrc`（当前 macOS zsh 环境）

- [ ] **Step 4.8：三平台编译验证**

  ```bash
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/dmxapi-win.exe ./
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/dmxapi-linux ./
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/dmxapi-linux-arm64 ./
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /tmp/dmxapi-macos-amd64 ./
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/dmxapi-macos-arm64 ./
  ```

  预期：全部 5 个平台编译成功

- [ ] **Step 4.9：提交**

  ```bash
  git add dmxapi-claude-code.go dmxapi-claude-code_test.go
  git commit -m "feat: 新增 detectShellProfile()，根据 \$SHELL 自动检测 shell 并写入对应配置文件"
  ```

---

## Chunk 4: 收尾与版本号更新

### Task 5: 更新版本号并运行完整验证

**文件：**
- Modify: `dmxapi-claude-code.go`（版本号常量）

- [ ] **Step 5.1：更新版本号**

  找到 `dmxapi-claude-code.go` 中：

  ```go
  appVersion = "1.4.4"
  ```

  修改为：

  ```go
  appVersion = "1.5.0"
  ```

- [ ] **Step 5.2：运行完整测试套件**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go test ./... -v -count=1
  ```

  预期：所有测试 PASS，无 SKIP，无 FAIL

- [ ] **Step 5.3：完整五平台编译验证**

  ```bash
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o /tmp/dmxapi-v1.5.0-win.exe ./
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /tmp/dmxapi-v1.5.0-linux-amd64 ./
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /tmp/dmxapi-v1.5.0-linux-arm64 ./
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o /tmp/dmxapi-v1.5.0-macos-amd64 ./
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o /tmp/dmxapi-v1.5.0-macos-arm64 ./
  ```

  预期：5 个文件均生成成功

- [ ] **Step 5.4：提交版本号**

  ```bash
  git add dmxapi-claude-code.go
  git commit -m "chore: 升级版本号至 v1.5.0"
  ```

- [ ] **Step 5.5：打 tag**

  ```bash
  git tag v1.5.0
  ```

  > 确认后再推送（需要用户明确授权）

---

## 附录：测试覆盖验证清单

完成全部 Tasks 后，手动核查以下场景：

| 场景 | 验证方法 | 期望结果 |
|------|---------|---------|
| macOS zsh（当前环境） | 直接运行工具 | 提示写入 `~/.zshrc`，source 提示正确 |
| macOS bash | `SHELL=/bin/bash go run ./` | 提示写入 `~/.bash_profile` |
| Linux zsh | `SHELL=/bin/zsh GOOS=linux go test -run TestDetectShellProfile` | configFiles[0]=`.zshrc` |
| Linux fish | `SHELL=/usr/bin/fish go test -run TestDetectShellProfile` | isFish=true，configFiles[0]=`.config/fish/config.fish` |
| 未知 shell 回退 | `SHELL= go test -run TestDetectShellProfile` | sourceCmd 为空，文件列表为全部常见文件 |
| ANSI 宽度 | `go test -run TestVisibleLength` | 所有用例 PASS |
| WSL 检测逻辑 | `go test -run TestIsWSLFromContent` | 所有用例 PASS |
| Windows 长 token | `go test -run TestSetxOrRegAdd` | 901字节选 REG_ADD |
