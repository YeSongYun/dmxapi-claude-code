# AI 增强更新日志 实施计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 CNB 流水线中集成 AI 润色，自动将 git-cliff 原始日志改写为用户友好的 CHANGELOG.md 并用于 Release 描述。

**Architecture:** git-cliff 生成原始变更日志 → Anthropic API 润色为用户友好语言 → 插入 CHANGELOG.md 头部并 push 回 main → Release 描述使用 AI 润色内容。首次 CHANGELOG.md 由本地 git-cliff 生成全量历史。

**Tech Stack:** git-cliff, Anthropic Messages API (`/v1/messages`), CNB CI/CD (`.cnb.yml`), shell (curl + jq)

**Design Spec:** `docs/superpowers/specs/2026-03-16-ai-changelog-design.md`

---

## Chunk 1: 首次生成 CHANGELOG.md

### Task 1: 安装 git-cliff 并生成全量 CHANGELOG.md

**Files:**
- Create: `CHANGELOG.md`

- [ ] **Step 1: 安装 git-cliff**

```bash
brew install git-cliff
```

- [ ] **Step 2: 验证 cliff.toml 存在**

```bash
ls -la cliff.toml
```

Expected: 文件存在，显示文件信息。

- [ ] **Step 3: 生成全量历史 CHANGELOG.md**

```bash
cd /Users/yesongyun/代码/dmxapi_claude_code
git-cliff --config cliff.toml -o CHANGELOG.md
```

- [ ] **Step 4: 验证生成结果**

```bash
head -30 CHANGELOG.md
```

Expected: 看到最新版本（1.5.1）到最早版本的变更日志，按 emoji 分组。

### Task 2: 添加 CHANGELOG.md 文件头部

**Files:**
- Modify: `CHANGELOG.md`（在文件顶部插入标题）

- [ ] **Step 1: 在 CHANGELOG.md 顶部插入标题和描述**

在文件最开头插入以下 3 行（加 1 个空行与正文隔开）：

```markdown
# 更新日志

所有版本的更新内容记录。

```

保留 git-cliff 生成的所有内容在后面。

- [ ] **Step 2: 验证文件结构正确**

```bash
head -10 CHANGELOG.md
```

Expected: 前 3 行是标题和描述，第 4 行空行，第 5 行开始是 `## 1.5.1 (...)` 格式的版本标题。

### Task 3: 提交 CHANGELOG.md

**Files:**
- `CHANGELOG.md`

- [ ] **Step 1: 提交**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG.md with full version history"
```

---

## Chunk 2: 修改 .cnb.yml — 添加 AI 润色步骤

> **关于并行执行**: CNB stages 在同一 `stages:` 数组中默认串行执行。AI 润色步骤和 CHANGELOG 更新步骤会在编译之前串行运行。虽然理论上可与编译并行，但 AI 调用通常只需几秒，串行的额外延迟可忽略，且避免了复杂的并行编排配置。

### Task 4: 在 .cnb.yml 中添加 "AI 润色更新日志" 步骤

**Files:**
- Modify: `.cnb.yml:54-62`（在 "生成更新日志" 步骤之后插入新步骤）

- [ ] **Step 1: 在 "生成更新日志" 步骤（第 62 行 `exports` 结束）之后，编译步骤之前，插入 AI 润色步骤**

在 `.cnb.yml` 的 `exports: stdout: RELEASE_NOTES` 之后、`# 2. 编译各平台二进制文件` 注释之前，插入以下 YAML：

