package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.4.4", "1.4.5", -1},  // 旧版 < 新版
		{"1.4.5", "1.4.4", 1},   // 新版 > 旧版
		{"1.4.4", "1.4.4", 0},   // 相等
		{"1.9.0", "1.10.0", -1}, // 防字符串陷阱：1.9 < 1.10
		{"1.0", "1.0.0", 0},     // 段数不足补0
		{"2.0.0", "1.9.9", 1},   // major 版本比较
		{"bad", "1.0.0", 0},     // 解析失败返回0
		{"1.4.4", "1.4.3", 1},   // 修订版本：新 > 旧
		{"0.0.0", "0.0.0", 0},   // 全零相等
		{"", "1.0.0", 0},        // 空字符串解析失败返回0
		{"1.0.0", "1.0", 0},     // 被比较方段数不足补0
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		if got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVisibleLength(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"hello", 5},                // 普通 ASCII
		{"\033[31mhello\033[0m", 5}, // SGR 颜色序列（原来就支持）
		{"\033[2Khello", 5},         // 清行序列 \033[2K（修复前会多算1）
		{"\033[1;32mOK\033[0m", 2},  // 多参数 SGR
		{"你好", 4},                   // CJK 双宽字符
		{"\033[Ahello\033[B", 5},    // 光标移动序列（\033[A 上移，\033[B 下移）
		{"", 0},                     // 空字符串
	}
	for _, c := range cases {
		got := visibleLength(c.input)
		if got != c.want {
			t.Errorf("visibleLength(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestRuneWidthAmbiguous(t *testing.T) {
	// 保存并在测试后恢复全局 cjkAmbiguous
	orig := cjkAmbiguous
	t.Cleanup(func() { cjkAmbiguous = orig })

	ambiguousRunes := []rune{'◆', '❯', '✔', '✘', '→', '↑', '↓'}

	// 非 CJK locale：ambiguous 字符按 1 宽度
	cjkAmbiguous = false
	for _, r := range ambiguousRunes {
		if got := runeWidth(r); got != 1 {
			t.Errorf("runeWidth(%q) 在 cjkAmbiguous=false 时 = %d，want 1", r, got)
		}
	}

	// CJK locale：ambiguous 字符按 2 宽度
	cjkAmbiguous = true
	for _, r := range ambiguousRunes {
		if got := runeWidth(r); got != 2 {
			t.Errorf("runeWidth(%q) 在 cjkAmbiguous=true 时 = %d，want 2", r, got)
		}
	}

	// 普通 ASCII 不受影响
	if got := runeWidth('a'); got != 1 {
		t.Errorf("ASCII 'a' = %d，want 1", got)
	}
	// 明确双宽 CJK 不受影响
	if got := runeWidth('你'); got != 2 {
		t.Errorf("CJK '你' = %d，want 2", got)
	}
}

func TestDetectCJKLocale(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		lcAll string
		want  bool
	}{
		{"zh_CN 简中", "zh_CN.UTF-8", "", true},
		{"ja_JP 日文", "ja_JP.UTF-8", "", true},
		{"ko_KR 韩文", "ko_KR.UTF-8", "", true},
		{"en_US 英文", "en_US.UTF-8", "", false},
		{"LC_ALL 覆盖", "en_US.UTF-8", "zh_CN.UTF-8", true},
		{"全空", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 清理所有 locale env，避免测试间污染
			t.Setenv("LANG", c.lang)
			t.Setenv("LC_ALL", c.lcAll)
			t.Setenv("LC_CTYPE", "")
			got := detectCJKLocale()
			// 在 Windows 宿主上且所有 env 都空时，可能因 ACP=CJK 返回 true；此用例跳过
			if c.lang == "" && c.lcAll == "" && got != c.want {
				t.Skipf("全空用例依赖 Windows ACP，当前 GOOS=%s；got=%v", runtime.GOOS, got)
			}
			if got != c.want && !(c.lang == "" && c.lcAll == "") {
				t.Errorf("detectCJKLocale() = %v, want %v (LANG=%q LC_ALL=%q)", got, c.want, c.lang, c.lcAll)
			}
		})
	}
}

