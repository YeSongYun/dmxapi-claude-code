# Windows 版本一致性与 Bug 修复实施计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 Windows 版本中的 4 个问题：install.ps1 乱码、Logo 无 ASCII art/版本号、VSCode JSONC 解析失败、L1 模型菜单按 Esc 不清除。

**Architecture:** 3 处修改在 `dmxapi-claude-code.go`（删除 Windows Logo 分支、新增 `stripJSONC` 函数并接入 `mergeVSCodeSettings`、修复 `runL1Menu` Esc 路径），1 处修改在 `install.ps1`（UTF-8 编码初始化）。

**Tech Stack:** Go 1.21+，PowerShell 7+，Go 标准库（`bytes`、`regexp`、`encoding/json`）

**Spec:** `docs/superpowers/specs/2026-03-16-windows-consistency-fix-design.md`

---

## Chunk 1：install.ps1 UTF-8 编码修复 + L1 菜单 Esc 清除

### Task 1：install.ps1 添加 UTF-8 编码设置

**Files:**
- Modify: `install.ps1`（第 1 行之后插入 2 行）

- [ ] **Step 1：读取并确认当前第 1 行内容**

  确认文件首行是 `$VERSION = "v1.4.9"`，后续插入不会影响版本变量。

- [ ] **Step 2：在 `$VERSION` 行之后插入 UTF-8 编码设置**

  在 `install.ps1` 修改为：
  ```powershell
  $VERSION = "v1.4.9"

  # 设置控制台输出为 UTF-8，确保中文正常显示
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
  $OutputEncoding = [System.Text.Encoding]::UTF8
  ```

  - `[Console]::OutputEncoding`：控制 `Write-Host` 的输出编码
  - `$OutputEncoding`：控制管道传输编码（防御性设置）

- [ ] **Step 3：手动验证（无自动化测试，属 PowerShell 运行时行为）**

  在 Windows PowerShell 7+ 中运行：
  ```powershell
  # 模拟测试：临时设置编码后输出中文
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
  Write-Host "正在下载 dmxapi..."
  ```
  预期：输出 `正在下载 dmxapi...`（不是 `???`）

---

### Task 2：修复 `runL1Menu` 按 Esc 不清除菜单

**Files:**
- Modify: `dmxapi-claude-code.go`，函数 `runL1Menu()`，约第 1812 行

- [ ] **Step 1：定位当前 `KeyEsc` 分支**

  在 `dmxapi-claude-code.go` 约第 1812 行找到：
  ```go
  case KeyEsc:
      restore()
      return
  ```

- [ ] **Step 2：添加 `clearMenuLines` 调用**

  修改为：
  ```go
  case KeyEsc:
      restore()
      clearMenuLines(linesPrinted)
      return
  ```

  原理：`clearMenuLines(n)` 向上移动光标 n 行并逐行清除，与 `runL2Menu` 和 `runItemMenu` 的行为保持一致。

