# VSCode 配置键名修复 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 VSCode 写入/检测键名从错误的 `claude-code.environmentVariables` 修正为 VSCode 扩展实际读取的 `claudeCode.environmentVariables`，同时向后兼容旧键检测。

**Architecture:** 新增 `vscodeEnvKeyOld` 常量保存旧键名，修改 `mergeVSCodeSettings` 写入新键，修改 `isVSCodeConfigured` 同时检测新旧两个键（OR 逻辑）。

**Tech Stack:** Go 标准库（`encoding/json`、`os`）

---

## Chunk 1: 测试更新（TDD 先行）

### Task 1: 更新 TestMergeVSCodeSettings

**Files:**
- Modify: `dmxapi-claude-code_test.go:247`

- [ ] **Step 1: 将"空文件写入"子测试的键名断言改为新键**

将 `dmxapi-claude-code_test.go` 第 247 行：
```go
if _, ok := result["claude-code.environmentVariables"]; !ok {
    t.Error("claude-code.environmentVariables key missing")
}
```
改为：
```go
if _, ok := result["claudeCode.environmentVariables"]; !ok {
    t.Error("claudeCode.environmentVariables key missing")
}
```

- [ ] **Step 2: 将"保留既有键"子测试的输入数据也改为新键**

将第 253 行：
```go
existing := []byte(`{"editor.fontSize": 14, "claude-code.environmentVariables": []}`)
```
改为：
```go
existing := []byte(`{"editor.fontSize": 14, "claudeCode.environmentVariables": []}`)
```

- [ ] **Step 3: 运行测试，确认失败（实现尚未修改）**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code
go test -run TestMergeVSCodeSettings -v
```
期望：FAIL —— `claudeCode.environmentVariables key missing`

---

### Task 2: 更新 TestIsVSCodeConfigured

**Files:**
- Modify: `dmxapi-claude-code_test.go:275-294`

- [ ] **Step 1: 将现有 cases 中旧键重命名为新键，并新增兼容用例**

将 `TestIsVSCodeConfigured` 的 `cases` 切片替换为：
```go
cases := []struct {
    name  string
    input []byte
    want  bool
}{
    {"含新键", []byte(`{"claudeCode.environmentVariables": []}`), true},
    {"含旧键（向后兼容）", []byte(`{"claude-code.environmentVariables": []}`), true},
    {"新旧键共存", []byte(`{"claudeCode.environmentVariables": [], "claude-code.environmentVariables": []}`), true},
    {"旧键值为空数组", []byte(`{"claude-code.environmentVariables": []}`), true},
    {"含其他键", []byte(`{"editor.fontSize": 14}`), false},
    {"空对象", []byte(`{}`), false},
    {"无效 JSON", []byte(`not json`), false},
    {"空字节", []byte(``), false},
}
```

- [ ] **Step 2: 运行测试，确认"含新键"用例失败（实现尚未修改）**

```bash
go test -run TestIsVSCodeConfigured -v
```
期望：`含新键` 子测试 FAIL，其余通过

---

## Chunk 2: 实现修改

### Task 3: 修改常量定义

**Files:**
- Modify: `dmxapi-claude-code.go:44-45`

- [ ] **Step 1: 将 vscodeEnvKey 改为新键，并新增 vscodeEnvKeyOld**

将第 44-45 行：
```go
// VSCode settings.json 中写入配置所用的键名
vscodeEnvKey = "claude-code.environmentVariables"
```
改为：
```go
// VSCode settings.json 中写入配置所用的键名（claudeCode 为扩展 package.json 中定义的配置前缀）
vscodeEnvKey    = "claudeCode.environmentVariables"
// vscodeEnvKeyOld 为旧版工具使用的错误键名，仅用于向后兼容检测，不再写入
vscodeEnvKeyOld = "claude-code.environmentVariables"
```

---

### Task 4: 修改 isVSCodeConfigured 函数

**Files:**
- Modify: `dmxapi-claude-code.go:2059-2068`

- [ ] **Step 1: 更新函数注释和检测逻辑**

将函数替换为：
```go
// isVSCodeConfigured 检测 JSON 内容是否含 claudeCode.environmentVariables 键（新键）
// 或旧版工具写入的 claude-code.environmentVariables 键（向后兼容）。
// 用于判断 VSCode settings.json 是否已由本工具写入配置。
func isVSCodeConfigured(data []byte) bool {
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}
	_, hasNew := settings[vscodeEnvKey]
	_, hasOld := settings[vscodeEnvKeyOld]
	return hasNew || hasOld
}
```

- [ ] **Step 2: 更新 buildVSCodeEnvVars 和 mergeVSCodeSettings 函数注释中的旧键名引用**

将 `dmxapi-claude-code.go` 第 722 行注释：
```go
// buildVSCodeEnvVars 根据 Config 构建 claude-code.environmentVariables 数组（纯函数）。
```
改为：
```go
// buildVSCodeEnvVars 根据 Config 构建 claudeCode.environmentVariables 数组（纯函数）。
```

将第 743 行注释：
```go
// mergeVSCodeSettings 将 envVars 写入现有 JSON 的 claude-code.environmentVariables 键，
```
改为：
```go
// mergeVSCodeSettings 将 envVars 写入现有 JSON 的 claudeCode.environmentVariables 键，
```

将第 820 行注释：
```go
// saveVSCodeConfig 将 cfg 写入 VSCode settings.json 的 claude-code.environmentVariables。
```
改为：
```go
// saveVSCodeConfig 将 cfg 写入 VSCode settings.json 的 claudeCode.environmentVariables。
```

- [ ] **Step 3: 运行所有 VSCode 相关测试，确认全部通过**

```bash
go test -run "TestMergeVSCodeSettings|TestIsVSCodeConfigured|TestBuildVSCodeEnvVars|TestVSCodeSettingsPath" -v
```
期望：全部 PASS

- [ ] **Step 4: 运行完整测试套件**

```bash
go test ./... -v
```
期望：全部 PASS，无新增失败

- [ ] **Step 5: 提交**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "fix: 修正 VSCode 配置键名为 claudeCode.environmentVariables，兼容旧键检测"
```
