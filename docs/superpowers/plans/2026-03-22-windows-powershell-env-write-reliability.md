# Windows PowerShell 环境变量写入可靠性修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 Windows PowerShell 下环境变量写入失败且闪退的问题，确保 User 级环境变量写入可验证、失败可见。

**Architecture:** 保持现有 CLI 配置流程不变，只收紧 Windows 持久化写入边界。把 `setEnvVarsWindows` / `removeEnvVarWindows` 改成“写入或删除 → 从同一 User 级持久化来源回读 → 校验”的闭环，并让 `install.ps1` 以 exe 退出码为准暴露失败而不再无条件 `exit`。

**Tech Stack:** Go 1.21+, PowerShell, Windows User environment persistence (`HKCU\Environment` / user-level environment APIs), Go testing package

---

## File Map

- Modify: `dmxapi-claude-code.go`
  - 现有 Windows 环境变量设置入口 `setEnvVarsWindows()`（约 `dmxapi-claude-code.go:611`）
  - 现有 Windows 环境变量删除入口 `removeEnvVarWindows()`（约 `dmxapi-claude-code.go:1241`）
  - 现有批量配置分发入口 `setEnvVars()`（约 `dmxapi-claude-code.go:2143`）
  - 需要新增可测试的 Windows User 环境变量读写抽象与统一验证逻辑
- Modify: `dmxapi-claude-code_test.go`
  - 现有测试文件，继续补充 Windows 写入/删除校验相关测试
- Modify: `install.ps1`
  - 现有 PowerShell 安装入口，需去掉无条件 `exit` 并显式检查 exe 退出码
- Optional Modify: `README.md`
  - 若实现细节改变了 Windows 验证/失败说明，则同步更新文档
- Reference: `docs/superpowers/specs/2026-03-22-windows-powershell-env-write-reliability-design.md`
  - 已批准设计，所有实现必须对齐此文档

---

### Task 1: 为 Windows User 环境变量写入设计可测试边界

**Files:**
- Modify: `dmxapi-claude-code.go:611-650`
- Modify: `dmxapi-claude-code.go:1241-1253`
- Test: `dmxapi-claude-code_test.go`

- [ ] **Step 1: 在测试文件中先写失败用例，描述目标边界**

```go
func TestSetEnvVarsWindows_VerifiesUserEnvAfterWrite(t *testing.T) {
    ops := &fakeWindowsEnvOps{
        getValues: map[string]string{
            envBaseURL: "https://api.example.com",
        },
    }

    err := setEnvVarsWindowsWithOps(map[string]string{
        envBaseURL: "https://api.example.com",
    }, ops)

    if err != nil {
        t.Fatalf("expected success, got %v", err)
    }
    if len(ops.setCalls) != 1 {
        t.Fatalf("expected 1 set call, got %d", len(ops.setCalls))
    }
    if len(ops.getCalls) != 1 {
        t.Fatalf("expected 1 get call, got %d", len(ops.getCalls))
    }
}
```

- [ ] **Step 2: 运行单测确认当前会失败**

Run: `go test ./... -run TestSetEnvVarsWindows_VerifiesUserEnvAfterWrite -v`
Expected: FAIL，提示 `setEnvVarsWindowsWithOps` 或 fake 抽象尚不存在

- [ ] **Step 3: 在主代码中添加最小可测试抽象**

在 `dmxapi-claude-code.go` 中新增聚焦的 Windows User 环境变量接口与默认实现，类似：

```go
type windowsEnvOps interface {
    setUserEnv(name, value string) error
    removeUserEnv(name string) error
    getUserEnv(name string) (string, bool, error)
}

type realWindowsEnvOps struct{}
```

要求：
- 只暴露 `setUserEnv` / `removeUserEnv` / `getUserEnv`
- 后续 `setEnvVarsWindows` 和 `removeEnvVarWindows` 都通过这个边界工作
- 先不要在这一步加入复杂并发，先把抽象立住

