# 先配模型再验证 API 实现计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `configMode == 1`（从头配置）流程中，将模型配置步骤提前到 API 连接验证之前，使验证时使用的模型与用户实际配置的模型一致。

**Architecture:** 仅修改 `main()` 函数内 3 处代码，原子性实施。无新文件，无新函数，无接口变更。

**Tech Stack:** Go 1.x，单文件项目 `dmxapi-claude-code.go`

---

## Chunk 1: 三处原子性代码改动

**Spec 参考:** `docs/superpowers/specs/2026-03-15-model-first-then-verify-design.md`

**Files:**
- Modify: `dmxapi-claude-code.go:2237-2243`（改动 1 + 改动 2）
- Modify: `dmxapi-claude-code.go:2307-2310`（改动 3）

> ⚠️ 三处改动必须一次性全部完成后再 build 验证，单独应用改动 3 会导致 mode 1 跳过模型配置。

---

- [ ] **步骤 1：改动 1+2 — Token 配置后插入模型配置调用，并删除兜底赋值**

  定位 `dmxapi-claude-code.go` 第 2237-2243 行，当前内容：

  ```go
  		// 配置 Auth Token
  		cfg.AuthToken = getNewAuthToken(cfg.AuthToken, hostname)

  		// 验证 API 连接（循环直到成功）
  		// 若用户首次配置尚未选择模型，使用默认模型进行验证
  		if cfg.Model == "" {
  			cfg.Model = defaultModel
  		}
  		fmt.Println()
  ```

  替换为：

  ```go
  		// 配置 Auth Token
  		cfg.AuthToken = getNewAuthToken(cfg.AuthToken, hostname)

  		// 配置模型（在 API 验证前，使验证所用模型与用户选择一致）
  		fmt.Println()
  		configureModels(&cfg)

  		// 验证 API 连接（循环直到成功）
  		fmt.Println()
  ```

  改动说明：
  - 新增 `configureModels(&cfg)` 调用（改动 1）
  - 删除原 `if cfg.Model == ""` 兜底块（改动 2）；`configureModels` 内部已保证 `cfg.Model` 非空

---

- [ ] **步骤 2：改动 3 — 将分支块之后的模型配置条件改为仅 mode 2**

  定位 `dmxapi-claude-code.go` 第 2307-2310 行，当前内容：

  ```go
  	// 配置模型（模式 3 直接跳过）
  	if configMode != 3 {
  		configureModels(&cfg)
  	}
  ```

  替换为：

  ```go
  	// 配置模型（仅 mode 2；mode 1 已在上方提前配置）
  	if configMode == 2 {
  		configureModels(&cfg)
  	}
  ```

---

- [ ] **步骤 3：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```

  预期：无报错，生成可执行文件。

---

- [ ] **步骤 4：运行现有单元测试**

  ```bash
  go test ./... -v
  ```

  预期：所有测试 PASS（本次改动不涉及任何被测函数，测试结果应与改动前完全相同）。

---

- [ ] **步骤 5：手动验证核心流程（mode 1）**

  运行程序，选择"从头配置"（mode 1），观察流程顺序：

  1. 配置 Base URL
  2. 配置 Auth Token
  3. **立即显示模型配置菜单**（期望：`配置模型设置` 标题出现在 API 验证之前）
  4. 完成模型配置
  5. API 连接验证（使用刚配置的模型）

  验证项：
  - [ ] 模型配置菜单出现在 API 验证之前
  - [ ] API 验证提示"正在验证 API 连接..."出现在模型配置之后
  - [ ] 验证成功后显示 Teams/VSCode 问询
  - [ ] mode 2（仅配置模型）流程不受影响

---

- [ ] **步骤 6：提交**

  ```bash
  git add dmxapi-claude-code.go
  git commit -m "feat: mode 1 先配置模型再验证 API 连接"
  ```
