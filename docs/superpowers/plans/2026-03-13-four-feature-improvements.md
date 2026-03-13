# 四项功能改进 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 dmxapi-claude-code 配置工具新增 API 模型检测、Claude Code 安装检测、Agent Teams 配置、自动更新检查四项功能。

**Architecture:** 所有改动均在 `dmxapi-claude-code.go` 单文件内完成。功能1修改现有 API 验证逻辑；功能2/4在 main() 启动流程头部各插入一次检查；功能3在主菜单新增选项4并实现独立配置流程。

**Tech Stack:** Go 1.21+，标准库（`net/http`、`os/exec`、`regexp`、`strings`、`strconv`）

---

## Chunk 1：功能1 — API 检测使用用户配置的默认模型

**Files:**
- Modify: `dmxapi-claude-code.go:603-673`（`validateAPIConnection` 函数）
- Modify: `dmxapi-claude-code.go:771-793`（`selectFixOption` 函数）
- Modify: `dmxapi-claude-code.go:1418-1448`（`main()` 验证循环）

### Task 1：修改 `validateAPIConnection` 函数签名和请求体

- [ ] **Step 1：修改函数签名，增加 `model` 参数**

  找到第 603 行的函数定义，将签名从：
  ```go
  func validateAPIConnection(baseURL, authToken string) error {
  ```
  改为：
  ```go
  func validateAPIConnection(baseURL, authToken, model string) error {
  ```

- [ ] **Step 2：将请求体中硬编码的模型名替换为参数**

  找到第 609 行的 `requestBody`，将：
  ```go
  "model": "claude-haiku-4-5-20251001",
  ```
  改为：
  ```go
  "model": model,
  ```

- [ ] **Step 3：更新 HTTP 404 的错误提示**

  找到第 658 行的 404 处理：
  ```go
  case 404:
      return fmt.Errorf("API 端点不存在: 请检查 Base URL 是否正确")
  ```
  改为：
  ```go
  case 404:
      return fmt.Errorf("API 端点不存在或模型名称不正确: 请检查 Base URL 和模型名")
  ```

- [ ] **Step 4：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：出现编译错误，提示 `validateAPIConnection` 调用处参数数量不匹配（正常，下一步修复）

---

### Task 2：修改 `selectFixOption` 增加"修改模型名"选项

- [ ] **Step 1：更新菜单展示**

  找到 `selectFixOption()` 函数中的 `printMenu` 调用，将：
  ```go
  printMenu("选择要修改的内容", []MenuItem{
      {"1", "修改 URL", "Base URL 有问题"},
      {"2", "修改 Key", "API Key 有问题"},
      {"3", "都修改", "URL 和 Key 都有问题"},
  })
  ```
  改为：
  ```go
  printMenu("选择要修改的内容", []MenuItem{
      {"1", "修改 URL", "Base URL 有问题"},
      {"2", "修改 Key", "API Key 有问题"},
      {"3", "都修改", "URL 和 Key 都有问题"},
      {"4", "修改模型名", "模型名称可能不正确"},
  })
  ```

- [ ] **Step 2：在 switch 中增加 case "4"**

  在 `selectFixOption()` 的 for 循环 switch 中，在 `case "3"` 后追加：
  ```go
  case "4":
      return 4
  ```
  并将 `default` 的错误提示改为：
  ```go
  default:
      printError("无效选项，请输入 1、2、3 或 4")
  ```

---

### Task 3：修改 `main()` 验证循环处理选项4

- [ ] **Step 1：更新 `validateAPIConnection` 调用处，传入 `cfg.Model`**

  找到第 1420 行：
  ```go
  if err := validateAPIConnection(cfg.BaseURL, cfg.AuthToken); err != nil {
  ```
  改为：
  ```go
  if err := validateAPIConnection(cfg.BaseURL, cfg.AuthToken, cfg.Model); err != nil {
  ```