- [ ] **Step 4: 运行测试确保新抽象已接通**

Run: `go test ./... -run TestSetEnvVarsWindows_VerifiesUserEnvAfterWrite -v`
Expected: 仍可能 FAIL，但这次应进入真实逻辑缺失阶段，而不是“符号不存在”

- [ ] **Step 5: Commit**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "refactor: add windows env ops abstraction"
```

---

### Task 2: 用 TDD 实现 Windows 写入后回读校验

**Files:**
- Modify: `dmxapi-claude-code.go:611-650`
- Test: `dmxapi-claude-code_test.go`

- [ ] **Step 1: 写 3 个失败测试，覆盖成功、写入失败、回读不一致**

```go
func TestSetEnvVarsWindowsWithOps_SetsAndVerifiesEachKey(t *testing.T) {
    ops := &fakeWindowsEnvOps{
        getValues: map[string]string{
            envBaseURL:   "https://api.example.com",
            envAuthToken: "sk-test",
        },
    }

    err := setEnvVarsWindowsWithOps(map[string]string{
        envBaseURL:   "https://api.example.com",
        envAuthToken: "sk-test",
    }, ops)

    if err != nil {
        t.Fatalf("expected success, got %v", err)
    }
}

func TestSetEnvVarsWindowsWithOps_FailsWhenSetFails(t *testing.T) {
    ops := &fakeWindowsEnvOps{
        setErrs: map[string]error{envBaseURL: errors.New("boom")},
    }

    err := setEnvVarsWindowsWithOps(map[string]string{envBaseURL: "https://api.example.com"}, ops)
    if err == nil || !strings.Contains(err.Error(), envBaseURL) {
        t.Fatalf("expected error mentioning %s, got %v", envBaseURL, err)
    }
}

func TestSetEnvVarsWindowsWithOps_FailsWhenVerifyMismatches(t *testing.T) {
    ops := &fakeWindowsEnvOps{
        getValues: map[string]string{envBaseURL: "https://wrong.example.com"},
    }

    err := setEnvVarsWindowsWithOps(map[string]string{envBaseURL: "https://api.example.com"}, ops)
    if err == nil || !strings.Contains(err.Error(), "verify") {
        t.Fatalf("expected verify error, got %v", err)
    }
}
```

- [ ] **Step 2: 运行这些测试确认失败**

Run: `go test ./... -run 'TestSetEnvVarsWindowsWithOps_' -v`
Expected: FAIL，至少有一个用例失败，说明验证闭环尚未实现

- [ ] **Step 3: 在 `setEnvVarsWindows` 中实现最小闭环逻辑**

实现要求：
- 抽出 `setEnvVarsWindowsWithOps(vars, ops)`，生产代码的 `setEnvVarsWindows` 只负责调用真实实现
- 对每个非空变量依次执行：`setUserEnv` → `getUserEnv`
- 校验回读值与期望值一致
- 任一步失败立刻返回，不允许只收集一个不确定错误
- 错误信息要包含变量名与失败阶段（set / verify）

建议最小实现形态：

```go
func setEnvVarsWindows(vars map[string]string) error {
    return setEnvVarsWindowsWithOps(vars, realWindowsEnvOps{})
}
```

- [ ] **Step 4: 运行目标测试确认通过**

Run: `go test ./... -run 'TestSetEnvVarsWindowsWithOps_' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "fix: verify windows user env writes"
```

---

### Task 3: 用 TDD 实现 Windows 删除后回读校验

**Files:**
- Modify: `dmxapi-claude-code.go:1241-1253`
- Test: `dmxapi-claude-code_test.go`

- [ ] **Step 1: 写失败测试，覆盖删除成功与删除后仍存在**

```go
func TestRemoveEnvVarWindowsWithOps_RemovesAndVerifies(t *testing.T) {
    ops := &fakeWindowsEnvOps{}
    err := removeEnvVarWindowsWithOps(envAgentTeams, ops)
    if err != nil {
        t.Fatalf("expected success, got %v", err)
    }
}

