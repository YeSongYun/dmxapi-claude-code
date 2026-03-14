# VSCode 插件配置支持 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 dmxapi-claude-code 工具中新增模式5，将已有配置写入 VSCode `settings.json` 的 `claude-code.environmentVariables`，同时在模式1完成后追加 Agent Teams 和 VSCode 配置的可选询问。

**Architecture:** 采用三层分离：纯函数层（路径构建、JSON合并，可单元测试）→ I/O层（文件读写）→ 交互层（UI提示）。所有新代码追加到 `dmxapi-claude-code.go`，测试追加到 `dmxapi-claude-code_test.go`，遵循项目既有单文件结构。

**Tech Stack:** Go 标准库（`encoding/json`、`os`、`os/exec`、`path/filepath`、`runtime`）；无新依赖。

---

## Chunk 1: 纯函数层

### Task 1: 路径构建纯函数 `vscodeSettingsPathFor`

**Files:**
- Modify: `dmxapi-claude-code_test.go`
- Modify: `dmxapi-claude-code.go`

- [ ] **Step 1: 写失败测试**

在 `dmxapi-claude-code_test.go` 末尾追加：

```go
func TestVscodeSettingsPathFor(t *testing.T) {
	cases := []struct {
		goos           string
		homeDir        string
		appData        string
		wslWindowsHome string
		want           string
	}{
		{
			goos:    "darwin",
			homeDir: "/Users/alice",
			want:    "/Users/alice/Library/Application Support/Code/User/settings.json",
		},
		{
			goos:    "linux",
			homeDir: "/home/bob",
			want:    "/home/bob/.config/Code/User/settings.json",
		},
		{
			goos:    "windows",
			appData: `C:\Users\carol\AppData\Roaming`,
			want:    `C:\Users\carol\AppData\Roaming\Code\User\settings.json`,
		},
		{
			// WSL: wslWindowsHome 非空时优先使用
			goos:           "linux",
			homeDir:        "/home/dave",
			wslWindowsHome: `/mnt/c/Users/dave`,
			want:           `/mnt/c/Users/dave/AppData/Roaming/Code/User/settings.json`,
		},
	}
	for _, c := range cases {
		got := vscodeSettingsPathFor(c.goos, c.homeDir, c.appData, c.wslWindowsHome)
		if got != c.want {
			t.Errorf("vscodeSettingsPathFor(%q,%q,%q,%q)\ngot  %q\nwant %q",
				c.goos, c.homeDir, c.appData, c.wslWindowsHome, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestVscodeSettingsPathFor -v .
```

期望：编译失败，提示 `vscodeSettingsPathFor` 未定义。

- [ ] **Step 3: 实现 `vscodeSettingsPathFor`**

在 `dmxapi-claude-code.go` 的 `// ==================== 环境变量管理 ====================` 区块之后（约第699行后）追加：

```go
// ==================== VSCode 插件配置 ====================

// vscodeSettingsPathFor 根据平台参数返回 VSCode settings.json 的绝对路径（纯函数，便于测试）。
// wslWindowsHome 非空时表示 WSL 环境，使用 Windows 侧路径。
func vscodeSettingsPathFor(goos, homeDir, appData, wslWindowsHome string) string {
	switch {
	case wslWindowsHome != "":
		// WSL：写入 Windows 侧 AppData
		return filepath.Join(wslWindowsHome, "AppData", "Roaming", "Code", "User", "settings.json")
	case goos == "windows":
		return filepath.Join(appData, "Code", "User", "settings.json")
	case goos == "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "settings.json")
	default:
		// linux 及其他
		return filepath.Join(homeDir, ".config", "Code", "User", "settings.json")
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestVscodeSettingsPathFor -v .
```

期望：PASS。