- [ ] **Step 2：在验证失败的 switch 中增加 case 4 处理**

  找到 `main()` 验证循环中的 switch（约第 1432 行），在 `case 3` 后追加：
  ```go
  case 4: // 修改模型名
      cfg.Model = runL2Menu("默认模型", cfg.Model)
  ```

- [ ] **Step 3：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`，无编译错误

- [ ] **Step 4：手动验证功能1**

  编译并运行：
  ```bash
  go build -o /tmp/dmxapi-test . && /tmp/dmxapi-test
  ```
  验证：
  - 选择"1 从头配置"，输入有效 URL 和 Token，API 验证应使用当前配置的默认模型（而非硬编码的 haiku）
  - 验证失败时，出现选项4"修改模型名"
  - 选择4后可以通过 L2 菜单选择新模型，并立即重新验证

- [ ] **Step 5：提交**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  git add dmxapi-claude-code.go
  git commit -m "feat: API 验证使用用户配置的默认模型，增加修改模型名选项"
  ```

---

## Chunk 2：功能2 — 启动时检测 Claude Code 是否已安装

**Files:**
- Modify: `dmxapi-claude-code.go`（新增 `checkClaudeCodeInstalled` 函数，修改 `main()`）

### Task 4：新增 `checkClaudeCodeInstalled` 函数

- [ ] **Step 1：在 `main()` 函数上方新增函数**

  在 `// ==================== 主程序 ====================` 注释块之前，插入：
  ```go
  // checkClaudeCodeInstalled 检测 claude 命令是否已安装
  func checkClaudeCodeInstalled() bool {
      _, err := exec.LookPath("claude")
      return err == nil
  }
  ```

- [ ] **Step 2：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`

---

### Task 5：在 `main()` 中插入安装检测逻辑

- [ ] **Step 1：在 `printLogo()` 调用后、`selectConfigMode()` 调用前插入检测**

  找到 `main()` 中 `printLogo()` 行，在其后紧跟插入：
  ```go
  // 检测 Claude Code 是否已安装
  if !checkClaudeCodeInstalled() {
      fmt.Println()
      printError("未检测到 Claude Code，请先安装后再运行此工具")
      fmt.Println()
      if runtime.GOOS == "windows" {
          printInfo("安装命令（PowerShell）:")
          fmt.Println("  irm https://claude.ai/install.ps1 | iex")
          fmt.Println()
          printInfo("安装命令（CMD）:")
          fmt.Println("  curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd")
      } else {
          printInfo("安装命令（macOS / Linux / WSL）:")
          fmt.Println("  curl -fsSL https://claude.ai/install.sh | bash")
      }
      fmt.Println()
      styledInput("按回车键退出")
      os.Exit(1)
  }
  ```

- [ ] **Step 2：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`

- [ ] **Step 3：手动验证功能2**

  测试已安装情况（正常运行）：
  ```bash
  go build -o /tmp/dmxapi-test . && /tmp/dmxapi-test
  ```
  预期：正常进入主菜单，无安装提示

  测试未安装情况（临时重命名模拟）：
  ```bash
  # 将 claude 重命名（如果已安装）
  which claude  # 记录路径
  # 在当前 shell 中临时覆盖 PATH，隐藏 claude
  env PATH=/usr/bin:/bin /tmp/dmxapi-test
  ```
  预期：显示错误提示和安装命令，等待回车后退出

- [ ] **Step 4：提交**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  git add dmxapi-claude-code.go
  git commit -m "feat: 启动时检测 Claude Code 是否已安装"
  ```

---

## Chunk 3：功能3 — Agent Teams 环境变量配置（主菜单选项4）

**Files:**
- Modify: `dmxapi-claude-code.go`（新增常量、新增3个函数、修改 `selectConfigMode`、修改 `main()`）

### Task 6：新增常量和 `removeEnvVarUnix` 函数

- [ ] **Step 1：在常量块中新增 `envAgentTeams`**

  在 `// 环境变量名` 常量块（约第 33 行），在 `envDisableExperimentalBetas` 后追加：
  ```go
  envAgentTeams = "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"
  ```

