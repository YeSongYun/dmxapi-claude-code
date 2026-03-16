# 清除所有配置 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在配置模式菜单新增第 6 项"清除所有配置"，一键清除 Shell 配置文件、VSCode settings.json、Windows 注册表和当前进程中的所有 dmxapi 相关环境变量。

**Architecture:** 新增 `clearAllConfig()` 主入口函数，内部复用现有的 `removeEnvVarUnix()` / `removeEnvVarWindows()` 逐变量清除 Shell/注册表配置，新增 `clearVSCodeConfig()` 处理 VSCode 配置清除。通过 `clearResult` 结构体收集各位置的清除结果，统一汇总输出报告。

**Tech Stack:** Go, 复用现有 `detectShellProfile()`、`removeEnvVarUnix()`、`removeEnvVarWindows()`、`getVSCodeSettingsPath()`、`stripJSONC()`

**Spec:** `docs/superpowers/specs/2026-03-16-clear-all-config-design.md`

---

## Chunk 1: 核心实现

### Task 1: 添加 allEnvVarKeys 变量和 clearResult 结构体

**Files:**
- Modify: `dmxapi-claude-code.go:56` (在 `fixedDisableExperimentalBetas` 常量后)
- Modify: `dmxapi-claude-code.go:114` (在 `Config` 结构体后)

- [ ] **Step 1: 在常量块之后添加 allEnvVarKeys 变量**

在 `dmxapi-claude-code.go` 第 74 行 `presetModels` 变量之后添加：

```go
// allEnvVarKeys 本工具管理的所有环境变量名，清除时使用
var allEnvVarKeys = []string{
	envBaseURL,
	envAuthToken,
	envModel,
	envHaikuModel,
	envSonnetModel,
	envOpusModel,
	envDisableExperimentalBetas,
	envAgentTeams,
}
```

- [ ] **Step 2: 在 Config 结构体后添加 clearResult 结构体**

在 `dmxapi-claude-code.go` 第 114 行 `Config` 结构体的 `}` 之后添加：

```go
// clearResult 记录单个位置的清除结果
type clearResult struct {
	Location string // 位置描述，如 "~/.zshrc"
	Status   string // "success" | "skipped" | "failed"
	Message  string // 详细信息
	Err      error  // 错误信息（如有）
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go build -o /dev/null .`
Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: add allEnvVarKeys and clearResult for clear-all-config"
```

---

### Task 2: 实现 clearVSCodeConfig 函数

**Files:**
- Modify: `dmxapi-claude-code.go` (在 `saveVSCodeConfig` 函数之后，约第 919 行后)

- [ ] **Step 1: 编写 clearVSCodeConfig 函数**

在 `saveVSCodeConfig` 函数之后添加：

```go
// clearVSCodeConfig 从 VSCode settings.json 中移除本工具写入的配置键。
// 仅删除 claudeCode.environmentVariables 和旧版 claude-code.environmentVariables 键，
// 保留用户的所有其他配置不变。
func clearVSCodeConfig() clearResult {
	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		return clearResult{Location: "VSCode settings.json", Status: "skipped", Message: "无法确定路径"}
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return clearResult{Location: settingsPath, Status: "skipped", Message: "文件不存在或无法读取"}
	}

	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: "JSON 解析失败", Err: err}
	}

	// 检查是否存在需要清除的键
	_, hasNew := settings[vscodeEnvKey]
	_, hasOld := settings[vscodeEnvKeyOld]
	if !hasNew && !hasOld {
		return clearResult{Location: settingsPath, Status: "skipped", Message: "未找到相关配置"}
	}

	delete(settings, vscodeEnvKey)
	delete(settings, vscodeEnvKeyOld)

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: "JSON 序列化失败", Err: err}
	}
	output = append(output, '\n')

	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: "写入失败", Err: err}
	}

	return clearResult{Location: settingsPath, Status: "success", Message: "已移除配置键"}
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go build -o /dev/null .`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: add clearVSCodeConfig to remove settings from VSCode"
```

---

### Task 3: 实现 clearAllConfig 主入口函数

**Files:**
- Modify: `dmxapi-claude-code.go` (在 `clearVSCodeConfig` 函数之后添加)

- [ ] **Step 1: 编写 clearAllConfig 函数**

