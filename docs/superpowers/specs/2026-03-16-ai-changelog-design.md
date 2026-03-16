# AI 增强自动更新日志 — 设计规范

## 概述

在 CNB 流水线的 tag 发布流程中，新增 AI 润色环节：git-cliff 生成原始变更日志后，调用 Anthropic API 将技术性 commit 改写为用户友好的中文更新说明，写入仓库的 `CHANGELOG.md` 并同步用于 Release 描述。

## 目标

- 用户在仓库中可查看 `CHANGELOG.md`，了解每个版本的变更内容
- 更新内容经 AI 润色，语言通俗易懂，而非原始 commit message
- 发版流程全自动，无需手动维护

## 非目标

- 不在 CLI 工具中展示更新日志
- 不生成网页版更新日志
- 不对历史版本做 AI 处理（仅当前版本；首次生成使用原始 git-cliff 输出）

## 流程设计

### 触发条件

Tag 推送触发 CNB 流水线（已有机制，无需新增）。

### 步骤编排与依赖关系

在现有 `.cnb.yml` tag_push 流程中，按如下顺序执行：

```
Step 0: sync tag to github（已有）
Step 1: 生成更新日志 / git-cliff（已有）
Step 2: AI 润色更新日志（新增，依赖 Step 1 的 RELEASE_NOTES）
Step 3: 更新 CHANGELOG.md 并推送回 main（新增，依赖 Step 2 的 AI_RELEASE_NOTES）
Step 4: 编译各平台二进制（已有，与 Step 2/3 并行，无依赖关系）
Step 5: 创建 Release（已有但修改，依赖 Step 2 和 Step 4）— 改用 AI_RELEASE_NOTES
Step 6: 上传 Release 附件（已有）
Step 7: 清理旧版本（已有）
```

> **注意**: AI 润色步骤（Step 2/3）与编译步骤（Step 4）无依赖关系，可并行执行。"创建 Release"（Step 5）需要等待 AI 润色和编译都完成后再执行。

### AI 调用细节

**接口**: Anthropic `/v1/messages`

**环境变量**（需在远程 env.yml 中新增，路径: `https://cnb.cool/dmxapi/claude-code-my/-/blob/main/env.yml`）:
- `AI_API_URL` — API 地址（如 `https://api.dmxapi.cn`）
- `AI_API_KEY` — API 密钥
- `AI_MODEL` — 模型名（如 `claude-sonnet-4-20250514`）

**CI 镜像**: 使用 `cnbcool/default-build-env`（已确认包含 `curl` 和 `jq`）。

**Prompt 设计**:
```
你是一个软件更新日志编辑器。请将以下 git 变更日志改写为面向普通用户的中文更新说明。

要求：
1. 保留版本号和日期
2. 用通俗易懂的语言描述每个变更，去掉技术术语
3. 保留 emoji 分类图标
4. 每条说明控制在一行以内
5. 输出必须以 "## " 开头（Markdown 二级标题）
6. 只输出改写后的更新日志，不要输出其他内容

原始变更日志：
{RELEASE_NOTES}
```

**实现方式**: 在 `.cnb.yml` 中用 shell script + curl 调用，使用 `jq` 解析返回的 JSON 中 `content[0].text` 字段。

**输出验证**: AI 返回内容必须满足：以 `## ` 开头且长度不超过 5000 字符。不满足则回退使用原始 `RELEASE_NOTES`。

**重试与错误处理**: AI 调用失败时自动重试 1 次（间隔 3 秒）。如果重试仍失败（网络错误、API 异常、HTTP 非 200、输出验证失败），回退使用原始 `RELEASE_NOTES`，不阻塞发布流程。

### CHANGELOG.md 文件结构

```markdown
# 更新日志

所有版本的更新内容记录。

## 1.5.1 (2026-03-16)

### 🐛 问题修复
- 修复了 Windows CMD 安装脚本因换行符问题导致安装失败
- 新增对 ARM64 架构安装脚本的支持

## 1.5.0 (2026-03-15)

### 🚀 新功能
- 支持 VSCode JSONC 格式配置文件解析

### 🐛 问题修复
- 修复 Windows 启动时 Logo 和版本号显示异常
- 修复安装脚本编码问题
```