- [ ] **Step 2：新增 `removeEnvVarUnix` 函数**

  在 `setEnvVarsUnix` 函数之后插入：
  ```go
  // removeEnvVarUnix 从 Unix shell 配置文件中删除指定环境变量（幂等）
  func removeEnvVarUnix(key string) error {
      homeDir, err := os.UserHomeDir()
      if err != nil {
          return err
      }

      var configFiles []string
      switch runtime.GOOS {
      case "darwin":
          configFiles = []string{".zshrc", ".bash_profile"}
      default:
          configFiles = []string{".bashrc", ".profile"}
      }

      marker := fmt.Sprintf("export %s=", key)
      for _, configFile := range configFiles {
          configPath := filepath.Join(homeDir, configFile)
          if _, err := os.Stat(configPath); os.IsNotExist(err) {
              continue
          }
          content, err := os.ReadFile(configPath)
          if err != nil {
              continue
          }
          normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
          normalized = strings.ReplaceAll(normalized, "\r", "\n")
          lines := strings.Split(normalized, "\n")

          newLines := make([]string, 0, len(lines))
          prevBlank := false
          for _, line := range lines {
              if strings.HasPrefix(strings.TrimSpace(line), marker) {
                  continue // 跳过该变量行
              }
              // 压缩连续空行
              isBlank := strings.TrimSpace(line) == ""
              if isBlank && prevBlank {
                  continue
              }
              newLines = append(newLines, line)
              prevBlank = isBlank
          }

          // 确保文件末尾有换行符
          newContent := strings.Join(newLines, "\n")
          if !strings.HasSuffix(newContent, "\n") {
              newContent += "\n"
          }
          if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
              return fmt.Errorf("写入 %s 失败: %v", configPath, err)
          }
      }
      return nil
  }
  ```

- [ ] **Step 3：新增 `removeEnvVarWindows` 函数**

  在 `removeEnvVarUnix` 之后插入：
  ```go
  // removeEnvVarWindows 从 Windows 用户环境变量中删除指定变量
  func removeEnvVarWindows(key string) error {
      // 优先用 REG DELETE 删除注册表用户变量
      err := runCommand("REG", "DELETE", `HKCU\Environment`, "/V", key, "/F")
      if err != nil {
          // 降级：setx 设置为空（Windows 下可能不完全清除，打印警告）
          _ = runCommand("setx", key, "")
          printWarning(fmt.Sprintf("无法完全清除 %s，请手动在系统环境变量中删除该项", key))
      }
      return nil
  }
  ```

- [ ] **Step 4：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`

---

### Task 7：新增 `configureAgentTeams` 函数

- [ ] **Step 1：在 `saveConfig` 函数之后插入 `configureAgentTeams` 函数**

  ```go
  // configureAgentTeams 配置实验性 Agent Teams 功能环境变量
  func configureAgentTeams() {
      printSectionHeader("配置实验性 Agent Teams 功能")
      fmt.Println()

      currentVal := getEnvVar(envAgentTeams)
      if currentVal == "1" {
          printInfo(fmt.Sprintf("当前状态: %s已启用%s", colorBrightGreen, colorReset))
      } else {
          printInfo(fmt.Sprintf("当前状态: %s未设置%s", styleDim, colorReset))
      }
      fmt.Println()

      enable := runConfirmMenu("是否启用 Agent Teams 功能")

      fmt.Println()
      var err error
      if enable {
          vars := map[string]string{envAgentTeams: "1"}
          switch runtime.GOOS {
          case "windows":
              err = setEnvVarsWindows(vars)
          default:
              err = setEnvVarsUnix(vars)
          }
          if err != nil {
              printError(fmt.Sprintf("设置失败: %v", err))
          } else {
              os.Setenv(envAgentTeams, "1")
              printSuccess(fmt.Sprintf("已启用 %s=1", envAgentTeams))
          }
      } else {
          if currentVal == "" {
              printInfo("当前未设置该变量，无需操作")
          } else {
              switch runtime.GOOS {
              case "windows":
                  err = removeEnvVarWindows(envAgentTeams)
              default:
                  err = removeEnvVarUnix(envAgentTeams)
              }
              if err != nil {
                  printError(fmt.Sprintf("删除失败: %v", err))
              } else {
                  os.Unsetenv(envAgentTeams)
                  printSuccess(fmt.Sprintf("已禁用并删除 %s", envAgentTeams))
              }
          }
      }

      fmt.Println()
      switch runtime.GOOS {
      case "windows":
          printTip("请重新打开终端窗口使配置生效")
      case "darwin":
          printTip("执行 source ~/.zshrc 或重启终端使配置生效")
      default:
          printTip("执行 source ~/.bashrc 或重启终端使配置生效")
      }
      fmt.Println()
      styledInput("按回车键退出")
  }
  ```

- [ ] **Step 2：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`

