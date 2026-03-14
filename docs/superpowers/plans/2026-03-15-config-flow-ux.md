# 配置流程 UX 优化 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Agent Teams / VSCode 问询时机从"保存后追问"改为"API 验证后、模型配置前"，同时使模型菜单的 ESC/q 真正做到"保存退出"。

**Architecture:** 只修改 `main()` 函数中 configMode == 1 的流程：新增两个 `bool` 变量在 `if-else` 链之前声明，在 API 验证成功后赋值（mode 1），删除旧的"保存后追问"块（该块对所有 configMode 有效，删除后 mode 2 也同步去除追问），在保存后用变量决定是否执行附加配置。

**Tech Stack:** Go 1.21+，标准库，无新依赖

---

## Chunk 1：流程重排

### Task 1：声明提前意向变量

**Files:**
- Modify: `dmxapi-claude-code.go`（约第 2185–2189 行）

- [ ] **Step 1：在 `cfg := loadExistingConfig()` 之后插入变量声明**

找到以下代码（约第 2185–2189 行）：
```go
	// 加载现有配置
	cfg := loadExistingConfig()

	// 根据配置模式执行不同流程
	if configMode == 1 {
```

在 `cfg := loadExistingConfig()` 和 `// 根据配置模式执行不同流程` 注释之间插入一行：
```go
	// 加载现有配置
	cfg := loadExistingConfig()

	// 提前收集附加配置意向（模式1时由用户选择，其他模式默认 false）
	var wantTeams, wantVSCode bool

	// 根据配置模式执行不同流程
	if configMode == 1 {
```

- [ ] **Step 2：运行现有测试，确认编译通过**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./...
```
期望：所有测试通过，无编译错误

---

### Task 2：在 API 验证成功后插入问询

**Files:**
- Modify: `dmxapi-claude-code.go`（约第 2238–2239 行）

- [ ] **Step 1：在 `printSuccess("API 连接验证成功!")` 后面、`} else if configMode == 3 {` 之前插入问询**

找到以下代码（约第 2238–2239 行）：
```go
		printSuccess("API 连接验证成功!")
	} else if configMode == 3 {
```

将其改为：
```go
		printSuccess("API 连接验证成功!")
		// 提前询问附加配置意向
		fmt.Println()
		wantTeams = styledConfirm("是否同时配置 Agent Teams 功能")
		fmt.Println()
		wantVSCode = styledConfirm("是否同时配置 VSCode 插件")
	} else if configMode == 3 {
```

注意：新增代码在 `configMode == 1` 的 `if` 块**内部**，因此它们只在 mode 1 时执行。

- [ ] **Step 2：运行测试**

```bash
go test ./...
```
期望：PASS

---

### Task 3：删除保存后的追问块，改为条件执行

**Files:**
- Modify: `dmxapi-claude-code.go`（约第 2277–2292 行）

**重要说明**：旧的追问块（`styledConfirm`）没有 `configMode` 条件判断，对所有模式都会执行。删除此块后，mode 2 也会同步去除末尾追问（符合规格中"仅配置模型"语义的要求）。

- [ ] **Step 1：找到旧的追问块并整体替换**

找到约第 2277–2292 行（`printSuccess("保存成功!")` 到 `// 打印摘要` 之间）：
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
```

替换为：
```go
	printSuccess("保存成功!")

	// 执行附加配置（仅 configMode==1 时 wantTeams/wantVSCode 可能为 true）
	if wantTeams {
		fmt.Println()
		configureAgentTeams(false)
	}
	if wantVSCode {
		fmt.Println()
		configureVSCode(cfg, false)
	}

	// 打印摘要
	printSummary(cfg)
```

- [ ] **Step 2：运行测试，确认一切正常**

```bash
go test ./...
```
期望：PASS

- [ ] **Step 3：手动冒烟测试（建议执行）**

```bash
go build -o /tmp/dmxapi-test . && /tmp/dmxapi-test
```

验证场景：

| 场景 | 验证点 |
|------|--------|
| 模式 1（从头配置） | API 验证成功后**立即**出现 Teams/VSCode 问询；模型配置后直接保存退出 |
| 模式 1 + 模型菜单按 q/ESC | 程序保存并退出，**不再**追问任何问题 |
| 模式 2（仅配置模型） | 模型配置完成后直接保存退出，**不出现** Teams/VSCode 问询 |
| 模式 3（解决400报错） | 不进入模型配置，直接保存退出，不出现 Teams/VSCode 问询 |
| 模式 4（配置实验性功能） | 行为与之前完全一致，配置完 Agent Teams 后立即退出 |
| 模式 5（配置 VSCode） | 行为与之前完全一致，配置完 VSCode 后立即退出 |

- [ ] **Step 4：提交**

```bash
git add dmxapi-claude-code.go
git commit -m "feat: 将 Agent Teams/VSCode 问询提前到 API 验证后、模型配置前

- configMode 1：API 验证成功后立即询问是否配置 Team/VSCode
- configMode 2：去除末尾 Team/VSCode 追问，回归「仅配置模型」语义
- ESC/q 在模型菜单：保存退出行为与提示文字一致
- 其他模式（3/4/5）行为不受影响

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```