- [ ] **Step 3：运行现有测试，确认无回归**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go test ./... -v
  ```
  预期：所有测试通过，无新增失败。

- [ ] **Step 4：Commit**

  ```bash
  git add install.ps1 dmxapi-claude-code.go
  git commit -m "fix: install.ps1 UTF-8 编码 + L1 模型菜单 Esc 后清除残留"
  ```

---

## Chunk 2：Windows Logo 修复（删除早返回分支）

### Task 3：删除 Windows Logo 早返回分支

**Files:**
- Modify: `dmxapi-claude-code.go`，函数 `printLogo()`，约第 229-235 行

- [ ] **Step 1：定位并删除 Windows 专用分支**

  删除以下代码块（第 229-235 行，纯删除，不替换任何内容，删除后下一行即为 `logo := []string{...}`）：
  ```go
  if runtime.GOOS == "windows" {
      fmt.Println()
      fmt.Println(colorCyan + styleBold + "  === DMXAPI ===" + colorReset)
      fmt.Println(styleDim + "  Claude Code CLI 配置工具" + colorReset)
      fmt.Printf("  %s%s/%s%s\n\n", colorMagenta, runtime.GOOS, runtime.GOARCH, colorReset)
      return
  }
  ```

  删除后，`printLogo()` 函数体直接从 `logo := []string{...}` 开始，Windows 和 Mac/Linux 统一走同一路径，显示完整 ASCII art + `v1.4.9  windows/amd64`。

- [ ] **Step 2：编译验证**

  ```bash
  cd /Users/yesongyun/代码/dmxapi_claude_code
  go build ./...
  ```
  预期：编译成功，无错误。

- [ ] **Step 3：运行现有测试**

  ```bash
  go test ./... -v
  ```
  预期：所有测试通过。

- [ ] **Step 4：Commit**

  ```bash
  git add dmxapi-claude-code.go
  git commit -m "fix: Windows 统一显示 ASCII art Logo 和版本号"
  ```

---

## Chunk 3：VSCode JSONC 解析修复（TDD）

### Task 4：实现 `stripJSONC` 函数（TDD）

**Files:**
- Modify: `dmxapi-claude-code_test.go`（新增 `TestStripJSONC`）
- Modify: `dmxapi-claude-code.go`（新增 `stripJSONC` + `trailingCommaRe`，修改 `mergeVSCodeSettings`）

- [ ] **Step 1：在测试文件末尾新增失败测试**

  在 `dmxapi-claude-code_test.go` 的最末尾（`TestIsVSCodeConfigured` 函数之后，文件末尾）添加：
  ```go
  func TestStripJSONC(t *testing.T) {
      cases := []struct {
          name  string
          input string
          // 期望：stripJSONC 后的内容能被 json.Unmarshal 解析
          wantKey   string
          wantValue interface{}
      }{
          {
              name:      "尾随逗号",
              input:     `{"editor.fontSize": 14,}`,
              wantKey:   "editor.fontSize",
              wantValue: float64(14),
          },
          {
              name:      "单行注释",
              input:     `{"key": "value" // 这是注释` + "\n}",
              wantKey:   "key",
              wantValue: "value",
          },
          {
              name:      "块注释",
              input:     `{"key": /* 块注释 */ "value"}`,
              wantKey:   "key",
              wantValue: "value",
          },
          {
              name:      "字符串内含双斜杠不误删",
              input:     `{"url": "http://example.com"}`,
              wantKey:   "url",
              wantValue: "http://example.com",
          },
          {
              name:      "字符串内含逗号不误删",
              input:     `{"data": "a,b,c"}`,
              wantKey:   "data",
              wantValue: "a,b,c",
          },
          {
              name:      "尾随逗号在数组内",
              input:     `{"arr": [1, 2, 3,]}`,
              wantKey:   "arr",
              wantValue: []interface{}{float64(1), float64(2), float64(3)},
          },
          {
              name:  "纯净 JSON 不变",
              input: `{"a": 1, "b": "hello"}`,
              wantKey:   "a",
              wantValue: float64(1),
          },
      }
      for _, c := range cases {
          t.Run(c.name, func(t *testing.T) {
              cleaned := stripJSONC([]byte(c.input))
              var result map[string]interface{}
              if err := json.Unmarshal(cleaned, &result); err != nil {
                  t.Fatalf("stripJSONC 后仍无法解析 JSON: %v\n输入: %q\n清理后: %q", err, c.input, string(cleaned))
              }
              got := result[c.wantKey]
              // 对 []interface{} 单独比较
              if wantSlice, ok := c.wantValue.([]interface{}); ok {
                  gotSlice, ok := got.([]interface{})
                  if !ok {
                      t.Fatalf("key %q: 期望 []interface{}, 实际 %T", c.wantKey, got)
                  }
                  if len(gotSlice) != len(wantSlice) {
                      t.Fatalf("key %q 长度: got %d, want %d", c.wantKey, len(gotSlice), len(wantSlice))
                  }
                  for i := range wantSlice {
                      if gotSlice[i] != wantSlice[i] {
                          t.Errorf("key %q[%d]: got %v, want %v", c.wantKey, i, gotSlice[i], wantSlice[i])
                      }
                  }
                  return
              }
              if got != c.wantValue {
                  t.Errorf("key %q: got %v (%T), want %v (%T)", c.wantKey, got, got, c.wantValue, c.wantValue)
              }
          })
      }
  }
  ```

- [ ] **Step 2：运行测试，确认失败（函数不存在）**

  ```bash
  go test ./... -run TestStripJSONC -v
  ```
  预期：编译失败，报错 `undefined: stripJSONC`

- [ ] **Step 3：在 `dmxapi-claude-code.go` 中实现 `stripJSONC`**

  在文件的 `// ==================== URL 处理 ====================` 注释块之前（约第 424 行附近，`ensureScheme` 之前）插入。注意：`bytes` 和 `regexp` 已在文件顶部 import 块中存在，无需新增 import：

  ```go
  // trailingCommaRe 匹配尾随逗号（} 或 ] 前的逗号）
  var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

  // stripJSONC 将 JSONC（VS Code settings.json 格式）转换为合法 JSON。
  // 支持剥离 // 单行注释、/* */ 块注释、以及尾随逗号。
  // 使用字符级状态机正确处理字符串内容（不误删字符串中的 // 或 ,）。
  func stripJSONC(data []byte) []byte {
      var buf bytes.Buffer
      inString := false
      i := 0
      for i < len(data) {
          c := data[i]
          if inString {
              buf.WriteByte(c)
              if c == '\\' && i+1 < len(data) {
                  // 转义字符：原样写入下一个字节
                  i++
                  buf.WriteByte(data[i])
              } else if c == '"' {
                  inString = false
              }
              i++
              continue
          }
          // 以下均为字符串外
          if c == '"' {
              inString = true
              buf.WriteByte(c)
              i++
              continue
          }
          // 单行注释 //
          if c == '/' && i+1 < len(data) && data[i+1] == '/' {
              i += 2
              for i < len(data) && data[i] != '\n' {
                  i++
              }
              continue
          }
          // 块注释 /* ... */
          if c == '/' && i+1 < len(data) && data[i+1] == '*' {
              i += 2
              for i+1 < len(data) {
                  if data[i] == '*' && data[i+1] == '/' {
                      i += 2
                      break
                  }
                  i++
              }
              continue
          }
          buf.WriteByte(c)
          i++
      }
      // 最后剥离尾随逗号
      return trailingCommaRe.ReplaceAll(buf.Bytes(), []byte("$1"))
  }
  ```