```yaml
        # 2. AI 润色更新日志（调用 Anthropic API）
        - name: AI 润色更新日志
          image: cnbcool/default-build-env
          script:
            - |
              # 构造 AI 请求的 prompt
              PROMPT="你是一个软件更新日志编辑器。请将以下 git 变更日志改写为面向普通用户的中文更新说明。\n\n要求：\n1. 保留版本号和日期\n2. 用通俗易懂的语言描述每个变更，去掉技术术语\n3. 保留 emoji 分类图标\n4. 每条说明控制在一行以内\n5. 输出必须以 \"## \" 开头（Markdown 二级标题）\n6. 只输出改写后的更新日志，不要输出其他内容\n\n原始变更日志：\n${RELEASE_NOTES}"

              # 调用 Anthropic API（带 1 次重试）
              call_ai() {
                curl -s -w "\n%{http_code}" "${AI_API_URL}/v1/messages" \
                  -H "Content-Type: application/json" \
                  -H "x-api-key: ${AI_API_KEY}" \
                  -H "anthropic-version: 2023-06-01" \
                  -d "$(jq -n \
                    --arg model "${AI_MODEL}" \
                    --arg prompt "${PROMPT}" \
                    '{model: $model, max_tokens: 4096, messages: [{role: "user", content: $prompt}]}')"
              }

              RESPONSE=$(call_ai)
              HTTP_CODE=$(echo "$RESPONSE" | tail -1)
              BODY=$(echo "$RESPONSE" | sed '$d')

              # 重试逻辑
              if [ "$HTTP_CODE" != "200" ]; then
                echo "AI API 首次调用失败 (HTTP $HTTP_CODE)，3 秒后重试..."
                sleep 3
                RESPONSE=$(call_ai)
                HTTP_CODE=$(echo "$RESPONSE" | tail -1)
                BODY=$(echo "$RESPONSE" | sed '$d')
              fi

              # 提取 AI 输出
              if [ "$HTTP_CODE" = "200" ]; then
                AI_TEXT=$(echo "$BODY" | jq -r '.content[0].text // empty')
                # 验证输出格式
                if echo "$AI_TEXT" | head -1 | grep -q '^## ' && [ ${#AI_TEXT} -le 5000 ]; then
                  echo "$AI_TEXT"
                  exit 0
                fi
                echo "AI 输出格式验证失败，使用原始日志" >&2
              else
                echo "AI API 调用失败 (HTTP $HTTP_CODE)，使用原始日志" >&2
              fi

              # 回退：输出原始 RELEASE_NOTES
              echo "${RELEASE_NOTES}"
          exports:
            stdout: AI_RELEASE_NOTES
```

- [ ] **Step 2: 验证 YAML 语法**

```bash
python3 -c "import yaml; yaml.safe_load(open('.cnb.yml'))" && echo "YAML valid" || echo "YAML invalid"
```

Expected: `YAML valid`

### Task 5: 在 .cnb.yml 中添加 "更新 CHANGELOG.md 并推送" 步骤

**Files:**
- Modify: `.cnb.yml`（在 AI 润色步骤之后、编译步骤之前插入）

- [ ] **Step 1: 在 AI 润色步骤之后、编译步骤之前，插入 CHANGELOG 更新步骤**

```yaml
        # 3. 更新 CHANGELOG.md 并推送回 main
        - name: 更新 CHANGELOG.md
          image: cnbcool/default-build-env
          script:
            - |
              git config user.name "CNB Bot"
              git config user.email "bot@cnb.cool"

              # 基于 origin/main 创建临时分支（避免 detached HEAD 问题）
              git fetch origin main
              git checkout -b temp-changelog origin/main

              # 找到第一个 "## " 标题的行号
              FIRST_HEADING=$(grep -n '^## ' CHANGELOG.md | head -1 | cut -d: -f1)

              if [ -n "$FIRST_HEADING" ]; then
                # 拆分文件：头部（标题+描述）和正文（各版本内容）
                head -n $((FIRST_HEADING - 1)) CHANGELOG.md > /tmp/changelog_header.md
                tail -n +${FIRST_HEADING} CHANGELOG.md > /tmp/changelog_body.md

                # 拼接：头部 + 新内容 + 空行 + 原正文（与 spec 一致）
                cat /tmp/changelog_header.md > CHANGELOG.md
                echo "" >> CHANGELOG.md
                echo "${AI_RELEASE_NOTES}" >> CHANGELOG.md
                echo "" >> CHANGELOG.md
                cat /tmp/changelog_body.md >> CHANGELOG.md
              else
                # 没有 ## 行，追加到末尾
                echo "" >> CHANGELOG.md
                echo "${AI_RELEASE_NOTES}" >> CHANGELOG.md
              fi

              # 提交并推送临时分支到 main
              git add CHANGELOG.md
              git commit -m "docs: update CHANGELOG.md for ${CNB_BRANCH}"
              git remote set-url origin https://${GIT_USERNAME}:${GIT_ACCESS_TOKEN}@cnb.cool/dmxapi/dmxapi_claude_code.git
              git push origin temp-changelog:main
```