---

### Task 8：修改 `selectConfigMode` 和 `main()` 接入功能3

- [ ] **Step 1：修改 `selectConfigMode()` 增加选项4**

  找到 `selectConfigMode()` 中的 `printMenu` 调用，将：
  ```go
  printMenu("配置模式选择", []MenuItem{
      {"1", "从头配置", "配置 URL、Token 和模型"},
      {"2", "仅配置模型", "跳过 URL 和 Token 配置"},
      {"3", "解决 400 报错", "禁用实验性请求头"},
  })
  ```
  改为：
  ```go
  printMenu("配置模式选择", []MenuItem{
      {"1", "从头配置", "配置 URL、Token 和模型"},
      {"2", "仅配置模型", "跳过 URL 和 Token 配置"},
      {"3", "解决 400 报错", "禁用实验性请求头"},
      {"4", "配置实验性功能", "启用/禁用 Agent Teams"},
  })
  ```

- [ ] **Step 2：在 switch 中增加 `case "4"`**

  在 `selectConfigMode()` for 循环中，`case "3"` 后追加：
  ```go
  case "4":
      return 4
  ```
  并将 `default` 错误提示改为：
  ```go
  default:
      printError("无效选项，请输入 1、2、3 或 4")
  ```

- [ ] **Step 3：在 `main()` 中处理 configMode == 4**

  找到 `main()` 中 `if configMode == 1 {` 的分支结构，在最后的 `else {` 块之前追加：
  ```go
  } else if configMode == 4 {
      configureAgentTeams()
      return
  ```

- [ ] **Step 4：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`

- [ ] **Step 5：手动验证功能3**

  ```bash
  go build -o /tmp/dmxapi-test . && /tmp/dmxapi-test
  ```
  验证：
  - 主菜单出现第4项"配置实验性功能"
  - 选4后进入独立流程，显示当前状态
  - 选"是"启用后退出，检查 `~/.zshrc` 中出现 `export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS='1'`
  - 重新运行选4，选"否"禁用后退出，检查 `~/.zshrc` 中该行已删除

- [ ] **Step 6：提交**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  git add dmxapi-claude-code.go
  git commit -m "feat: 新增主菜单选项4，支持启用/禁用 Agent Teams 实验性功能"
  ```

---

## Chunk 4：功能4 — 启动时检查版本更新

**Files:**
- Modify: `dmxapi-claude-code.go`（修改 `appVersion` 常量，新增3个函数，修改 `main()`）

### Task 9：修正 `appVersion` 并新增 `compareVersions` 函数

- [ ] **Step 1：将 `appVersion` 常量从 `"1.0.0"` 更正为 `"1.4.4"`**

  找到第 90 行：
  ```go
  appVersion = "1.0.0"
  ```
  改为：
  ```go
  appVersion = "1.4.4"
  ```