func TestApplyLegacyTheme(t *testing.T) {
	// 保存所有受影响的全局变量，测试后完整恢复
	origColorReset := colorReset
	origBoxDH := boxDH
	origBoxV := boxV
	origIconPrompt := iconPrompt
	origIconSuccess := iconSuccess
	origSpinner := make([]string, len(spinnerFrames))
	copy(origSpinner, spinnerFrames)
	origSectionStart := sectionStart
	t.Cleanup(func() {
		colorReset = origColorReset
		boxDH = origBoxDH
		boxV = origBoxV
		iconPrompt = origIconPrompt
		iconSuccess = origIconSuccess
		spinnerFrames = origSpinner
		sectionStart = origSectionStart
		// 其他变量依赖可以继续扩展；这里只覆盖断言用到的
	})

	applyLegacyTheme()

	if colorReset != "" {
		t.Errorf("legacy 下 colorReset 应置空，got %q", colorReset)
	}
	if boxDH != "=" {
		t.Errorf("legacy 下 boxDH 应为 '='，got %q", boxDH)
	}
	if boxV != "|" {
		t.Errorf("legacy 下 boxV 应为 '|'，got %q", boxV)
	}
	if iconPrompt != ">" {
		t.Errorf("legacy 下 iconPrompt 应为 '>'，got %q", iconPrompt)
	}
	if iconSuccess != "[OK]" {
		t.Errorf("legacy 下 iconSuccess 应为 '[OK]'，got %q", iconSuccess)
	}
	if sectionStart != "+-" {
		t.Errorf("legacy 下 sectionStart 应为 '+-'，got %q", sectionStart)
	}
	if len(spinnerFrames) != 4 || spinnerFrames[0] != "|" {
		t.Errorf("legacy 下 spinnerFrames 应为 |/-\\，got %q", spinnerFrames)
	}
}

func TestIsWSLFromContent(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"Linux version 5.15.0-microsoft-standard-WSL2", true}, // WSL2 微软内核
		{"Linux version 4.4.0-19041-Microsoft", true},          // WSL1 旧格式
		{"Linux version 5.4.0-generic #1 Ubuntu", false},       // 普通 Linux
		{"Darwin Kernel Version 23.0.0", false},                // macOS
		{"", false},                                            // 空内容
		{"some wsl mention", true},                             // 包含 wsl 关键字
	}
	for _, c := range cases {
		got := wslContentMatches(c.input)
		if got != c.want {
			t.Errorf("wslContentMatches(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestSetxOrRegAdd(t *testing.T) {
	chooseCmd := func(value string) string {
		if len(value) > 900 {
			return "REG_ADD"
		}
		return "SETX"
	}

	shortVal := strings.Repeat("a", 100)
	borderVal := strings.Repeat("a", 900)
	longVal := strings.Repeat("a", 901)

	if chooseCmd(shortVal) != "SETX" {
		t.Errorf("短值（100字节）应使用 SETX")
	}
	if chooseCmd(borderVal) != "SETX" {
		t.Errorf("边界值（900字节）应使用 SETX")
	}
	if chooseCmd(longVal) != "REG_ADD" {
		t.Errorf("超长值（901字节）应使用 REG_ADD")
	}
}

func TestDetectShellProfile(t *testing.T) {
	cases := []struct {
		shellEnv    string
		goos        string
		wantFiles   []string
		wantSrc     string
		wantFish    bool
	}{
		{"/bin/zsh", "darwin", []string{".zshrc", ".zprofile"}, "source ~/.zshrc", false},
		{"/bin/bash", "darwin", []string{".bash_profile"}, "source ~/.bash_profile", false},
		{"/usr/local/bin/fish", "darwin", []string{".config/fish/config.fish"}, "", true},
		{"/bin/zsh", "linux", []string{".zshrc"}, "source ~/.zshrc", false},
		{"/bin/bash", "linux", []string{".bashrc"}, "source ~/.bashrc", false},
		{"/usr/bin/fish", "linux", []string{".config/fish/config.fish"}, "", true},
		{"/opt/homebrew/bin/zsh", "darwin", []string{".zshrc", ".zprofile"}, "source ~/.zshrc", false},
		{"", "darwin", []string{".zshrc", ".zprofile", ".bash_profile"}, "", false},
		{"", "linux", []string{".bashrc", ".profile"}, "", false},
	}

	for _, c := range cases {
		t.Setenv("SHELL", c.shellEnv)
		profile := detectShellProfile(c.goos)
		if len(profile.configFiles) != len(c.wantFiles) {
			t.Fatalf("SHELL=%q GOOS=%q: configFiles=%q, want %q", c.shellEnv, c.goos, profile.configFiles, c.wantFiles)
		}
		for i, wantFile := range c.wantFiles {
			if profile.configFiles[i] != wantFile {
				t.Errorf("SHELL=%q GOOS=%q: configFiles[%d]=%q, want %q", c.shellEnv, c.goos, i, profile.configFiles[i], wantFile)
			}
		}
		if c.shellEnv != "" && profile.sourceCmd != c.wantSrc {
			t.Errorf("SHELL=%q GOOS=%q: sourceCmd=%q, want %q", c.shellEnv, c.goos, profile.sourceCmd, c.wantSrc)
		}
		if profile.isFish != c.wantFish {
			t.Errorf("SHELL=%q GOOS=%q: isFish=%v, want %v", c.shellEnv, c.goos, profile.isFish, c.wantFish)
		}
	}
}

func TestShellLineManagesEnvVar(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		key    string
		isFish bool
		want   bool
	}{
		{"export", "export ANTHROPIC_BASE_URL='https://api.example.com'", envBaseURL, false, true},
		{"leading spaces", "  export ANTHROPIC_BASE_URL=https://api.example.com", envBaseURL, false, true},
		{"declare x", "declare -x ANTHROPIC_AUTH_TOKEN='sk-test'", envAuthToken, false, true},
		{"typeset x", "typeset -x ANTHROPIC_MODEL='claude-sonnet-4-6-cc'", envModel, false, true},
		{"assign and export", "ANTHROPIC_MODEL='claude-sonnet-4-6-cc'; export ANTHROPIC_MODEL", envModel, false, true},
		{"fish set ux", "set -Ux ANTHROPIC_MODEL claude-sonnet-4-6-cc", envModel, true, true},
		{"comment ignored", "# export ANTHROPIC_MODEL=foo", envModel, false, false},
		{"different key", "export OTHER_KEY=1", envModel, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shellLineManagesEnvVar(c.line, c.key, c.isFish)
			if got != c.want {
				t.Fatalf("shellLineManagesEnvVar(%q, %q, %v) = %v, want %v", c.line, c.key, c.isFish, got, c.want)
			}
		})
	}
}

func TestRemoveEnvVarsUnixFromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".zshrc")
	content := strings.Join([]string{
		"export ANTHROPIC_BASE_URL='https://api.example.com'",
		"declare -x ANTHROPIC_AUTH_TOKEN='sk-test'",
		"ANTHROPIC_MODEL='claude-sonnet-4-6-cc'; export ANTHROPIC_MODEL",
		"export KEEP_ME=1",
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeEnvVarsUnixFromFile(configPath, []string{envBaseURL, envAuthToken, envModel}, false)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Fatalf("removed=%d, want 3", removed)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, envBaseURL) || strings.Contains(got, envAuthToken) || strings.Contains(got, envModel) {
		t.Fatalf("managed env vars should be removed, got %q", got)
	}
	if !strings.Contains(got, "export KEEP_ME=1") {
		t.Fatalf("unmanaged line should be preserved, got %q", got)
	}
}

func TestVscodeSettingsPathFor(t *testing.T) {
	cases := []struct {
		goos           string
		homeDir        string
		appData        string
		wslWindowsHome string
		want           string
	}{
		{
			goos:    "darwin",
			homeDir: "/Users/alice",
			want:    "/Users/alice/Library/Application Support/Code/User/settings.json",
		},
		{
			goos:    "linux",
			homeDir: "/home/bob",
			want:    "/home/bob/.config/Code/User/settings.json",
		},
		{
			goos:    "windows",
			appData: `C:\Users\carol\AppData\Roaming`,
			want:    `C:\Users\carol\AppData\Roaming\Code\User\settings.json`,
		},
		{
			goos:           "linux",
			homeDir:        "/home/dave",
			wslWindowsHome: `/mnt/c/Users/dave`,
			want:           `/mnt/c/Users/dave/AppData/Roaming/Code/User/settings.json`,
		},
	}
	for _, c := range cases {
		got := vscodeSettingsPathFor(c.goos, c.homeDir, c.appData, c.wslWindowsHome)
		if got != c.want {
			t.Errorf("vscodeSettingsPathFor(%q,%q,%q,%q)\ngot  %q\nwant %q", c.goos, c.homeDir, c.appData, c.wslWindowsHome, got, c.want)
		}
	}
}

func TestClaudeSettingsPathFor(t *testing.T) {
	got := claudeSettingsPathFor("/Users/alice")
	want := "/Users/alice/.claude/settings.json"
	if got != want {
		t.Errorf("claudeSettingsPathFor() = %q, want %q", got, want)
	}
}

func TestBuildVSCodeEnvVars(t *testing.T) {
	cfg := Config{
		BaseURL:     "https://api.example.com",
		AuthToken:   "sk-test-token",
		Model:       "claude-sonnet-4-6-cc",
		HaikuModel:  "claude-haiku-4-5-20251001-cc",
		SonnetModel: "claude-sonnet-4-6-cc",
		OpusModel:   "claude-opus-4-6-cc",
	}

	vars := buildVSCodeEnvVars(cfg, "")
	if len(vars) != 7 {
		t.Fatalf("expected 7 vars, got %d", len(vars))
	}
	found := false
	for _, v := range vars {
		if v["name"] == "ANTHROPIC_BASE_URL" && v["value"] == "https://api.example.com" {
			found = true
		}
	}
	if !found {
		t.Error("ANTHROPIC_BASE_URL not found or wrong value")
	}
	found = false
	for _, v := range vars {
		if v["name"] == "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS" && v["value"] == "1" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS not found or wrong value")
	}

	vars2 := buildVSCodeEnvVars(cfg, "1")
	if len(vars2) != 8 {
		t.Fatalf("expected 8 vars with agent teams, got %d", len(vars2))
	}
	found = false
	for _, v := range vars2 {
		if v["name"] == "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS" && v["value"] == "1" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS not found")
	}
}