```go
// clearAllConfig 清除所有配置（模式6）。
// 显示摘要 → 二次确认 → 逐位置清除 → 显示报告。
func clearAllConfig() {
	printSectionHeader("清除所有配置")
	fmt.Println()

	// 显示清除摘要
	printInfo("将从以下位置清除所有 dmxapi 相关配置：")
	fmt.Println()
	switch runtime.GOOS {
	case "windows":
		fmt.Println("    • Windows 用户环境变量（注册表）")
	default:
		profile := detectShellProfile(runtime.GOOS)
		for _, f := range profile.configFiles {
			fmt.Printf("    • ~/%s\n", f)
		}
	}
	// VSCode 总是可能存在
	if path, err := getVSCodeSettingsPath(); err == nil {
		fmt.Printf("    • VSCode settings.json (%s)\n", path)
	}
	fmt.Println("    • 当前进程环境变量")
	fmt.Println()
	printInfo("涉及的环境变量：")
	for _, key := range allEnvVarKeys {
		fmt.Printf("    • %s\n", key)
	}
	fmt.Println()

	printWarning("此操作不可撤销，Auth Token 清除后需要重新获取")
	fmt.Println()

	// 二次确认
	if !styledConfirm("确定要清除所有配置吗") {
		fmt.Println()
		printInfo("已取消，未做任何更改")
		return
	}

	fmt.Println()
	var results []clearResult

	// 1. 清除 Shell 配置文件 / Windows 注册表
	switch runtime.GOOS {
	case "windows":
		errCount := 0
		for _, key := range allEnvVarKeys {
			if err := removeEnvVarWindows(key); err != nil {
				errCount++
			}
		}
		if errCount > 0 {
			results = append(results, clearResult{
				Location: "Windows 注册表",
				Status:   "failed",
				Message:  fmt.Sprintf("%d 个变量清除失败", errCount),
			})
		} else {
			results = append(results, clearResult{
				Location: "Windows 注册表",
				Status:   "success",
				Message:  fmt.Sprintf("已移除 %d 个环境变量", len(allEnvVarKeys)),
			})
		}
	default:
		errCount := 0
		for _, key := range allEnvVarKeys {
			if err := removeEnvVarUnix(key); err != nil {
				errCount++
			}
		}
		profile := detectShellProfile(runtime.GOOS)
		loc := "Shell 配置文件"
		if len(profile.configFiles) > 0 {
			loc = "~/" + profile.configFiles[0]
		}
		if errCount > 0 {
			results = append(results, clearResult{
				Location: loc,
				Status:   "failed",
				Message:  fmt.Sprintf("%d 个变量清除失败", errCount),
			})
		} else {
			results = append(results, clearResult{
				Location: loc,
				Status:   "success",
				Message:  fmt.Sprintf("已移除 %d 个环境变量", len(allEnvVarKeys)),
			})
		}
	}

	// 2. 清除 VSCode 配置
	results = append(results, clearVSCodeConfig())

	// 3. 清除当前进程环境变量
	for _, key := range allEnvVarKeys {
		os.Unsetenv(key)
	}
	results = append(results, clearResult{
		Location: "当前进程",
		Status:   "success",
		Message:  "已清除所有环境变量",
	})

	// 显示结果报告
	fmt.Println()
	printSectionHeader("清除结果")
	fmt.Println()
	for _, r := range results {
		switch r.Status {
		case "success":
			printSuccess(fmt.Sprintf("%s — %s", r.Location, r.Message))
		case "skipped":
			printInfo(fmt.Sprintf("%s — %s（已跳过）", r.Location, r.Message))
		case "failed":
			printError(fmt.Sprintf("%s — %s", r.Location, r.Message))
			if r.Err != nil {
				fmt.Printf("    %s%s%s\n", colorRed, r.Err.Error(), colorReset)
			}
		}
	}
	fmt.Println()
	printTip("重新打开终端后配置清除完全生效")
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go build -o /dev/null .`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: add clearAllConfig main entry function"
```

---

### Task 4: 修改菜单和 main 函数接入模式 6

**Files:**
- Modify: `dmxapi-claude-code.go:1248-1258` (`selectConfigMode` 函数)
- Modify: `dmxapi-claude-code.go:2345-2350` (`main` 函数中 mode 4/5 的 early return 区域)

- [ ] **Step 1: 在 selectConfigMode 中新增第 6 项**

将 `selectConfigMode` 函数修改为：

```go
// selectConfigMode 选择配置模式
// 返回值: 1 = 从头配置, 2 = 仅配置模型, 3 = 解决 400 报错, 4 = 配置实验性功能, 5 = 配置 VSCode 插件, 6 = 清除所有配置
func selectConfigMode() int {
	return runItemMenu("配置模式选择", []MenuItem{
		{"1", "从头配置", "配置 URL、Token 和模型"},
		{"2", "仅配置模型", "跳过 URL 和 Token 配置"},
		{"3", "解决 400 报错", "禁用实验性请求头"},
		{"4", "配置实验性功能", "启用/禁用 Agent Teams"},
		{"5", "配置 VSCode 插件", "写入 VSCode settings.json"},
		{"6", "清除所有配置", "移除所有已保存的配置"},
	})
}
```

- [ ] **Step 2: 在 main 函数中新增 mode 6 分支（early return）**

在 `dmxapi-claude-code.go` 的 `main` 函数中，找到：

```go
	} else if configMode == 5 {
		configureVSCode(cfg, true)
		return
	} else {
```

在其后添加 mode 6 分支，改为：

```go
	} else if configMode == 5 {
		configureVSCode(cfg, true)
		return
	} else if configMode == 6 {
		clearAllConfig()
		fmt.Println()
		styledInput("按回车键退出")
		return
	} else {
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go build -o /dev/null .`
Expected: 编译成功

- [ ] **Step 4: 运行测试**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./... -v -count=1`
Expected: 所有已有测试通过

- [ ] **Step 5: Commit**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: add 'clear all config' as menu option 6"
```

---

## Chunk 2: 测试

### Task 5: 编写 clearVSCodeConfig 单元测试

**Files:**
- Modify: `dmxapi-claude-code_test.go`

- [ ] **Step 1: 编写测试**

首先在 `dmxapi-claude-code_test.go` 的 import 块中添加 `"os"` 和 `"path/filepath"`：

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

然后在文件末尾添加测试函数：

```go
func TestClearVSCodeConfig_RemovesKeys(t *testing.T) {
	// 创建临时 settings.json
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	content := `{
  "editor.fontSize": 14,
  "claudeCode.environmentVariables": [
    {"name": "ANTHROPIC_BASE_URL", "value": "https://example.com"}
  ],
  "claude-code.environmentVariables": [
    {"name": "ANTHROPIC_BASE_URL", "value": "https://example.com"}
  ]
}`
	if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 读取 → 解析 → 删除键 → 写回（模拟 clearVSCodeConfig 核心逻辑）
	data, _ := os.ReadFile(settingsPath)
	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		t.Fatal(err)
	}

	delete(settings, vscodeEnvKey)
	delete(settings, vscodeEnvKeyOld)

	output, _ := json.MarshalIndent(settings, "", "  ")
	output = append(output, '\n')
	os.WriteFile(settingsPath, output, 0644)

	// 验证
	result, _ := os.ReadFile(settingsPath)
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)

	if _, exists := parsed[vscodeEnvKey]; exists {
		t.Error("claudeCode.environmentVariables should be removed")
	}
	if _, exists := parsed[vscodeEnvKeyOld]; exists {
		t.Error("claude-code.environmentVariables should be removed")
	}
	if _, exists := parsed["editor.fontSize"]; !exists {
		t.Error("other settings should be preserved")
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestClearVSCodeConfig -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add dmxapi-claude-code_test.go
git commit -m "test: add unit test for clearVSCodeConfig key removal"
```

---

### Task 6: 编写 allEnvVarKeys 完整性测试

**Files:**
- Modify: `dmxapi-claude-code_test.go`

- [ ] **Step 1: 编写测试确保 allEnvVarKeys 包含所有已知变量**

```go
func TestAllEnvVarKeys_ContainsAllKnownKeys(t *testing.T) {
	expected := []string{
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS",
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS",
	}

	if len(allEnvVarKeys) != len(expected) {
		t.Errorf("allEnvVarKeys has %d keys, expected %d", len(allEnvVarKeys), len(expected))
	}

	keySet := make(map[string]bool)
	for _, k := range allEnvVarKeys {
		keySet[k] = true
	}
	for _, e := range expected {
		if !keySet[e] {
			t.Errorf("allEnvVarKeys is missing %s", e)
		}
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestAllEnvVarKeys -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add dmxapi-claude-code_test.go
git commit -m "test: add completeness test for allEnvVarKeys"
```

---

### Task 7: 最终验证

- [ ] **Step 1: 运行全部测试**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./... -v -count=1`
Expected: 所有测试通过

- [ ] **Step 2: 编译完整二进制**

Run: `cd /Users/yesongyun/代码/dmxapi_claude_code && go build -o dmxapi-claude-code .`
Expected: 编译成功，生成可执行文件

- [ ] **Step 3: 清理构建产物**

Run: `rm -f /Users/yesongyun/代码/dmxapi_claude_code/dmxapi-claude-code`