- [ ] **Step 2：新增 `compareVersions` 函数**

  在 `// ==================== 主程序 ====================` 注释块之前插入：
  ```go
  // compareVersions 比较两个版本号字符串（major.minor.patch 格式）
  // 返回 -1（a<b）、0（a==b）、1（a>b）
  // 段数不足3段时补0；任何段解析失败返回0
  func compareVersions(a, b string) int {
      parseSegments := func(v string) [3]int {
          parts := strings.SplitN(v, ".", 4) // 最多取3段
          var segs [3]int
          for i := 0; i < 3; i++ {
              if i < len(parts) {
                  n, err := strconv.Atoi(parts[i])
                  if err != nil {
                      return [3]int{} // 解析失败返回全0
                  }
                  segs[i] = n
              }
          }
          return segs
      }
      sa, sb := parseSegments(a), parseSegments(b)
      for i := 0; i < 3; i++ {
          if sa[i] < sb[i] {
              return -1
          }
          if sa[i] > sb[i] {
              return 1
          }
      }
      return 0
  }
  ```

- [ ] **Step 3：在 import 块中追加 `strconv` 和 `regexp`**

  找到文件顶部 `import (` 块（第6行），在已有包列表中追加（按字母序插入）：
  ```go
  "regexp"
  "strconv"
  ```
  当前文件缺少这两个包，`compareVersions` 用到 `strconv.Atoi`，`fetchLatestVersion` 用到 `regexp.MustCompile`，两者都需要提前加好。

- [ ] **Step 4：为 `compareVersions` 编写单元测试**

  新建文件 `dmxapi-claude-code_test.go`：
  ```go
  package main

  import "testing"

  func TestCompareVersions(t *testing.T) {
      cases := []struct {
          a, b string
          want int
      }{
          {"1.4.4", "1.4.5", -1},  // 旧版 < 新版
          {"1.4.5", "1.4.4", 1},   // 新版 > 旧版
          {"1.4.4", "1.4.4", 0},   // 相等
          {"1.9.0", "1.10.0", -1}, // 防字符串陷阱：1.9 < 1.10
          {"1.0", "1.0.0", 0},     // 段数不足补0
          {"2.0.0", "1.9.9", 1},   // major 版本比较
          {"bad", "1.0.0", 0},     // 解析失败返回0
          {"1.4.4", "1.4.3", 1},   // 修订版本：新 > 旧
          {"0.0.0", "0.0.0", 0},   // 全零相等
          {"", "1.0.0", 0},        // 空字符串解析失败返回0
          {"1.0.0", "1.0", 0},     // 被比较方段数不足补0
      }
      for _, c := range cases {
          got := compareVersions(c.a, c.b)
          if got != c.want {
              t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
          }
      }
  }
  ```

