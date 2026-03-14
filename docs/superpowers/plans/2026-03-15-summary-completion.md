# 配置摘要完善 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `printSummary` 的配置摘要表格中新增 Agent Teams 和 VSCode Plugin 两行状态显示。

**Architecture:** 提取 VSCode 检测逻辑为独立纯函数 `isVSCodeConfigured([]byte) bool`，便于单元测试；Agent Teams 直接读取环境变量（已有 `getEnvVar` 封装）；两者均在 `printSummary` 内调用，函数签名不变。

**Tech Stack:** Go 标准库（`encoding/json`、`os`）；测试框架为 `testing`（表驱动 + subtests）。

---

## Chunk 1: 实现 isVSCodeConfigured + 更新 printSummary

### Task 1: 编写 `isVSCodeConfigured` 的失败测试

**Files:**
- Modify: `dmxapi-claude-code_test.go`（追加到文件末尾）

- [ ] **Step 1: 在 `dmxapi-claude-code_test.go` 末尾追加以下测试函数**

```go
func TestIsVSCodeConfigured(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  bool
	}{
		{"含目标键", []byte(`{"claude-code.environmentVariables": []}`), true},
		{"含其他键", []byte(`{"editor.fontSize": 14}`), false},
		{"空对象", []byte(`{}`), false},
		{"无效 JSON", []byte(`not json`), false},
		{"空字节", []byte(``), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isVSCodeConfigured(c.input)
			if got != c.want {
				t.Errorf("isVSCodeConfigured(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试，确认编译失败（函数未定义）**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./... -run TestIsVSCodeConfigured -v
```

期望输出：编译错误 `undefined: isVSCodeConfigured`

---

### Task 2: 实现 `isVSCodeConfigured` 纯函数

**Files:**
- Modify: `dmxapi-claude-code.go`（在 `maskToken` 函数之前，约第 2038 行前新增）

- [ ] **Step 1: 在 `dmxapi-claude-code.go` 的 `// maskToken 遮盖 Token` 注释前新增以下函数**

```go
// isVSCodeConfigured 检测 JSON 内容是否含 claude-code.environmentVariables 键。
// 用于判断 VSCode settings.json 是否已由本工具写入配置。
func isVSCodeConfigured(data []byte) bool {
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}
	_, ok := settings["claude-code.environmentVariables"]
	return ok
}
```

- [ ] **Step 2: 运行测试，确认通过**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./... -run TestIsVSCodeConfigured -v
```

期望输出：
```
--- PASS: TestIsVSCodeConfigured (0.00s)
    --- PASS: TestIsVSCodeConfigured/含目标键 (0.00s)
    --- PASS: TestIsVSCodeConfigured/含其他键 (0.00s)
    --- PASS: TestIsVSCodeConfigured/空对象 (0.00s)
    --- PASS: TestIsVSCodeConfigured/无效_JSON (0.00s)
    --- PASS: TestIsVSCodeConfigured/空字节 (0.00s)
PASS
```

- [ ] **Step 3: 运行全量测试，确认无回归**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./... -v
```

期望：所有测试 PASS

---

### Task 3: 修改 `printSummary` 新增两行状态

**Files:**
- Modify: `dmxapi-claude-code.go`：`printSummary` 函数体（约第 1984-2036 行）

- [ ] **Step 1: 在 `printSummary` 函数中，`makeRow` 辅助函数定义之后、`lines := []string{...}` 之前，新增以下状态检测代码**

插入位置：紧接在 `makeRow := func(...) {...}` 闭包定义结束的 `}` 之后、`lines := []string{` 之前。

```go
	// Agent Teams：读取当前进程环境变量（配置后 os.Setenv 已更新）
	agentTeamsDisplay, agentTeamsColor := "未启用", colorWhite
	if getEnvVar(envAgentTeams) == "1" {
		agentTeamsDisplay, agentTeamsColor = "已启用", colorBrightGreen
	}

	// VSCode Plugin：解析 settings.json，检测目标键是否存在
	vscodeDisplay, vscodeColor := "未配置", colorWhite
	if path, err := getVSCodeSettingsPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if isVSCodeConfigured(data) {
				vscodeDisplay, vscodeColor = "已配置", colorBrightGreen
			}
		}
	}
```

- [ ] **Step 2: 在 `lines := []string{...}` 末尾（`makeRow("Disable Betas", ...)` 之后）追加两行**

```go
		makeRow("Agent Teams", agentTeamsDisplay, agentTeamsColor),
		makeRow("VSCode Plugin", vscodeDisplay, vscodeColor),
```

修改后完整 `lines` 应为：

```go
	lines := []string{
		makeRow("Base URL", cfg.BaseURL, colorBrightGreen),
		makeRow("Auth Token", maskToken(cfg.AuthToken), colorBrightYellow),
		makeRow("Model", cfg.Model, colorCyan),
		makeRow("Haiku Model", cfg.HaikuModel, colorCyan),
		makeRow("Sonnet Model", cfg.SonnetModel, colorCyan),
		makeRow("Opus Model", cfg.OpusModel, colorCyan),
		makeRow("Disable Betas", fixedDisableExperimentalBetas, colorMagenta),
		makeRow("Agent Teams", agentTeamsDisplay, agentTeamsColor),
		makeRow("VSCode Plugin", vscodeDisplay, vscodeColor),
	}
```

- [ ] **Step 3: 编译确认无语法错误**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go build ./...
```

期望：无输出（编译成功）

- [ ] **Step 4: 运行全量测试**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./... -v
```

期望：所有测试 PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && \
git add dmxapi-claude-code.go dmxapi-claude-code_test.go && \
git commit -m "feat: 配置摘要新增 Agent Teams 和 VSCode Plugin 状态行"
```

---

### Task 4: 同步提交设计文档

**Files:**
- Modify: `docs/superpowers/specs/2026-03-15-summary-completion-design.md`（已存在）

- [ ] **Step 1: 提交规格文档**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && \
git add docs/superpowers/specs/2026-03-15-summary-completion-design.md \
        docs/superpowers/plans/2026-03-15-summary-completion.md && \
git commit -m "docs: 新增配置摘要完善设计文档和实现计划"
```