### 插入逻辑

新版本内容插入到 `CHANGELOG.md` 中第一个 `## ` 标题之前、文件头部描述行之后。

由于新内容是多行 Markdown（包含 `#`、换行、特殊字符），`sed` 内联插入不可靠。使用文件拆分拼接方式：

```bash
# 1. 找到第一个 "## " 的行号
FIRST_HEADING=$(grep -n '^## ' CHANGELOG.md | head -1 | cut -d: -f1)

# 2. 拆分：头部（标题+描述）和正文（各版本内容）
head -n $((FIRST_HEADING - 1)) CHANGELOG.md > /tmp/changelog_header.md
tail -n +${FIRST_HEADING} CHANGELOG.md > /tmp/changelog_body.md

# 3. 拼接：头部 + 新内容 + 空行 + 原正文
cat /tmp/changelog_header.md > CHANGELOG.md
echo "" >> CHANGELOG.md
echo "${AI_RELEASE_NOTES}" >> CHANGELOG.md
echo "" >> CHANGELOG.md
cat /tmp/changelog_body.md >> CHANGELOG.md
```

如果文件中没有 `## ` 行（不应出现，首次生成已包含历史版本），则将新内容追加到文件末尾。

> **注意**: 此 shell 脚本在 CI 的 Linux 环境中执行（`cnbcool/default-build-env`），无需考虑 macOS BSD 工具差异。

### 首次生成

在本地手动执行以下命令生成全量历史 CHANGELOG.md（使用原始 git-cliff 输出，不经过 AI 处理）：

```bash
git-cliff --config cliff.toml -o CHANGELOG.md
```

然后在生成的文件顶部手动添加标题：

```markdown
# 更新日志

所有版本的更新内容记录。
```

提交并推送到 main 分支。此后由流水线自动维护。

> **注意**: `CHANGELOG.md` 不在 `.gitignore` 中，可以正常被 git 追踪。

### 推送回 main 的处理

新增步骤需要 git push 权限：
- 使用已有的 `GIT_USERNAME` 和相关 token（从 env.yml imports）
- commit message 格式: `docs: update CHANGELOG.md for {TAG}`
- 推送目标: main 分支

> **副作用**: push 回 main 会触发现有的 "自动同步到 GitHub" 流水线（main.push），这是预期行为——CHANGELOG.md 的变更也应同步到 GitHub 镜像。不会触发 tag_push 流水线。

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `CHANGELOG.md` | 新建 | 首次用 `git-cliff --config cliff.toml -o CHANGELOG.md` 生成全量历史 |
| `.cnb.yml` | 修改 | tag_push 中新增 "AI 润色更新日志" 和 "更新 CHANGELOG.md" 两个步骤；Release description 改用 AI_RELEASE_NOTES |
| `env.yml`（远程: `https://cnb.cool/dmxapi/claude-code-my/-/blob/main/env.yml`）| 修改 | 新增 `AI_API_URL` / `AI_API_KEY` / `AI_MODEL` 三个环境变量 |

> `cliff.toml` 无需修改。header 模板由 CHANGELOG.md 文件自身的固定头部代替。

## 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| AI API 调用失败 | 回退使用原始 git-cliff 输出，不阻塞发布 |
| AI 输出格式异常 | 验证输出以 `## ` 开头且长度合理，否则回退 |
| push 回 main 触发循环 | `docs:` 前缀的 commit 不会触发 tag_push；触发 main.push 的 GitHub 同步是预期行为 |
| CI 镜像缺少工具 | 使用 `cnbcool/default-build-env`，已包含 curl 和 jq |
| Token 消耗 | 仅处理当前版本（通常几百 token），成本可忽略 |
| 快速连续推送多个 tag | 两个流水线同时 push CHANGELOG.md 到 main 可能冲突；实际操作中极少连续推 tag，可接受 |