- [ ] **Step 5：运行单元测试，确认全部通过**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go test -v -run TestCompareVersions ./...
  ```
  预期：所有7个 case 通过，输出 `PASS`

---

### Task 10：新增 `fetchLatestVersion` 和 `checkForUpdates` 函数

- [ ] **Step 1：新增 `fetchLatestVersion` 函数**

  在 `compareVersions` 之后插入：
  ```go
  // fetchLatestVersion 从 CNB releases 页面获取最新版本号（不含 v 前缀）
  // 失败时返回空字符串（静默跳过）
  func fetchLatestVersion() string {
      client := &http.Client{Timeout: 5 * time.Second}
      resp, err := client.Get("https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases")
      if err != nil {
          return ""
      }
      defer resp.Body.Close()
      if resp.StatusCode != 200 {
          return ""
      }
      // 读取前 64KB，足以覆盖 initialState 中的第一个 tagRef
      buf := make([]byte, 65536)
      n, _ := resp.Body.Read(buf)
      body := string(buf[:n])

      // CNB releases 页面为 SSR，tagRef 按发布时间倒序，第一条即最新版
      re := regexp.MustCompile(`"tagRef":"refs/tags/(v\d+\.\d+\.\d+)"`)
      match := re.FindStringSubmatch(body)
      if len(match) < 2 {
          return ""
      }
      // 去掉 "v" 前缀，返回如 "1.4.5"
      return strings.TrimPrefix(match[1], "v")
  }
  ```

- [ ] **Step 2：确认 `regexp` 已在 import 中**

  Task 9 Step 3 已统一添加了 `regexp` 和 `strconv`，此处无需重复操作，直接编译验证即可。

- [ ] **Step 3：新增 `openBrowser` 辅助函数**

  在 `fetchLatestVersion` 之后插入：
  ```go
  // openBrowser 用系统命令打开浏览器，失败时打印链接
  func openBrowser(url string) {
      var err error
      switch runtime.GOOS {
      case "windows":
          err = exec.Command("cmd", "/c", "start", "", url).Start()
      case "darwin":
          err = exec.Command("open", url).Start()
      default:
          err = exec.Command("xdg-open", url).Start()
      }
      if err != nil {
          printInfo("请手动访问: " + url)
      }
  }
  ```

- [ ] **Step 4：新增 `checkForUpdates` 函数**

  在 `openBrowser` 之后插入：
  ```go
  // checkForUpdates 检查是否有新版本，有则提示用户
  func checkForUpdates() {
      latest := fetchLatestVersion()
      if latest == "" {
          return // 网络失败或解析失败，静默跳过
      }
      if compareVersions(appVersion, latest) >= 0 {
          return // 当前版本已是最新
      }
      fmt.Println()
      printInfo(fmt.Sprintf("发现新版本 v%s（当前 v%s）", latest, appVersion))
      fmt.Println()
      wantDownload := runConfirmMenu(fmt.Sprintf("发现新版本 v%s，是否立即前往下载页？", latest))
      if wantDownload {
          openBrowser("https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases")
          os.Exit(0)
      }
  }
  ```

- [ ] **Step 5：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：`BUILD OK`

---

### Task 11：在 `main()` 中接入 `checkForUpdates`，并运行全量测试

- [ ] **Step 1：在 `checkClaudeCodeInstalled` 检测块之后插入 `checkForUpdates()` 调用**

  找到功能2插入的 `if !checkClaudeCodeInstalled() { ... }` 块结束的 `}` 后，紧跟追加：
  ```go
  // 检查版本更新（失败则静默跳过）
  checkForUpdates()
  ```

- [ ] **Step 2：运行全量单元测试**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go test -v ./...
  ```
  预期：`PASS`

- [ ] **Step 3：编译并完整手动验证流程**

  ```bash
  go build -o /tmp/dmxapi-test . && /tmp/dmxapi-test
  ```
  逐项验证：
  1. Logo 版本号显示 `v1.4.4`（而非旧的 `v1.0.0`）
  2. 启动时无更新提示（当前版本已是最新）
  3. 主菜单有4个选项
  4. 选项4独立配置 Agent Teams，退出后不影响其他流程
  5. 选项1从头配置时，API 验证失败显示4个修复选项包括"修改模型名"

- [ ] **Step 4：最终提交**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  git add dmxapi-claude-code.go dmxapi-claude-code_test.go
  git commit -m "feat: 新增版本更新检查，修正 appVersion 为 1.4.4，新增 compareVersions 单元测试"
  ```

---

## 附：完整 main() 最终调用顺序（供参考）

```go
func main() {
    initWindowsConsole()
    printLogo()

    // 功能2：检测 Claude Code 是否已安装
    if !checkClaudeCodeInstalled() { ... os.Exit(1) }

    // 功能4：检查版本更新
    checkForUpdates()

    // 原有流程
    configMode := selectConfigMode()
    cfg := loadExistingConfig()

    if configMode == 1 {
        // 从头配置（含功能1改动的验证循环）
    } else if configMode == 2 {
        // 仅配置模型
    } else if configMode == 3 {
        // 解决 400 报错
    } else if configMode == 4 {
        // 功能3：配置 Agent Teams
        configureAgentTeams()
        return
    }

    // 配置模型、保存、打印摘要
    ...
}
```
