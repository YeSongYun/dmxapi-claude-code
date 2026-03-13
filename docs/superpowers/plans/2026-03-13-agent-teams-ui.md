# Agent Teams 配置页面 UI 改进 实现计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 改进 Agent Teams 配置页面：状态文字"未设置"→"未开启"（红色）、新增功能介绍文字、菜单选项改为"启用/禁用"。

**Architecture:** 将 `renderConfirmMenu` 的 labels/descs 提取为参数（`renderConfirmMenuCore`），原函数作为"是/否"包装器，新增 `runEnableDisableMenu` 作为"启用/禁用"包装器，最后更新 `configureAgentTeams` 调用新函数并补充介绍文字。

**Tech Stack:** Go，单文件 `dmxapi-claude-code.go`

---

## Chunk 1: 重构渲染函数 + 新增启用禁用菜单

### Task 1: 提取 `renderConfirmMenuCore`

**Files:**
- Modify: `dmxapi-claude-code.go:1072-1124`

- [ ] **Step 1: 在 `renderConfirmMenu` 正上方新增 `renderConfirmMenuCore`**

  在第 1072 行（`renderConfirmMenu` 函数）之前插入：

  ```go
  // renderConfirmMenuCore 渲染确认菜单核心逻辑，返回渲染行数（固定8行）
  // selectedIdx: 0=选项1, 1=选项2
  func renderConfirmMenuCore(question string, labels [2]string, descs [2]string, selectedIdx int, linesPrinted int) int {
  	if linesPrinted > 0 {
  		fmt.Printf("\033[%dA", linesPrinted)
  	}
  	const innerW = 44
  	border := strings.Repeat("─", innerW)
  	fmt.Printf("╭%s╮\033[K\r\n", border)

  	questionW := visibleLength(question)
  	lPad := (innerW - questionW) / 2
  	if lPad < 0 {
  		lPad = 0
  	}
  	rPad := innerW - questionW - lPad
  	if rPad < 0 {
  		rPad = 0
  	}
  	fmt.Printf("│%s%s%s%s%s│\033[K\r\n",
  		strings.Repeat(" ", lPad), styleBold+colorBrightWhite, question, colorReset, strings.Repeat(" ", rPad))
  	fmt.Printf("├%s┤\033[K\r\n", border)

  	for i := 0; i < 2; i++ {
  		label := labels[i]
  		desc := descs[i]
  		pad := innerW - 5 - visibleLength(label) - visibleLength(desc)
  		if pad < 0 {
  			pad = 0
  		}
  		if i == selectedIdx {
  			fmt.Printf("│ %s❯ %s%s  %s%s%s%s│\033[K\r\n",
  				colorBrightCyan+styleBold,
  				label, colorReset,
  				colorBrightCyan, desc, colorReset,
  				strings.Repeat(" ", pad))
  		} else {
  			fmt.Printf("│ %s  %s%s  %s%s%s%s│\033[K\r\n",
  				styleDim,
  				label, colorReset,
  				styleDim, desc, colorReset,
  				strings.Repeat(" ", pad))
  		}
  	}

  	fmt.Printf("╰%s╯\033[K\r\n", border)
  	fmt.Printf("\033[K\r\n")
  	fmt.Printf("  %s↑↓ 导航%s  %sEnter 确认%s\033[K\r\n",
  		styleDim, colorReset, styleDim, colorReset)
  	return 8
  }
  ```

- [ ] **Step 2: 将原 `renderConfirmMenu` 改为调用 core 的包装器**

  将第 1072-1124 行的 `renderConfirmMenu` 函数体替换为：

  ```go
  // renderConfirmMenu 渲染默认确认菜单（是/否），返回渲染行数（固定8行）
  // selectedIdx: 0=是, 1=否
  func renderConfirmMenu(question string, selectedIdx int, linesPrinted int) int {
  	return renderConfirmMenuCore(
  		question,
  		[2]string{"是", "否"},
  		[2]string{"确认修改", "保持当前值不变"},
  		selectedIdx,
  		linesPrinted,
  	)
  }
  ```

- [ ] **Step 3: 编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code && go build ./...
  ```

  预期：无编译错误，无输出。

- [ ] **Step 4: 运行已有测试，确认不回归**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./...
  ```

  预期：`ok  	dmxapi_claude_code`

- [ ] **Step 5: 提交**

  ```bash
  git add dmxapi-claude-code.go
  git commit -m "refactor: 提取 renderConfirmMenuCore，支持自定义 labels/descs"
  ```

---

### Task 2: 新增 `runEnableDisableMenu`

**Files:**
- Modify: `dmxapi-claude-code.go`（在 `runConfirmMenu` 结束后插入）

