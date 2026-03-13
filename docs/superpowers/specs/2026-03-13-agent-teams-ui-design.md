# Agent Teams 配置页面 UI 改进设计

**日期**: 2026-03-13
**状态**: 已确认

## 背景

当前 `configureAgentTeams()` 页面使用通用确认菜单，选项文字（"是/否"）和状态描述（"未设置"）对用户不够直观，需针对启用/禁用场景做专项优化。

## 改动范围

### 1. 状态显示

| 状态 | 当前 | 改后 |
|------|------|------|
| 未启用 | `未设置`（灰色） | `未开启`（红色） |
| 已启用 | `已启用`（绿色） | 保持不变 |

### 2. 新增功能介绍文字

在"当前状态"下方、菜单上方插入两段说明：

```
Agent Teams 是 Claude Code 的实验性多智能体协作功能，
允许多个 AI 代理并行处理复杂任务。

关闭后将移除 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS
环境变量，Agent Teams 功能将停止工作。
```

### 3. 菜单选项文字

| 选项 | 当前标签 | 当前描述 | 改后标签 | 改后描述 |
|------|----------|----------|----------|----------|
| 选项1 | 是 | 确认修改 | 启用 | 开启 Agent Teams 功能 |
| 选项2 | 否 | 保持当前值不变 | 禁用 | 关闭 Agent Teams 功能 |

## 最终页面效果

```
┌─ 配置实验性 Agent Teams 功能

→ 当前状态: 未开启   （红色）

  Agent Teams 是 Claude Code 的实验性多智能体协作功能，
  允许多个 AI 代理并行处理复杂任务。

  关闭后将移除 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS
  环境变量，Agent Teams 功能将停止工作。

╭────────────────────────────────────────────╮
│         是否启用 Agent Teams 功能          │
├────────────────────────────────────────────┤
│ ❯ 启用  开启 Agent Teams 功能              │
│   禁用  关闭 Agent Teams 功能              │
╰────────────────────────────────────────────╯
```

## 技术实现方案

### 不影响现有调用方

`runConfirmMenu` 现有 5 处调用（Base URL / Token / 模型 / Agent Teams / 版本下载）中，只有 Agent Teams 需要"启用/禁用"语义，其余保留"是/否"。

### 代码改动

1. **`renderConfirmMenu`** → 重构为 `renderConfirmMenuCore(question string, labels [2]string, descs [2]string, selectedIdx int, linesPrinted int) int`，行数固定返回 `8`，与原实现一致。原 `renderConfirmMenu` 作为包装调用 core 并传入默认"是/否"文字。

2. **新增 `runEnableDisableMenu(question string) bool`** → 调用 `renderConfirmMenuCore`，传入"启用/禁用"文字。降级路径（非终端）同样更新 `MenuItem` 文字。

3. **`configureAgentTeams`**：
   - 状态文字改为"未开启" + `colorRed`（复用已有 `colorRed = "\033[31m"` 常量）
   - 新增两段介绍文字，使用 `fmt.Printf("  ...\n")` 输出以保持两空格缩进
   - 调用 `runEnableDisableMenu` 替代 `runConfirmMenu`

### 受影响文件

- `dmxapi-claude-code.go`（唯一文件）