func TestRemoveEnvVarWindowsWithOps_FailsWhenKeyStillExists(t *testing.T) {
    ops := &fakeWindowsEnvOps{
        getValues: map[string]string{envAgentTeams: "1"},
        getExists: map[string]bool{envAgentTeams: true},
    }

    err := removeEnvVarWindowsWithOps(envAgentTeams, ops)
    if err == nil || !strings.Contains(err.Error(), envAgentTeams) {
        t.Fatalf("expected removal verify error, got %v", err)
    }
}
```

- [ ] **Step 2: 运行删除相关测试确认失败**

Run: `go test ./... -run 'TestRemoveEnvVarWindowsWithOps_' -v`
Expected: FAIL

- [ ] **Step 3: 实现删除闭环**

实现要求：
- 抽出 `removeEnvVarWindowsWithOps(key, ops)`
- 调用 `removeUserEnv(key)` 后，立即 `getUserEnv(key)`
- 若变量仍存在，返回明确错误
- 删除逻辑不再保留“尽力而为后只打 warning 继续成功”的路径

最小目标是让删除语义与写入语义一致：都必须可验证。

- [ ] **Step 4: 运行删除测试确认通过**

Run: `go test ./... -run 'TestRemoveEnvVarWindowsWithOps_' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "fix: verify windows user env removal"
```

---

### Task 4: 落实真实 Windows User 级持久化基准

**Files:**
- Modify: `dmxapi-claude-code.go`
- Test: `dmxapi-claude-code_test.go`

- [ ] **Step 1: 先写失败测试，固定真实实现的唯一持久化基准**

不要用 `Skip`。先把“同一 User 级持久化基准”写成会失败的测试或可断言的命令构造测试。

如果实现通过命令行访问 Windows 用户环境变量，则测试至少要能断言：
- `setUserEnv` 写入的是 `HKCU\Environment`（或等价的 User 持久化基准）
- `removeUserEnv` 删除的是同一个 User 持久化基准
- `getUserEnv` 回读的也是同一个 User 持久化基准
- 不允许出现写 `HKCU\Environment`、读 `os.Getenv` / 进程环境变量的实现

建议把真实调用拆成“命令构造”纯函数后先测它，例如：

```go
func TestWindowsUserEnvCommandBuilders_UseSameUserScope(t *testing.T) {
    setName, setArgs := buildWindowsSetUserEnvCommand("ANTHROPIC_BASE_URL", "https://api.example.com")
    getName, getArgs := buildWindowsGetUserEnvCommand("ANTHROPIC_BASE_URL")
    removeName, removeArgs := buildWindowsRemoveUserEnvCommand("ANTHROPIC_BASE_URL")

    if setName != "REG" || getName != "REG" || removeName != "REG" {
        t.Fatal("all windows user env operations should use REG against the same user scope")
    }
    if !strings.Contains(strings.Join(setArgs, " "), `HKCU\Environment`) {
        t.Fatal("set command must target HKCU\\Environment")
    }
    if !strings.Contains(strings.Join(getArgs, " "), `HKCU\Environment`) {
        t.Fatal("get command must target HKCU\\Environment")
    }
    if !strings.Contains(strings.Join(removeArgs, " "), `HKCU\Environment`) {
        t.Fatal("remove command must target HKCU\\Environment")
    }
}
```

- [ ] **Step 2: 运行该测试，确认当前实现未满足这个约束**

Run: `go test ./... -run TestWindowsUserEnvCommandBuilders_UseSameUserScope -v`
Expected: FAIL，提示相关 builder 尚不存在或当前实现仍在使用不统一路径

- [ ] **Step 3: 实现真实 User 级持久化读写，并把基准写死到同一来源**

实现要求：
- `realWindowsEnvOps` 的 `setUserEnv` / `removeUserEnv` / `getUserEnv` 必须全部基于同一个 User 级持久化来源
- 推荐直接固定到 `HKCU\Environment`
- 若抽出命令构造函数，则三者都必须通过这些 builder 生成命令，避免散落实现再次分叉
- `getUserEnv` 必须直接从 User 持久化层读取，而不是读当前进程环境变量
- 长值兼容逻辑若保留，也必须仍然写入并回读同一 User 级来源

- [ ] **Step 4: 运行相关测试和完整测试集确认通过**

Run: `go test ./... -run 'TestWindowsUserEnvCommandBuilders_UseSameUserScope|TestSetEnvVarsWindowsWithOps_|TestRemoveEnvVarWindowsWithOps_' -v`
Expected: PASS

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "refactor: unify windows user env persistence"
```