- [ ] **Step 1: 找到 `runConfirmMenu` 结束位置**

  当前 `runConfirmMenu` 函数约在第 1126-1171 行。在其结束的 `}` 后插入新函数：

  ```go
  // runEnableDisableMenu 运行启用/禁用确认菜单，返回是否启用（true=启用，false=禁用）
  // 默认选中"禁用"（索引1）
  func runEnableDisableMenu(question string) bool {
  	restore, err := enterRawMode()
  	if err != nil {
  		// 降级：非终端时用数字选项
  		printMenu(question, []MenuItem{
  			{"1", "启用", "开启 Agent Teams 功能"},
  			{"2", "禁用", "关闭 Agent Teams 功能"},
  		})
  		fmt.Println()
  		for {
  			input := styledInput("选项")
  			switch input {
  			case "1":
  				return true
  			case "2":
  				return false
  			default:
  				printError("请输入 1 或 2")
  			}
  		}
  	}
  	defer restore()

  	selectedIdx := 1 // 默认"禁用"
  	linesPrinted := 0

  	for {
  		linesPrinted = renderConfirmMenuCore(
  			question,
  			[2]string{"启用", "禁用"},
  			[2]string{"开启 Agent Teams 功能", "关闭 Agent Teams 功能"},
  			selectedIdx,
  			linesPrinted,
  		)
  		key := readRawKey()
  		switch key {
  		case KeyUp:
  			selectedIdx = (selectedIdx - 1 + 2) % 2
  		case KeyDown:
  			selectedIdx = (selectedIdx + 1) % 2
  		case KeyEnter:
  			restore()
  			clearMenuLines(linesPrinted)
  			return selectedIdx == 0
  		case KeyEsc:
  			restore()
  			clearMenuLines(linesPrinted)
  			return false
  		}
  	}
  }
  ```

- [ ] **Step 2: 编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code && go build ./...
  ```

  预期：无编译错误。

- [ ] **Step 3: 运行测试**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./...
  ```

  预期：`ok  	dmxapi_claude_code`

- [ ] **Step 4: 提交**

  ```bash
  git add dmxapi-claude-code.go
  git commit -m "feat: 新增 runEnableDisableMenu，用于启用/禁用语义菜单"
  ```

---

## Chunk 2: 更新 configureAgentTeams 页面

### Task 3: 更新 `configureAgentTeams`

**Files:**
- Modify: `dmxapi-claude-code.go:1475-1488`

- [ ] **Step 1: 将状态"未设置"改为红色"未开启"**

  找到第 1484 行：
  ```go
  printInfo(fmt.Sprintf("当前状态: %s未设置%s", styleDim, colorReset))
  ```
  改为：
  ```go
  printInfo(fmt.Sprintf("当前状态: %s未开启%s", colorRed, colorReset))
  ```

- [ ] **Step 2: 在状态行下方插入功能介绍文字**

  保留第 1486 行的 `fmt.Println()` 不动，将以下代码整体替换现有的：
  ```go
  fmt.Println()

  enable := runEnableDisableMenu("是否启用 Agent Teams 功能")
  ```
  替换为（注意：此步骤包含 Step 3 的调用替换，一起完成避免中间状态）：
  ```go
  fmt.Println()
  fmt.Printf("  Agent Teams 是 Claude Code 的实验性多智能体协作功能，\n")
  fmt.Printf("  允许多个 AI 代理并行处理复杂任务。\n")
  fmt.Println()
  fmt.Printf("  关闭后将移除 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS\n")
  fmt.Printf("  环境变量，Agent Teams 功能将停止工作。\n")
  fmt.Println()

  enable := runEnableDisableMenu("是否启用 Agent Teams 功能")
  ```

- [ ] **Step 3: ~~将 `runConfirmMenu` 调用改为 `runEnableDisableMenu`~~（已在 Step 2 代码块中完成）**

- [ ] **Step 4: 编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code && go build ./...
  ```

  预期：无编译错误。

- [ ] **Step 5: 运行全量测试**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code && go test ./...
  ```

  预期：`ok  	dmxapi_claude_code`

- [ ] **Step 6: 人工验证页面效果**

  运行程序，进入"配置实验性功能 → Agent Teams"，目视确认：
  - 当前状态为红色"未开启"（未启用时）或绿色"已启用"（已启用时）
  - 两段介绍文字正确显示，各有两空格缩进
  - 菜单选项显示"启用 / 开启 Agent Teams 功能"和"禁用 / 关闭 Agent Teams 功能"
  - 上下键导航、Enter 确认功能正常

- [ ] **Step 7: 提交**

  ```bash
  git add dmxapi-claude-code.go
  git commit -m "feat: Agent Teams 页面新增功能介绍、状态改为红色未开启、菜单改为启用/禁用"
  ```