- [ ] **Step 5: 提交**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "feat: 新增 vscodeSettingsPathFor 纯函数及测试"
```

---

### Task 2: 环境变量列表构建纯函数 `buildVSCodeEnvVars`

**Files:**
- Modify: `dmxapi-claude-code_test.go`
- Modify: `dmxapi-claude-code.go`

- [ ] **Step 1: 写失败测试**

在 `dmxapi-claude-code_test.go` 末尾追加：

```go
func TestBuildVSCodeEnvVars(t *testing.T) {
	cfg := Config{
		BaseURL:     "https://api.example.com",
		AuthToken:   "sk-test-token",
		Model:       "claude-sonnet-4-6-cc",
		HaikuModel:  "claude-haiku-4-5-20251001-cc",
		SonnetModel: "claude-sonnet-4-6-cc",
		OpusModel:   "claude-opus-4-6-cc",
	}

	// 不含 Agent Teams
	vars := buildVSCodeEnvVars(cfg, "")
	if len(vars) != 7 {
		t.Fatalf("expected 7 vars, got %d", len(vars))
	}
	// 验证第一项是 ANTHROPIC_BASE_URL
	found := false
	for _, v := range vars {
		if v["name"] == "ANTHROPIC_BASE_URL" && v["value"] == "https://api.example.com" {
			found = true
		}
	}
	if !found {
		t.Error("ANTHROPIC_BASE_URL not found or wrong value")
	}
	// 验证 CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS 始终为 "1"
	found = false
	for _, v := range vars {
		if v["name"] == "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS" && v["value"] == "1" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS not found or wrong value")
	}

	// 含 Agent Teams
	vars2 := buildVSCodeEnvVars(cfg, "1")
	if len(vars2) != 8 {
		t.Fatalf("expected 8 vars with agent teams, got %d", len(vars2))
	}
	found = false
	for _, v := range vars2 {
		if v["name"] == "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS" && v["value"] == "1" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS not found")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestBuildVSCodeEnvVars -v .
```

期望：编译失败，提示 `buildVSCodeEnvVars` 未定义。

- [ ] **Step 3: 实现 `buildVSCodeEnvVars`**

在 `vscodeSettingsPathFor` 函数之后追加：

```go
// buildVSCodeEnvVars 根据 Config 构建 claude-code.environmentVariables 数组（纯函数）。
// agentTeamsVal 为空时不写入 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS。
func buildVSCodeEnvVars(cfg Config, agentTeamsVal string) []map[string]string {
	entries := []map[string]string{
		{"name": envBaseURL, "value": cfg.BaseURL},
		{"name": envAuthToken, "value": cfg.AuthToken},
		{"name": envModel, "value": cfg.Model},
		{"name": envHaikuModel, "value": cfg.HaikuModel},
		{"name": envSonnetModel, "value": cfg.SonnetModel},
		{"name": envOpusModel, "value": cfg.OpusModel},
		{"name": envDisableExperimentalBetas, "value": fixedDisableExperimentalBetas},
	}
	if agentTeamsVal != "" {
		entries = append(entries, map[string]string{
			"name":  envAgentTeams,
			"value": agentTeamsVal,
		})
	}
	return entries
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestBuildVSCodeEnvVars -v .
```

期望：PASS。

- [ ] **Step 5: 提交**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "feat: 新增 buildVSCodeEnvVars 纯函数及测试"
```

---

### Task 3: JSON 合并纯函数 `mergeVSCodeSettings`

**Files:**
- Modify: `dmxapi-claude-code_test.go`
- Modify: `dmxapi-claude-code.go`

- [ ] **Step 1: 写失败测试**

在 `dmxapi-claude-code_test.go` 末尾追加：

```go
func TestMergeVSCodeSettings(t *testing.T) {
	envVars := []map[string]string{
		{"name": "ANTHROPIC_BASE_URL", "value": "https://api.example.com"},
	}

	t.Run("空文件写入", func(t *testing.T) {
		out, err := mergeVSCodeSettings([]byte(`{}`), envVars)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if _, ok := result["claude-code.environmentVariables"]; !ok {
			t.Error("claude-code.environmentVariables key missing")
		}
	})

	t.Run("保留既有键", func(t *testing.T) {
		existing := []byte(`{"editor.fontSize": 14, "claude-code.environmentVariables": []}`)
		out, err := mergeVSCodeSettings(existing, envVars)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if result["editor.fontSize"] != float64(14) {
			t.Error("editor.fontSize should be preserved")
		}
	})

	t.Run("JSON 无效返回错误", func(t *testing.T) {
		_, err := mergeVSCodeSettings([]byte(`not json`), envVars)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}
```

注意：需要在测试文件 import 块中追加 `"encoding/json"`。找到现有 import 块（文件顶部）：

```go
import (
	"strings"
	"testing"
)
```

在 `"strings"` 行之前插入一行，改为：

```go
import (
	"encoding/json"
	"strings"
	"testing"
)
```

若 import 块已包含 `"encoding/json"` 则跳过此步。

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestMergeVSCodeSettings -v .
```

期望：编译失败，提示 `mergeVSCodeSettings` 未定义。

- [ ] **Step 3: 实现 `mergeVSCodeSettings`**

在 `buildVSCodeEnvVars` 之后追加：

```go
// mergeVSCodeSettings 将 envVars 写入现有 JSON 的 claude-code.environmentVariables 键，
// 保留所有其他键。existingJSON 必须是合法 JSON 对象（{}或更复杂的对象均可）。
// 返回格式化后的 JSON 字节（2空格缩进）。
func mergeVSCodeSettings(existingJSON []byte, envVars []map[string]string) ([]byte, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(existingJSON, &settings); err != nil {
		return nil, fmt.Errorf("解析 settings.json 失败: %v", err)
	}
	settings["claude-code.environmentVariables"] = envVars
	return json.MarshalIndent(settings, "", "  ")
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -run TestMergeVSCodeSettings -v .
```

期望：PASS。

- [ ] **Step 5: 运行全部测试确保无回归**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -v .
```

期望：所有测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "feat: 新增 mergeVSCodeSettings 纯函数及测试"
```

---

## Chunk 2: I/O 层与交互层

### Task 4: 重构 `configureAgentTeams` 支持 `exitOnDone`

**背景**：`configureAgentTeams()` 末尾硬编码了 `styledInput("按回车键退出")`。在模式1后置流程中调用它时，会产生多余的等待提示，打断后续 VSCode 配置步骤。需添加 `exitOnDone bool` 参数，使其行为与即将新增的 `configureVSCode` 保持一致。

**Files:**
- Modify: `dmxapi-claude-code.go`（`configureAgentTeams` 函数，约第1672行；`main` 函数中的调用处，约第2009行）

- [ ] **Step 1: 修改 `configureAgentTeams` 签名**

找到（约第1672行）：
```go
func configureAgentTeams() {
```
替换为：
```go
// configureAgentTeams 配置实验性 Agent Teams 功能环境变量。
// exitOnDone=true 时末尾显示"按回车键退出"（独立运行模式4时使用）；
// 嵌入模式1后置步骤时传 false，由 main 统一处理退出。
func configureAgentTeams(exitOnDone bool) {
```

- [ ] **Step 2: 修改函数末尾的退出提示**

找到（约第1738行，`configureAgentTeams` 函数末尾，`isWSL()` 提示块之后）：
```go
	}
	fmt.Println()
	styledInput("按回车键退出")
}

// printSummary 打印配置摘要
```
替换为：
```go
	}
	if exitOnDone {
		fmt.Println()
		styledInput("按回车键退出")
	}
}

// printSummary 打印配置摘要
```

- [ ] **Step 3: 更新模式4的独立调用**

找到（约第2009行）：
```go
	} else if configMode == 4 {
		configureAgentTeams()
		return
```
替换为：
```go
	} else if configMode == 4 {
		configureAgentTeams(true)
		return
```

- [ ] **Step 4: 编译验证**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go build .
```

期望：编译成功，无错误。

- [ ] **Step 5: 提交**

```bash
git add dmxapi-claude-code.go
git commit -m "refactor: configureAgentTeams 新增 exitOnDone 参数"
```

---

### Task 5: I/O 层 `getWindowsHomeFromWSL`、`getVSCodeSettingsPath` 与 `saveVSCodeConfig`

**Files:**
- Modify: `dmxapi-claude-code.go`

- [ ] **Step 1: 实现 `getVSCodeSettingsPath`**

在 `mergeVSCodeSettings` 之后追加：

```go
// winPathToWSL 将 Windows 路径（如 C:\Users\alice）转换为 WSL 挂载路径（/mnt/c/Users/alice）。
// 注意：不使用 filepath.ToSlash，因为 filepath.ToSlash 在 Linux/WSL 宿主上不转换反斜杠。
func winPathToWSL(winPath string) string {
	winPath = strings.TrimSpace(winPath)
	if len(winPath) < 3 || winPath[1] != ':' {
		return ""
	}
	drive := strings.ToLower(string(winPath[0]))
	rest := strings.ReplaceAll(winPath[2:], "\\", "/")
	return "/mnt/" + drive + rest
}

// getWindowsHomeFromWSL 在 WSL 环境中获取 Windows 用户目录对应的 WSL 路径。
// 优先调用 cmd.exe；失败时回退到扫描 /mnt/c/Users/ 中的第一个非系统用户目录。
func getWindowsHomeFromWSL() string {
	// 方法1：调用 cmd.exe /c echo %USERPROFILE%
	cmd := exec.Command("cmd.exe", "/c", "echo", "%USERPROFILE%")
	if out, err := cmd.Output(); err == nil {
		if p := winPathToWSL(string(out)); p != "" {
			return p
		}
	}

	// 方法2：扫描 /mnt/c/Users/ 取第一个非系统目录
	usersDir := "/mnt/c/Users"
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return ""
	}
	systemDirs := map[string]bool{
		"Default": true, "Public": true, "All Users": true,
		"Default User": true, "desktop.ini": true,
	}
	for _, e := range entries {
		if e.IsDir() && !systemDirs[e.Name()] {
			return filepath.Join(usersDir, e.Name())
		}
	}
	return ""
}

// getVSCodeSettingsPath 返回当前系统 VSCode settings.json 的绝对路径。
func getVSCodeSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	appData := os.Getenv("APPDATA")
	if appData == "" && runtime.GOOS == "windows" {
		// 回退：用 homeDir 拼出 AppData\Roaming
		appData = filepath.Join(homeDir, "AppData", "Roaming")
	}

	wslWindowsHome := ""
	if isWSL() {
		wslWindowsHome = getWindowsHomeFromWSL()
	}

	path := vscodeSettingsPathFor(runtime.GOOS, homeDir, appData, wslWindowsHome)
	if path == "" {
		return "", fmt.Errorf("无法确定 VSCode settings.json 路径")
	}
	return path, nil
}
```

- [ ] **Step 2: 实现 `saveVSCodeConfig`**

在 `getVSCodeSettingsPath` 之后追加：

```go
// saveVSCodeConfig 将 cfg 写入 VSCode settings.json 的 claude-code.environmentVariables。
// 若文件不存在则自动创建；若 JSON 解析失败则询问用户是否备份重建。
func saveVSCodeConfig(cfg Config) error {
	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 读取现有内容（不存在则用空对象）
	existingJSON := []byte("{}")
	if data, err := os.ReadFile(settingsPath); err == nil {
		existingJSON = data
	}

	agentTeamsVal := getEnvVar(envAgentTeams)
	envVars := buildVSCodeEnvVars(cfg, agentTeamsVal)

	merged, err := mergeVSCodeSettings(existingJSON, envVars)
	if err != nil {
		// JSON 解析失败：询问是否备份重建
		printError(fmt.Sprintf("settings.json 解析失败: %v", err))
		if !styledConfirm("是否备份原文件并重新创建") {
			return fmt.Errorf("用户取消：保留原文件，跳过写入")
		}
		backupPath := settingsPath + ".bak"
		if berr := os.Rename(settingsPath, backupPath); berr != nil {
			return fmt.Errorf("备份失败: %v", berr)
		}
		printInfo(fmt.Sprintf("原文件已备份至: %s", backupPath))
		merged, err = mergeVSCodeSettings([]byte("{}"), envVars)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(settingsPath, merged, 0644)
}
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go build .
```

期望：编译成功，无错误。

- [ ] **Step 4: 提交**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: 新增 getVSCodeSettingsPath、saveVSCodeConfig I/O 函数"
```

---

### Task 6: 交互层 `configureVSCode`

**Files:**
- Modify: `dmxapi-claude-code.go`

- [ ] **Step 1: 实现 `configureVSCode`**

在 `saveVSCodeConfig` 之后追加：

```go
// configureVSCode 模式5交互流程：展示将写入的配置，用户确认后写入 VSCode settings.json。
// exitOnDone=true 时末尾显示"按回车键退出"（独立运行模式5时使用）；
// 嵌入模式1后置步骤时传 false，由 main 统一处理退出。
func configureVSCode(cfg Config, exitOnDone bool) {
	printSectionHeader("配置 VSCode 插件")
	fmt.Println()

	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		printError(fmt.Sprintf("无法确定 settings.json 路径: %v", err))
		if exitOnDone {
			fmt.Println()
			styledInput("按回车键退出")
		}
		return
	}
	printInfo(fmt.Sprintf("目标文件: %s", settingsPath))
	fmt.Println()

	// 展示将写入的配置
	agentTeamsVal := getEnvVar(envAgentTeams)
	envVars := buildVSCodeEnvVars(cfg, agentTeamsVal)

	if cfg.BaseURL == "" || cfg.AuthToken == "" {
		printWarning("未检测到 BaseURL 或 Token 配置，建议先运行「从头配置」")
		fmt.Println()
	}

	printInfo("将写入以下环境变量:")
	for _, v := range envVars {
		val := v["value"]
		if v["name"] == envAuthToken && len(val) > 8 {
			val = maskToken(val)
		}
		fmt.Printf("  %-45s = %s\n", v["name"], val)
	}
	fmt.Println()

	if !styledConfirm("确认写入 VSCode settings.json") {
		printInfo("已取消")
		if exitOnDone {
			fmt.Println()
			styledInput("按回车键退出")
		}
		return
	}

	fmt.Println()
	err = runWithSpinner("正在写入 VSCode 配置...", func() error {
		return saveVSCodeConfig(cfg)
	})
	if err != nil {
		printError(fmt.Sprintf("写入失败: %v", err))
	} else {
		printSuccess("VSCode 配置写入成功!")
		printInfo(fmt.Sprintf("文件路径: %s", settingsPath))
		if isWSL() {
			printTip("注意：已写入 Windows 侧 VSCode 配置，重启 VSCode 后生效")
		} else {
			printTip("重启 VSCode 后配置生效")
		}
	}

	if exitOnDone {
		fmt.Println()
		styledInput("按回车键退出")
	}
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go build .
```

期望：编译成功。

- [ ] **Step 3: 提交**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: 新增 configureVSCode 交互函数"
```

---

### Task 7: 更新主菜单与 `main()` 函数

**Files:**
- Modify: `dmxapi-claude-code.go`（`selectConfigMode` 函数，约第959行；`main` 函数，约第1919行）

- [ ] **Step 1: 更新 `selectConfigMode` 新增模式5**

找到（约第959行）：

```go
func selectConfigMode() int {
	return runItemMenu("配置模式选择", []MenuItem{
		{"1", "从头配置", "配置 URL、Token 和模型"},
		{"2", "仅配置模型", "跳过 URL 和 Token 配置"},
		{"3", "解决 400 报错", "禁用实验性请求头"},
		{"4", "配置实验性功能", "启用/禁用 Agent Teams"},
	})
}
```

替换为：

```go
func selectConfigMode() int {
	return runItemMenu("配置模式选择", []MenuItem{
		{"1", "从头配置", "配置 URL、Token 和模型"},
		{"2", "仅配置模型", "跳过 URL 和 Token 配置"},
		{"3", "解决 400 报错", "禁用实验性请求头"},
		{"4", "配置实验性功能", "启用/禁用 Agent Teams"},
		{"5", "配置 VSCode 插件", "写入 VSCode settings.json"},
	})
}
```

- [ ] **Step 2: 在 `main()` 中处理模式5**

找到（约第2009行，此时 `configureAgentTeams` 已在 Task 4 改为接受 `exitOnDone` 参数）：

```go
	} else if configMode == 4 {
		configureAgentTeams(true)
		return
	} else {
```

替换为：

```go
	} else if configMode == 4 {
		configureAgentTeams(true)
		return
	} else if configMode == 5 {
		cfg := loadExistingConfig()
		configureVSCode(cfg, true)
		return
	} else {
```

- [ ] **Step 3: 在模式1保存后追加可选步骤**

找到（约第2039行，保存成功后、`printSummary` 之前）：

```go
	printSuccess("保存成功!")

	// 打印摘要
	printSummary(cfg)

	// 等待用户退出
	fmt.Println()
	styledInput("按回车键退出")
```

替换为：

```go
	printSuccess("保存成功!")

	// 可选：配置 Agent Teams（exitOnDone=false，由 main 统一处理退出）
	fmt.Println()
	if styledConfirm("是否同时配置 Agent Teams 功能") {
		configureAgentTeams(false)
	}

	// 可选：配置 VSCode 插件
	fmt.Println()
	if styledConfirm("是否同时配置 VSCode 插件") {
		configureVSCode(cfg, false)
	}

	// 打印摘要
	printSummary(cfg)

	// 等待用户退出
	fmt.Println()
	styledInput("按回车键退出")
```

- [ ] **Step 4: 编译验证**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go build .
```

期望：编译成功，无错误。

- [ ] **Step 5: 运行全部测试**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -v .
```

期望：所有测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: 主菜单新增模式5，模式1追加 Agent Teams 和 VSCode 可选配置"
```

---

## Chunk 3: 收尾

### Task 8: 版本号更新与最终验证

**Files:**
- Modify: `dmxapi-claude-code.go`（`appVersion` 常量，约第93行）

- [ ] **Step 1: 更新版本号**

找到：
```go
appVersion = "1.4.5"
```

替换为：
```go
appVersion = "1.5.0"
```

- [ ] **Step 2: 运行全部测试**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test -v .
```

期望：所有测试 PASS。

- [ ] **Step 3: 编译所有平台二进制（可选，验证交叉编译无误）**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code
GOOS=darwin  GOARCH=amd64 go build -o /dev/null .
GOOS=darwin  GOARCH=arm64 go build -o /dev/null .
GOOS=linux   GOARCH=amd64 go build -o /dev/null .
GOOS=windows GOARCH=amd64 go build -o /dev/null .
```

期望：四个平台均编译成功。

- [ ] **Step 4: 提交版本号**

```bash
git add dmxapi-claude-code.go
git commit -m "chore: 版本号升级 v1.4.5 → v1.5.0"
```