func TestMergeVSCodeSettings(t *testing.T) {
	envVars := []map[string]string{{"name": "ANTHROPIC_BASE_URL", "value": "https://api.example.com"}}

	t.Run("空文件写入", func(t *testing.T) {
		out, err := mergeVSCodeSettings([]byte(`{}`), envVars)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if _, ok := result["claudeCode.environmentVariables"]; !ok {
			t.Error("claudeCode.environmentVariables key missing")
		}
	})

	t.Run("保留既有键", func(t *testing.T) {
		existing := []byte(`{"editor.fontSize": 14, "claudeCode.environmentVariables": []}`)
		out, err := mergeVSCodeSettings(existing, envVars)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if result["editor.fontSize"] != float64(14) {
			t.Error("editor.fontSize should be preserved")
		}
	})

	t.Run("JSON 无效返回错误", func(t *testing.T) {
		_, err := mergeVSCodeSettings([]byte(`not json`), envVars)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

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
}

func TestMergeClaudeSettings(t *testing.T) {
	managed := map[string]string{
		envBaseURL:                  "https://api.example.com",
		envAuthToken:                "sk-test-token",
		envDisableExperimentalBetas: fixedDisableExperimentalBetas,
	}

	t.Run("空文件写入", func(t *testing.T) {
		out, err := mergeClaudeSettings([]byte(`{}`), managed)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		env, ok := result[claudeSettingsEnvKey].(map[string]interface{})
		if !ok {
			t.Fatal("env should be an object")
		}
		if env[envBaseURL] != "https://api.example.com" {
			t.Errorf("expected %s to be written", envBaseURL)
		}
	})

	t.Run("保留其他设置和其他 env 键", func(t *testing.T) {
		existing := []byte(`{
			"permissions": {"allow": ["Read(README.md)"]},
			"env": {
				"FOO": "bar",
				"ANTHROPIC_BASE_URL": "old"
			}
		}`)
		out, err := mergeClaudeSettings(existing, managed)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if _, ok := result["permissions"]; !ok {
			t.Fatal("permissions should be preserved")
		}
		env := result[claudeSettingsEnvKey].(map[string]interface{})
		if env["FOO"] != "bar" {
			t.Error("non-managed env key should be preserved")
		}
		if env[envBaseURL] != "https://api.example.com" {
			t.Error("managed env key should be overwritten")
		}
	})

	t.Run("JSONC 也可解析", func(t *testing.T) {
		jsonc := []byte(`{
			// 注释
			"env": {
				"FOO": "bar",
			},
		}`)
		out, err := mergeClaudeSettings(jsonc, managed)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		env := result[claudeSettingsEnvKey].(map[string]interface{})
		if env["FOO"] != "bar" {
			t.Error("JSONC input should preserve existing env keys")
		}
	})

	t.Run("env 不是对象时报错", func(t *testing.T) {
		_, err := mergeClaudeSettings([]byte(`{"env": []}`), managed)
		if err == nil {
			t.Error("expected error when env is not an object")
		}
	})
}

func TestClearClaudeSettingsManagedKeys(t *testing.T) {
	t.Run("仅删除受管键", func(t *testing.T) {
		existing := []byte(`{
			"env": {
				"FOO": "bar",
				"ANTHROPIC_BASE_URL": "https://example.com",
				"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1"
			},
			"permissions": {"allow": ["Read(README.md)"]}
		}`)
		out, removed, err := clearClaudeSettingsManagedKeys(existing)
		if err != nil {
			t.Fatal(err)
		}
		if !removed {
			t.Fatal("expected managed keys to be removed")
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		env := result[claudeSettingsEnvKey].(map[string]interface{})
		if _, exists := env[envBaseURL]; exists {
			t.Error("managed key should be removed")
		}
		if env["FOO"] != "bar" {
			t.Error("non-managed key should be preserved")
		}
		if _, ok := result["permissions"]; !ok {
			t.Error("other settings should be preserved")
		}
	})

	t.Run("env 清空后删除 env 对象", func(t *testing.T) {
		existing := []byte(`{"env": {"ANTHROPIC_BASE_URL": "https://example.com"}}`)
		out, removed, err := clearClaudeSettingsManagedKeys(existing)
		if err != nil {
			t.Fatal(err)
		}
		if !removed {
			t.Fatal("expected managed key to be removed")
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if _, exists := result[claudeSettingsEnvKey]; exists {
			t.Error("empty env object should be removed")
		}
	})

	t.Run("没有受管键时返回 skipped", func(t *testing.T) {
		out, removed, err := clearClaudeSettingsManagedKeys([]byte(`{"env": {"FOO": "bar"}}`))
		if err != nil {
			t.Fatal(err)
		}
		if removed {
			t.Fatal("expected removed=false")
		}
		if out != nil {
			t.Fatal("expected nil output when nothing removed")
		}
	})
}

func TestRunCommandIncludesCombinedOutput(t *testing.T) {
	err := runCommand("/bin/sh", "-c", "printf 'boom'; printf ' fail' 1>&2; exit 9")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom") || !strings.Contains(msg, "fail") {
		t.Fatalf("expected combined output in error, got %q", msg)
	}
}

func TestSetAndVerifyUserEnvWithOps(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := map[string]string{}
		err := setAndVerifyUserEnvWithOps("ANTHROPIC_MODEL", "claude-sonnet-4-6-cc",
			func(key, value string) error {
				store[key] = value
				return nil
			},
			func(key string) (string, bool, error) {
				v, ok := store[key]
				return v, ok, nil
			},
		)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	t.Run("set failure", func(t *testing.T) {
		err := setAndVerifyUserEnvWithOps("ANTHROPIC_MODEL", "claude-sonnet-4-6-cc",
			func(key, value string) error {
				return fmt.Errorf("access denied")
			},
			func(key string) (string, bool, error) {
				return "", false, nil
			},
		)
		if err == nil || !strings.Contains(err.Error(), "设置环境变量 ANTHROPIC_MODEL 失败") {
			t.Fatalf("expected set failure, got %v", err)
		}
	})

	t.Run("verify missing", func(t *testing.T) {
		err := setAndVerifyUserEnvWithOps("ANTHROPIC_MODEL", "claude-sonnet-4-6-cc",
			func(key, value string) error { return nil },
			func(key string) (string, bool, error) { return "", false, nil },
		)
		if err == nil || !strings.Contains(err.Error(), "变量未写入 Windows 用户环境变量") {
			t.Fatalf("expected verify missing failure, got %v", err)
		}
	})

	t.Run("verify mismatch", func(t *testing.T) {
		err := setAndVerifyUserEnvWithOps("ANTHROPIC_MODEL", "claude-sonnet-4-6-cc",
			func(key, value string) error { return nil },
			func(key string) (string, bool, error) { return "wrong", true, nil },
		)
		if err == nil || !strings.Contains(err.Error(), "期望 \"claude-sonnet-4-6-cc\"，实际 \"wrong\"") {
			t.Fatalf("expected verify mismatch failure, got %v", err)
		}
	})
}

func TestRemoveAndVerifyUserEnvWithOps(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := map[string]string{"ANTHROPIC_MODEL": "claude-sonnet-4-6-cc"}
		err := removeAndVerifyUserEnvWithOps("ANTHROPIC_MODEL",
			func(key string) error {
				delete(store, key)
				return nil
			},
			func(key string) (string, bool, error) {
				v, ok := store[key]
				return v, ok, nil
			},
		)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	// removeFn 报错但变量已不存在 → 目标达成，返回成功
	t.Run("remove failure but already gone", func(t *testing.T) {
		err := removeAndVerifyUserEnvWithOps("ANTHROPIC_MODEL",
			func(key string) error { return fmt.Errorf("registry locked") },
			func(key string) (string, bool, error) { return "", false, nil },
		)
		if err != nil {
			t.Fatalf("expected success (variable already gone), got %v", err)
		}
	})

	// removeFn 报错且变量仍存在 → 返回删除失败
	t.Run("remove failure and still exists", func(t *testing.T) {
		err := removeAndVerifyUserEnvWithOps("ANTHROPIC_MODEL",
			func(key string) error { return fmt.Errorf("registry locked") },
			func(key string) (string, bool, error) { return "claude-sonnet-4-6-cc", true, nil },
		)
		if err == nil || !strings.Contains(err.Error(), "删除 ANTHROPIC_MODEL 失败") {
			t.Fatalf("expected remove failure, got %v", err)
		}
	})

	// removeFn 成功但变量仍存在 → 返回删除失败
	t.Run("remove ok but still exists", func(t *testing.T) {
		err := removeAndVerifyUserEnvWithOps("ANTHROPIC_MODEL",
			func(key string) error { return nil },
			func(key string) (string, bool, error) { return "claude-sonnet-4-6-cc", true, nil },
		)
		if err == nil || !strings.Contains(err.Error(), "删除 ANTHROPIC_MODEL 失败") {
			t.Fatalf("expected still exists failure, got %v", err)
		}
	})
}

func TestParseRegQueryValue(t *testing.T) {
	value, exists, err := parseRegQueryValue("ANTHROPIC_AUTH_TOKEN", []byte("\r\nHKEY_CURRENT_USER\\Environment\r\n    ANTHROPIC_AUTH_TOKEN    REG_SZ    token with spaces\r\n"))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !exists {
		t.Fatal("expected value to exist")
	}
	if value != "token with spaces" {
		t.Fatalf("got %q, want %q", value, "token with spaces")
	}

	_, _, err = parseRegQueryValue("ANTHROPIC_AUTH_TOKEN", []byte("HKEY_CURRENT_USER\\Environment\r\n"))
	if err == nil {
		t.Fatal("expected parse failure")
	}
}

func TestInstallScriptsExposeWindowsCompatibilityFixes(t *testing.T) {
	ps1, err := os.ReadFile("install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ps1Content := string(ps1)
	for _, want := range []string{"-PassThru", "$process.ExitCode", "throw \"Configuration tool failed with exit code $exitCode\""} {
		if !strings.Contains(ps1Content, want) {
			t.Fatalf("install.ps1 missing %q", want)
		}
	}
	if strings.Contains(ps1Content, "\nexit\n") || strings.HasSuffix(ps1Content, "\nexit") {
		t.Fatal("install.ps1 should not end with unconditional exit")
	}

	cmdBytes, err := os.ReadFile("install.cmd")
	if err != nil {
		t.Fatal(err)
	}
	cmdContent := string(cmdBytes)
	for _, want := range []string{"set EXIT_CODE=%ERRORLEVEL%", "endlocal & exit /b %EXIT_CODE%"} {
		if !strings.Contains(cmdContent, want) {
			t.Fatalf("install.cmd missing %q", want)
		}
	}
	if !bytes.Contains(cmdBytes, []byte("\r\n")) {
		t.Fatal("install.cmd should use CRLF line endings")
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "-o \"%TEMP%\\install.cmd\" && call \"%TEMP%\\install.cmd\"") {
		t.Fatal("README Windows CMD example should quote and call install.cmd")
	}
}

func TestStripJSONC(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantKey   string
		wantValue interface{}
	}{
		{"尾随逗号", `{"editor.fontSize": 14,}`, "editor.fontSize", float64(14)},
		{"单行注释", `{"key": "value" // 这是注释` + "\n}", "key", "value"},
		{"块注释", `{"key": /* 块注释 */ "value"}`, "key", "value"},
		{"字符串内含双斜杠不误删", `{"url": "http://example.com"}`, "url", "http://example.com"},
		{"字符串内含逗号不误删", `{"data": "a,b,c"}`, "data", "a,b,c"},
		{"尾随逗号在数组内", `{"arr": [1, 2, 3,]}`, "arr", []interface{}{float64(1), float64(2), float64(3)}},
		{"纯净 JSON 不变", `{"a": 1, "b": "hello"}`, "a", float64(1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cleaned := stripJSONC([]byte(c.input))
			var result map[string]interface{}
			if err := json.Unmarshal(cleaned, &result); err != nil {
				t.Fatalf("stripJSONC 后仍无法解析 JSON: %v\n输入: %q\n清理后: %q", err, c.input, string(cleaned))
			}
			got := result[c.wantKey]
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

func TestIsClaudeSettingsConfigured(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  bool
	}{
		{"含受管键", []byte(`{"env": {"ANTHROPIC_BASE_URL": "https://example.com"}}`), true},
		{"仅含非受管键", []byte(`{"env": {"FOO": "bar"}}`), false},
		{"JSONC", []byte(`{// 注释
"env": {"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",},}`), true},
		{"无效 JSON", []byte(`not json`), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isClaudeSettingsConfigured(c.input)
			if got != c.want {
				t.Errorf("isClaudeSettingsConfigured(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestIsVSCodeConfigured(t *testing.T) {
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
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isVSCodeConfigured(c.input)
			if got != c.want {
				t.Errorf("isVSCodeConfigured(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestClearVSCodeConfig_RemovesKeys(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	content := `{
  "editor.fontSize": 14,
  "claudeCode.environmentVariables": [
    {"name": "ANTHROPIC_BASE_URL", "value": "https://example.com"}
  ],
  "claude-code.environmentVariables": [
    {"name": "ANTHROPIC_BASE_URL", "value": "https://example.com"}
  ]
}`
	if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(settingsPath)
	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		t.Fatal(err)
	}

	delete(settings, vscodeEnvKey)
	delete(settings, vscodeEnvKeyOld)

	output, _ := json.MarshalIndent(settings, "", "  ")
	output = append(output, '\n')
	os.WriteFile(settingsPath, output, 0644)

	result, _ := os.ReadFile(settingsPath)
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)

	if _, exists := parsed[vscodeEnvKey]; exists {
		t.Error("claudeCode.environmentVariables should be removed")
	}
	if _, exists := parsed[vscodeEnvKeyOld]; exists {
		t.Error("claude-code.environmentVariables should be removed")
	}
	if _, exists := parsed["editor.fontSize"]; !exists {
		t.Error("other settings should be preserved")
	}
}

func TestLoadExistingConfig_FallsBackToClaudeSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}

	settingsPath := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := `{
  "env": {
    "ANTHROPIC_BASE_URL": "https://from-settings.example.com",
    "ANTHROPIC_AUTH_TOKEN": "token-from-settings",
    "ANTHROPIC_MODEL": "claude-sonnet-4-6-cc",
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}`
	if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("缺失时从 Claude settings 补齐", func(t *testing.T) {
		t.Setenv(envBaseURL, "")
		t.Setenv(envAuthToken, "")
		t.Setenv(envModel, "")
		t.Setenv(envAgentTeams, "")
		cfg := loadExistingConfig()
		if cfg.BaseURL != "https://from-settings.example.com" {
			t.Errorf("BaseURL fallback failed: %q", cfg.BaseURL)
		}
		if cfg.AuthToken != "token-from-settings" {
			t.Errorf("AuthToken fallback failed: %q", cfg.AuthToken)
		}
		if cfg.Model != "claude-sonnet-4-6-cc" {
			t.Errorf("Model fallback failed: %q", cfg.Model)
		}
		if got := getManagedAgentTeamsValue(); got != "1" {
			t.Errorf("AgentTeams fallback failed: %q", got)
		}
	})

	t.Run("当前环境变量优先", func(t *testing.T) {
		t.Setenv(envBaseURL, "https://from-env.example.com")
		t.Setenv(envAgentTeams, "0")
		cfg := loadExistingConfig()
		if cfg.BaseURL != "https://from-env.example.com" {
			t.Errorf("env value should override settings fallback: %q", cfg.BaseURL)
		}
		if got := getManagedAgentTeamsValue(); got != "0" {
			t.Errorf("AgentTeams env value should override settings fallback: %q", got)
		}
	})
}

func TestAllEnvVarKeys_ContainsAllKnownKeys(t *testing.T) {
	expected := []string{
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS",
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS",
	}

	if len(allEnvVarKeys) != len(expected) {
		t.Errorf("allEnvVarKeys has %d keys, expected %d", len(allEnvVarKeys), len(expected))
	}

	keySet := make(map[string]bool)
	for _, k := range allEnvVarKeys {
		keySet[k] = true
	}
	for _, e := range expected {
		if !keySet[e] {
			t.Errorf("allEnvVarKeys is missing %s", e)
		}
	}
}

func TestWriteFileAtomic(t *testing.T) {
	t.Run("正常写入后原子替换", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.txt")
		if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := writeFileAtomic(path, []byte("new content"), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new content" {
			t.Errorf("want %q, got %q", "new content", string(got))
		}
	})
	t.Run("不残留临时文件", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cfg.txt")
		if err := writeFileAtomic(path, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.Name() != "cfg.txt" {
				t.Errorf("unexpected leftover file: %s", e.Name())
			}
		}
	})
	t.Run("路径含空格", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "path with space")
		if err := os.Mkdir(subdir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(subdir, "cfg.txt")
		if err := writeFileAtomic(path, []byte("ok"), 0644); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "ok" {
			t.Errorf("want %q got %q", "ok", string(got))
		}
	})
}

func TestAllUnixCandidateConfigFiles(t *testing.T) {
	files := allUnixCandidateConfigFiles()
	required := []string{
		".zshrc", ".zshenv", ".zprofile", ".zlogin",
		".bashrc", ".bash_profile", ".bash_login", ".profile",
		".config/fish/config.fish",
		".kshrc", ".cshrc", ".tcshrc",
	}
	set := make(map[string]bool)
	for _, f := range files {
		set[f] = true
	}
	for _, want := range required {
		if !set[want] {
			t.Errorf("allUnixCandidateConfigFiles 缺少 %q", want)
		}
	}
}

func TestRemoveJSONCTopKeys_PreservesComments(t *testing.T) {
	input := []byte(`{
    // VSCode 用户设置
    "editor.fontSize": 14,
    "claudeCode.environmentVariables": [
        {"name": "ANTHROPIC_BASE_URL", "value": "https://example.com"}
    ],
    // 其他配置
    "workbench.colorTheme": "Dark+",
}`)
	out, count := removeJSONCTopKeys(input, []string{"claudeCode.environmentVariables"})
	if count != 1 {
		t.Fatalf("count=%d want 1", count)
	}
	outStr := string(out)
	// 目标键应该消失
	if strings.Contains(outStr, "claudeCode.environmentVariables") {
		t.Error("claudeCode.environmentVariables should be removed")
	}
	// 注释应保留
	if !strings.Contains(outStr, "// VSCode 用户设置") {
		t.Error("`// VSCode 用户设置` 注释应保留")
	}
	if !strings.Contains(outStr, "// 其他配置") {
		t.Error("`// 其他配置` 注释应保留")
	}
	// 其他键应保留
	if !strings.Contains(outStr, `"editor.fontSize": 14`) {
		t.Error("editor.fontSize 应保留")
	}
	if !strings.Contains(outStr, `"workbench.colorTheme"`) {
		t.Error("workbench.colorTheme 应保留")
	}
	// 结果应能被 JSONC 解析（stripJSONC + Unmarshal）
	cleaned := stripJSONC(out)
	var parsed map[string]interface{}
	if err := json.Unmarshal(cleaned, &parsed); err != nil {
		t.Fatalf("结果不是合法 JSONC: %v\n%s", err, outStr)
	}
}

func TestRemoveJSONCNestedKeys_PreservesComments(t *testing.T) {
	input := []byte(`{
    // Claude Code 设置
    "permissions": {"allow": ["Read(README.md)"]},
    "env": {
        "FOO": "bar",
        // managed by dmxapi tool
        "ANTHROPIC_BASE_URL": "https://example.com",
        "ANTHROPIC_AUTH_TOKEN": "sk-test",
        "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1"
    }
}`)
	out, removed, _ := removeJSONCNestedKeys(input, "env", []string{
		"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS",
	})
	if removed != 3 {
		t.Fatalf("removed=%d want 3", removed)
	}
	outStr := string(out)
	if strings.Contains(outStr, "ANTHROPIC_BASE_URL") || strings.Contains(outStr, "ANTHROPIC_AUTH_TOKEN") {
		t.Error("受管键应被删除")
	}
	if !strings.Contains(outStr, `"FOO": "bar"`) {
		t.Error("非受管 env 键 FOO 应保留")
	}
	if !strings.Contains(outStr, "// Claude Code 设置") {
		t.Error("注释应保留")
	}
	// 结果解析为合法 JSONC
	cleaned := stripJSONC(out)
	var parsed map[string]interface{}
	if err := json.Unmarshal(cleaned, &parsed); err != nil {
		t.Fatalf("结果不是合法 JSONC: %v\n%s", err, outStr)
	}
}

func TestRemoveEnvVarsUnixFromFile_NoOpSkipsWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	content := "# user config\nexport PATH=/usr/local/bin:$PATH\nalias ll='ls -la'\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// 故意让文件 mtime 足够旧，好检测是否被重写
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	origMtime := info.ModTime()

	removed, err := removeEnvVarsUnixFromFile(path, []string{envBaseURL, envAuthToken}, false)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("removed=%d want 0", removed)
	}

	info2, _ := os.Stat(path)
	if !info2.ModTime().Equal(origMtime) {
		t.Errorf("无命中行时不应重写文件；mtime 变了：%v → %v", origMtime, info2.ModTime())
	}
	got, _ := os.ReadFile(path)
	if string(got) != content {
		t.Errorf("文件内容被修改；want %q got %q", content, string(got))
	}
}

func TestShellLineManagesEnvVar_FishExtended(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"set -Ux", "set -Ux ANTHROPIC_BASE_URL 'https://x'", true},
		{"set -gx", "set -gx ANTHROPIC_BASE_URL 'https://x'", true},
		{"set -x", "set -x ANTHROPIC_BASE_URL 'https://x'", true},
		{"set -U", "set -U ANTHROPIC_BASE_URL foo", true},
		{"set -Ue only", "set -Ue ANTHROPIC_BASE_URL", true},
		{"set only", "set ANTHROPIC_BASE_URL foo", true},
		{"带尾注释", "set -Ux ANTHROPIC_BASE_URL 'x' # note", true},
		{"不同键", "set -Ux OTHER_KEY 'x'", false},
		{"注释行", "# set -Ux ANTHROPIC_BASE_URL 'x'", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shellLineManagesEnvVar(c.line, envBaseURL, true)
			if got != c.want {
				t.Errorf("shellLineManagesEnvVar(%q, fish) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}

func TestShellLineManagesEnvVar_Csh(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"setenv ANTHROPIC_BASE_URL https://example.com", true},
		{"setenv ANTHROPIC_BASE_URL", true},
		{"unsetenv ANTHROPIC_BASE_URL", true},
		{"setenv OTHER_KEY x", false},
	}
	for _, c := range cases {
		got := shellLineManagesEnvVar(c.line, envBaseURL, false)
		if got != c.want {
			t.Errorf("shellLineManagesEnvVar(%q, csh-mode) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestBroadcastEnvironmentChangeStub(t *testing.T) {
	// 非 Windows 下为 no-op；Windows 下函数存在也可调用（不做实际广播验证）
	broadcastEnvironmentChange()
}