- [ ] **Step 4：运行测试，确认通过**

  ```bash
  go test ./... -run TestStripJSONC -v
  ```
  预期：所有 7 个子测试全部 PASS

- [ ] **Step 5：更新 `mergeVSCodeSettings` 使用 `stripJSONC`**

  将约第 748 行的 `mergeVSCodeSettings` 修改为：
  ```go
  func mergeVSCodeSettings(existingJSON []byte, envVars []map[string]string) ([]byte, error) {
      cleaned := stripJSONC(existingJSON)
      var settings map[string]interface{}
      if err := json.Unmarshal(cleaned, &settings); err != nil {
          return nil, fmt.Errorf("解析 settings.json 失败: %v", err)
      }
      settings[vscodeEnvKey] = envVars
      return json.MarshalIndent(settings, "", "  ")
  }
  ```

- [ ] **Step 6：更新 `TestMergeVSCodeSettings` - 新增 JSONC 子测试**

  在 `dmxapi-claude-code_test.go` 的 `TestMergeVSCodeSettings` 函数末尾（`}` 之前）添加：
  ```go
  t.Run("JSONC 尾随逗号", func(t *testing.T) {
      jsonc := []byte(`{
          "editor.fontSize": 14,
          "claudeCode.environmentVariables": [],
      }`)
      out, err := mergeVSCodeSettings(jsonc, envVars)
      if err != nil {
          t.Fatal(err)
      }
      var result map[string]interface{}
      if err := json.Unmarshal(out, &result); err != nil {
          t.Fatal(err)
      }
      if result["editor.fontSize"] != float64(14) {
          t.Error("editor.fontSize should be preserved from JSONC input")
      }
  })

  t.Run("JSONC 单行注释", func(t *testing.T) {
      jsonc := []byte(`{
          // 这是注释
          "editor.fontSize": 14
      }`)
      out, err := mergeVSCodeSettings(jsonc, envVars)
      if err != nil {
          t.Fatal(err)
      }
      var result map[string]interface{}
      if err := json.Unmarshal(out, &result); err != nil {
          t.Fatal(err)
      }
      if result["editor.fontSize"] != float64(14) {
          t.Error("editor.fontSize should be preserved from JSONC with comments")
      }
  })
  ```

- [ ] **Step 7：运行全部测试**

  ```bash
  go test ./... -v
  ```
  预期：全部测试通过，包含新增的 JSONC 子测试

- [ ] **Step 8：Commit**

  ```bash
  git add dmxapi-claude-code.go dmxapi-claude-code_test.go
  git commit -m "feat: 新增 stripJSONC 函数，修复 VSCode JSONC 格式 settings.json 解析失败"
  ```

---

## 验收检查

- [ ] `go test ./... -v` 全部通过
- [ ] `go build ./...` 编译无错误
- [ ] Windows 侧手动测试：install.ps1 中文不乱码
- [ ] Windows Terminal 手动测试：Logo 显示 ASCII art + 版本号
- [ ] L1 模型菜单按 Esc 后不残留菜单内容
- [ ] JSONC settings.json 的 VSCode 配置写入成功
