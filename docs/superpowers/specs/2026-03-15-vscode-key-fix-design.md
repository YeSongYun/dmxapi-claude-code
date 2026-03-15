# VSCode Plugin 配置键名修复设计

**日期**: 2026-03-15
**状态**: 已批准

---

## 问题背景

用户通过本工具配置 VSCode Plugin 后，配置摘要中"VSCode Plugin"一栏始终显示"未配置"，但实际上 VSCode `settings.json` 中已存在有效配置。

### 根因

工具使用了错误的 JSON 键名：

| 位置 | 键名 | 说明 |
|------|------|------|
| 工具写入/检测 | `claude-code.environmentVariables` | 连字符格式，VSCode 不识别 |
| VSCode 扩展实际读取 | `claudeCode.environmentVariables` | 驼峰格式，VSCode 插件设置规范 |

VSCode 扩展 ID 为 `claude-code`，但在 `settings.json` 中对应的配置前缀为 `claudeCode`（驼峰转换），这是 VSCode 扩展设置的命名规范。工具从 VSCode 功能引入之初就使用了错误键名，导致：

1. **写入无效**：工具写入的 `claude-code.environmentVariables` 不被 VSCode 扩展读取
2. **检测失效**：`isVSCodeConfigured` 检测旧键，而用户 settings.json 中是正确的新键

---

## 设计方案（方案 B）

### 核心变更

新增向后兼容常量，修改写入键为正确值：

```go
vscodeEnvKey    = "claudeCode.environmentVariables"   // 正确键，用于写入和检测
vscodeEnvKeyOld = "claude-code.environmentVariables"  // 旧键，仅用于向后兼容检测
```

### 变更范围

**1. 常量定义**
- 将 `vscodeEnvKey` 值改为 `"claudeCode.environmentVariables"`
- 新增 `vscodeEnvKeyOld = "claude-code.environmentVariables"` 用于兼容检测

**2. `mergeVSCodeSettings` 函数**
- 写入键由旧键改为新键 `claudeCode.environmentVariables`
- 写入结构不变（仍为 `[]map[string]string` 数组格式）

**3. `isVSCodeConfigured` 函数**
- 检测逻辑改为：新键 OR 旧键均视为已配置
- 保证已用旧版工具配置过的用户不出现误报

**4. 代码注释**
- 函数文档、行内注释中的旧键名同步更新

### 不在本次范围

- 不主动清理 settings.json 中残留的旧键 `claude-code.environmentVariables`
- 不修改写入数据结构（字段名、格式等）
- 不调整其他 VSCode 相关逻辑

---

## 测试更新

| 测试用例 | 当前断言 | 修改后断言 |
|----------|----------|------------|
| `TestMergeVSCodeSettings` - 写入键断言 | `claude-code.environmentVariables` 存在 | `claudeCode.environmentVariables` 存在 |
| `TestIsVSCodeConfigured` - 已有配置 | 含旧键返回 true | 含新键返回 true；含旧键也返回 true（新增兼容用例） |

---

## 预期效果

| 场景 | 修复前 | 修复后 |
|------|--------|--------|
| 用户 settings.json 含 `claudeCode.environmentVariables` | 显示"未配置" | 显示"已配置" |
| 用户 settings.json 含 `claude-code.environmentVariables`（旧版） | 显示"已配置" | 显示"已配置"（兼容保留） |
| 工具新写入配置 | 写入无效键，VSCode 扩展不读取 | 写入正确键，VSCode 扩展可读取 |

---

## 文件影响

- `dmxapi-claude-code.go`：常量定义、`mergeVSCodeSettings`、`isVSCodeConfigured`、相关注释
- `dmxapi-claude-code_test.go`：`TestMergeVSCodeSettings`、`TestIsVSCodeConfigured`