**Files:**
- Modify: `install.ps1`
- Test: 手工验证（PowerShell）

- [ ] **Step 1: 先写下目标脚本行为（作为注释或计划对照），再最小修改脚本**

脚本最终必须满足：
- 下载失败时明确报错
- exe 启动失败或 exe 非 0 退出时明确报错
- 不再无条件 `exit`
- 失败时通过错误流/异常向 `iex` 调用方暴露失败，而不是直接结束宿主会话

建议目标代码形态：

```powershell
$process = Start-Process -FilePath $tmpFile -NoNewWindow -Wait -PassThru
if ($process.ExitCode -ne 0) {
    Write-Error "Configuration failed with exit code $($process.ExitCode)"
    throw "Configuration failed"
}
```

- [ ] **Step 2: 在本地阅读变更后，用静态检查确认没有保留末尾无条件 `exit`**

Run: `grep` 不可用时，用代码审阅确认 `install.ps1` 不再包含结尾无条件 `exit`，并且读取到了 `ExitCode`
Expected: 脚本包含 `-PassThru` 与退出码检查

- [ ] **Step 3: 如 README 中 Windows 说明不再准确，同步更新最小文案**

例如在常见问题里强调：
- 失败时脚本会直接报错
- 成功后仍需重新打开终端使 User 级变量生效

只在实际文案已失真时修改，避免无关文档 churn。

- [ ] **Step 4: 运行完整测试 + 做手工验证清单**

Run: `go test ./...`
Expected: PASS

Manual verification on Windows PowerShell:
- 成功路径：`iwr -useb .../install.ps1 | iex` 后完成配置，先运行 `[System.Environment]::GetEnvironmentVariable("ANTHROPIC_BASE_URL", "User")` 确认 User 级持久化层已有值
- 成功路径：重新打开 PowerShell 后，`echo $env:ANTHROPIC_BASE_URL` 仍可读到相同值
- 失败路径：人为制造 exe 非 0 退出时，终端应显示明确错误，不再秒退

- [ ] **Step 5: Commit**

```bash
git add install.ps1 README.md dmxapi-claude-code.go dmxapi-claude-code_test.go
git commit -m "fix: surface windows powershell env setup failures"
```

---

## Final Verification Checklist

- [ ] `go test ./...`
- [ ] Windows 写入路径对每个变量执行写后回读校验
- [ ] Windows 删除路径执行删后回读校验
- [ ] User 持久化层可通过 `[System.Environment]::GetEnvironmentVariable(..., "User")` 直接验证
- [ ] `install.ps1` 不再无条件 `exit`
- [ ] `install.ps1` 检查 exe 退出码并清楚暴露失败
- [ ] 如修改 README，文档内容与实现一致

---

## Notes for Executor

- 不要顺手重构 UI、菜单、Logo 或其他与本 bug 无关的区域。
- 不要引入 Machine 级环境变量或管理员权限逻辑。
- 优先让错误信息具体到变量名和失败阶段，避免“配置失败”这类模糊提示。
- 如果发现现有 `runCommand()` 不足以保留错误上下文，可以做最小改造，但不要把命令执行层大范围重写。