- [ ] **Step 2: 验证 YAML 语法**

```bash
python3 -c "import yaml; yaml.safe_load(open('.cnb.yml'))" && echo "YAML valid" || echo "YAML invalid"
```

Expected: `YAML valid`

### Task 6: 修改 "创建 Release" 步骤使用 AI_RELEASE_NOTES

**Files:**
- Modify: `.cnb.yml`（Release 步骤中的 `${RELEASE_NOTES}` 替换为 `${AI_RELEASE_NOTES}`）

- [ ] **Step 1: 替换 Release description 中的变量**

在 "创建 Release" 步骤的 `description` 字段中，将 `${RELEASE_NOTES}` 替换为 `${AI_RELEASE_NOTES}`。

- [ ] **Step 2: 验证 YAML 语法**

```bash
python3 -c "import yaml; yaml.safe_load(open('.cnb.yml'))" && echo "YAML valid" || echo "YAML invalid"
```

Expected: `YAML valid`

### Task 7: 统一更新 .cnb.yml 中的注释编号

**Files:**
- Modify: `.cnb.yml`（更新所有步骤的注释编号，确保顺序一致）

- [ ] **Step 1: 更新所有注释编号**

Task 4 和 Task 5 插入了两个新步骤后，原有步骤的编号需要统一调整：

| 原注释 | 新注释 |
|--------|--------|
| `# 1. 生成更新日志` | 保持不变 |
| （新增）`# 2. AI 润色更新日志` | 已由 Task 4 添加 |
| （新增）`# 3. 更新 CHANGELOG.md` | 已由 Task 5 添加 |
| `# 2. 编译各平台二进制文件` | 改为 `# 4. 编译各平台二进制文件` |
| `# 3. 先创建 Release` | 改为 `# 5. 先创建 Release` |
| `# 4. 再上传附件` | 改为 `# 6. 再上传附件` |
| `# 5. 清理旧版本` | 改为 `# 7. 清理旧版本` |

- [ ] **Step 2: 验证编号无冲突**

在 `.cnb.yml` 中搜索所有 `# N.` 格式的注释，确认编号从 0 到 7 连续无重复。

### Task 8: 提交 .cnb.yml 变更

**Files:**
- `.cnb.yml`

- [ ] **Step 1: 提交**

```bash
git add .cnb.yml
git commit -m "ci: add AI-enhanced changelog generation to tag push pipeline"
```

---

## Chunk 3: 用户手动步骤（env.yml 配置）

### Task 9: 用户在远程 env.yml 中添加 AI 环境变量

**说明:** 此步骤需要用户手动在远程仓库 `https://cnb.cool/dmxapi/claude-code-my/-/blob/main/env.yml` 中添加以下三个环境变量：

- [ ] **Step 1: 添加 AI_API_URL**

在 `env.yml` 中添加 `AI_API_URL` 变量，值为 Anthropic API 的地址（如 `https://api.dmxapi.cn`）。

- [ ] **Step 2: 添加 AI_API_KEY**

在 `env.yml` 中添加 `AI_API_KEY` 变量，值为 API 密钥。

- [ ] **Step 3: 添加 AI_MODEL**

在 `env.yml` 中添加 `AI_MODEL` 变量，值为使用的模型名（如 `claude-sonnet-4-20250514`）。

- [ ] **Step 4: 验证变量已生效**

推送一个测试 tag 触发流水线，观察 "AI 润色更新日志" 步骤是否正确执行。
