package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
		shellEnv  string
		goos      string
		wantFiles []string
		wantSrc   string
		wantFish  bool
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

func TestApplyModelSuffix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"claude-opus-5-cc", "claude-opus-5-cc[1m]"},     // 预设表内 Ctx1M
		{"claude-opus-5", "claude-opus-5[1m]"},           // 表外家族匹配保持
		{"claude-opus-4-8-cc", "claude-opus-4-8-cc[1m]"}, // 表外家族匹配保持
		{"claude-opus-4-8", "claude-opus-4-8[1m]"},
		{"claude-opus-4-7-cc", "claude-opus-4-7-cc[1m]"},     // 表外家族匹配保持
		{"claude-sonnet-4-6-cc", "claude-sonnet-4-6-cc[1m]"}, // 表外家族匹配保持
		{"claude-opus-5-cc[1m]", "claude-opus-5-cc[1m]"},     // 已有后缀幂等
		{"claude-opus-4-6-cc", "claude-opus-4-6-cc"},         // 不匹配，原样返回
		{"kimi-k3-cc", "kimi-k3-cc[1m]"},                     // 预设表内第三方模型也加后缀
		{"glm-5.3-cc", "glm-5.3-cc[1m]"},
		{"claude-haiku-4-5-20251001-cc", "claude-haiku-4-5-20251001-cc"}, // 表内但 Ctx1M=false，不加后缀
		{"glm-5.1-cc", "glm-5.1-cc"},                                     // 表外第三方模型原样返回
	}
	for _, c := range cases {
		if got := applyModelSuffix(c.in); got != c.want {
			t.Errorf("applyModelSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripModelSuffix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"claude-opus-4-8-cc[1m]", "claude-opus-4-8-cc"}, // 去掉 [1m] 后缀
		{"claude-opus-4-8-cc", "claude-opus-4-8-cc"},     // 无后缀原样返回
		{"glm-5.1-cc", "glm-5.1-cc"},                     // 第三方模型原样返回
		{"", ""},                                         // 空字符串
	}
	for _, c := range cases {
		if got := stripModelSuffix(c.in); got != c.want {
			t.Errorf("stripModelSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildVSCodeEnvVars(t *testing.T) {
	t.Setenv(envEffortLevel, "") // 隔离开发机环境，避免 CLAUDE_CODE_EFFORT_LEVEL 影响结果
	// getManagedEffortLevelValue 在环境变量为空时会回退读取 ~/.claude/settings.json，
	// 故把 HOME 指向空临时目录，防止开发机里的 EffortLevel 多塞一个变量导致计数失败。
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	cfg := Config{
		BaseURL:     "https://api.example.com",
		AuthToken:   "sk-test-token",
		Model:       "claude-sonnet-4-6-cc",
		HaikuModel:  "claude-haiku-4-5-20251001-cc",
		SonnetModel: "claude-sonnet-4-6-cc",
		OpusModel:   "claude-opus-4-6-cc",
	}

	vars := buildVSCodeEnvVars(cfg, "")
	if len(vars) != 8 {
		t.Fatalf("expected 8 vars, got %d", len(vars))
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
	if len(vars2) != 9 {
		t.Fatalf("expected 9 vars with agent teams, got %d", len(vars2))
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

	t.Run("清除历史错误写入的 _NAME 键", func(t *testing.T) {
		existing := []byte(`{
			"env": {
				"FOO": "bar",
				"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "claude-opus-4-8-cc",
				"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME": "claude-opus-4-8-cc",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME": "claude-opus-4-8-cc"
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
		env := result[claudeSettingsEnvKey].(map[string]interface{})
		for _, k := range legacyInvalidEnvKeys {
			if _, ok := env[k]; ok {
				t.Errorf("错误键 %s 应被删除，但仍存在", k)
			}
		}
		if env["FOO"] != "bar" {
			t.Error("非受管 env 键应保留")
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

// TestStripJSONC_PreservesStringWithBraceComma 回归测试：末尾剥离尾随逗号的逻辑此前用不区分
// 字符串边界的正则做最后一遍处理，会把字符串内容里字面出现的 ,} / ,]（如常见的 VSCode
// brace-glob 排除写法）当成结构性尾随逗号误删。修复后剥离逻辑本身具备字符串边界感知。
func TestStripJSONC_PreservesStringWithBraceComma(t *testing.T) {
	input := []byte(`{"files.exclude": {"**/*.{js,}": true}}`)
	cleaned := stripJSONC(input)
	var parsed map[string]interface{}
	if err := json.Unmarshal(cleaned, &parsed); err != nil {
		t.Fatalf("stripJSONC 后无法解析: %v\n%s", err, cleaned)
	}
	excl, ok := parsed["files.exclude"].(map[string]interface{})
	if !ok {
		t.Fatalf("files.exclude 类型不对: %v", parsed["files.exclude"])
	}
	if _, exists := excl["**/*.{js,}"]; !exists {
		t.Errorf("字符串键 \"**/*.{js,}\" 内容被误伤，got=%v", excl)
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
		"ANTHROPIC_DEFAULT_FABLE_MODEL",
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS",
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS",
		"CLAUDE_CODE_EFFORT_LEVEL",
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

// TestRemoveJSONCTopKeys_RemovesDuplicateTopLevelKey 回归测试：外部编辑/合并冲突可能产生同名
// 重复顶层键，Go map 天然去重只反映"存在"不反映"出现几次"；此前只删第一次出现，第二份会残留
// 却被上层判定为"已清除"。修复后应循环删至找不到为止。
func TestRemoveJSONCTopKeys_RemovesDuplicateTopLevelKey(t *testing.T) {
	input := []byte(`{"a":1,"claudeCode.environmentVariables":[1],"b":2,"claudeCode.environmentVariables":[2]}`)
	out, count := removeJSONCTopKeys(input, []string{"claudeCode.environmentVariables"})
	if count != 2 {
		t.Fatalf("count=%d want 2", count)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("结果不是合法 JSON: %v\n%s", err, out)
	}
	if _, exists := parsed["claudeCode.environmentVariables"]; exists {
		t.Errorf("重复顶层键应被全部删除，仍残留: %s", out)
	}
}

// TestRemoveJSONCTopKey_LastKeyPrecededByComment 回归测试：被删的最后一个键前面紧邻的是注释而非
// 逗号时，此前的反向空白扫描找不到逗号，会残留非法尾随逗号 + 悬挂的孤立注释，导致标准 json.Unmarshal
// 解析失败。修复后用正向遍历中记录的 token 级逗号位置定位，不受中间注释影响。
func TestRemoveJSONCTopKey_LastKeyPrecededByComment(t *testing.T) {
	input := []byte("{\n  \"model\": \"sonnet\",\n  // dmxapi 配置\n  \"env\": {\n    \"a\": 1\n  }\n}")
	out, removed := removeJSONCTopKey(input, 0, "env")
	if !removed {
		t.Fatal("expected removed=true")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("结果不是合法 JSON: %v\n%s", err, out)
	}
	if v, ok := parsed["model"]; !ok || v != "sonnet" {
		t.Errorf("model 应保留，got=%s", out)
	}
	if _, exists := parsed["env"]; exists {
		t.Errorf("env 应被删除，got=%s", out)
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
		{"unsetenv ANTHROPIC_BASE_URL ", true},
		{"setenv OTHER_KEY x", false},
		// 回归测试：unsetenv 此前缺少单词边界检查，前缀相同但实际不同名的变量会被误判为受管行
		{"unsetenv ANTHROPIC_BASE_URL_BACKUP", false},
		{"setenv ANTHROPIC_BASE_URL_BACKUP x", false},
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

func TestDmxapiConfigDirFor(t *testing.T) {
	got := dmxapiConfigDirFor("/Users/alice")
	want := filepath.Join("/Users/alice", ".DMXAPI", "claude_code")
	if got != want {
		t.Errorf("dmxapiConfigDirFor() = %q, want %q", got, want)
	}
}

func TestSanitizeConfigFileName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"work", "work"},
		{"  spaced  ", "spaced"},
		{"", "config"},
		{"   ", "config"},
		{"../etc/passwd", "_etc_passwd"},
		{"a/b\\c:d*e?f\"g<h>i|j", "a_b_c_d_e_f_g_h_i_j"},
		{"trail.", "trail"},
		{".lead", "lead"},
		{"con", "_con"},
		{"NUL", "_NUL"},
		{"Com1", "_Com1"},
		{"我的配置", "我的配置"},
	}
	for _, c := range cases {
		if got := sanitizeConfigFileName(c.in); got != c.want {
			t.Errorf("sanitizeConfigFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// 超长按 rune 截断到 80
	long := strings.Repeat("配", 200)
	if got := sanitizeConfigFileName(long); len([]rune(got)) != 80 {
		t.Errorf("sanitizeConfigFileName(long) rune len = %d, want 80", len([]rune(got)))
	}
}

func TestNamedConfigFileName(t *testing.T) {
	if got := namedConfigFileName("work"); got != "work.json" {
		t.Errorf("namedConfigFileName() = %q, want %q", got, "work.json")
	}
	if got := namedConfigFileName("../x"); got != "_x.json" {
		t.Errorf("namedConfigFileName(../x) = %q, want %q", got, "_x.json")
	}
}

func TestMapTopMenuIndex(t *testing.T) {
	cases := []struct {
		idx, n int
		want   topMenuKind
	}{
		// n = 0：1=推荐，2=新增，3=清除
		{1, 0, topRecommended},
		{2, 0, topAdd},
		{3, 0, topClear},
		// n = 2：1=推荐，2/3=命名，4=新增，5=清除
		{1, 2, topRecommended},
		{2, 2, topNamed},
		{3, 2, topNamed},
		{4, 2, topAdd},
		{5, 2, topClear},
	}
	for _, c := range cases {
		if got := mapTopMenuIndex(c.idx, c.n); got != c.want {
			t.Errorf("mapTopMenuIndex(idx=%d, n=%d) = %d, want %d", c.idx, c.n, got, c.want)
		}
	}
}

func TestAdjustL2Window(t *testing.T) {
	// adjustL2Window 是纯函数，total/windowSize 均为入参；这里用合成值 total=25、
	// windowSize=16 覆盖各类边界，与 presetModels 的实际长度无关。
	cases := []struct {
		name                           string
		idx, offset, windowSize, total int
		want                           int
	}{
		// 窗口不小于总数：恒为 0
		{"窗口装得下全部", 5, 3, 25, 25, 0},
		{"窗口大于总数", 5, 3, 30, 25, 0},
		// idx 在窗口内：offset 不动
		{"窗口内不滚动", 8, 5, 16, 25, 5},
		{"窗口首项", 5, 5, 16, 25, 5},
		{"窗口末项", 20, 5, 16, 25, 5},
		// 向下越界：offset = idx - windowSize + 1
		{"向下滚动一格", 21, 5, 16, 25, 6},
		// 向上越界：offset = idx
		{"向上滚动", 4, 5, 16, 25, 4},
		// wrap-around：末项跳到 0 → offset 归 0
		{"下翻越过末项回到首项", 0, 9, 16, 25, 0},
		// wrap-around：首项跳到 total-1 → offset clamp 到 total-windowSize
		{"上翻越过首项跳到末项", 24, 0, 16, 25, 9},
		// 初始居中传入的负 offset 被 clamp 到 0
		{"负偏移钳到 0", 2, -6, 16, 25, 0},
		// 初始居中传入的过大 offset 被 clamp 到 total-windowSize
		{"过大偏移钳到上界", 24, 20, 16, 25, 9},
	}
	for _, c := range cases {
		got := adjustL2Window(c.idx, c.offset, c.windowSize, c.total)
		if got != c.want {
			t.Errorf("%s: adjustL2Window(idx=%d, offset=%d, windowSize=%d, total=%d) = %d, want %d",
				c.name, c.idx, c.offset, c.windowSize, c.total, got, c.want)
		}
		// 不变式：返回的窗口必须让 idx 可见且落在合法范围
		if c.windowSize < c.total {
			if c.idx < got || c.idx >= got+c.windowSize {
				t.Errorf("%s: idx=%d 不在窗口 [%d, %d) 内", c.name, c.idx, got, got+c.windowSize)
			}
			if got < 0 || got > c.total-c.windowSize {
				t.Errorf("%s: offset=%d 超出合法范围 [0, %d]", c.name, got, c.total-c.windowSize)
			}
		}
	}
}

// TestRenderL2MenuLineCount 校验 renderL2Menu 全量/窗口两种模式下：
// 返回的行数与实际打印行数一致（清屏/重绘数学依赖它），且盒子各行右边框对齐。
func TestRenderL2MenuLineCount(t *testing.T) {
	total := len(presetModels) + 1
	cases := []struct {
		name                string
		selectedIdx, offset int
		windowSize          int
		want                int
	}{
		{"全量模式", 0, 0, 0, len(presetModels) + 7},
		{"窗口尺寸不小于总数等同全量", 0, 0, total, len(presetModels) + 7},
		{"窗口顶部", 0, 0, 6, 6 + 8},
		{"窗口中部", 5, 3, 6, 6 + 8},
		{"窗口底部含自定义项", total - 1, total - 6, 6, 6 + 8},
		{"极小窗口", 2, 1, 3, 3 + 8},
	}
	for _, c := range cases {
		var got int
		out := captureStdout(t, func() {
			got = renderL2Menu("Opus 模型", "claude-opus-4-8-cc", c.selectedIdx, c.offset, c.windowSize, 0)
		})
		if got != c.want {
			t.Errorf("%s: renderL2Menu 返回 %d 行, want %d", c.name, got, c.want)
		}
		if printed := strings.Count(out, "\r\n"); printed != got {
			t.Errorf("%s: 实际打印 %d 行, 返回值为 %d", c.name, printed, got)
		}
		widths := map[int]struct{}{}
		for _, raw := range strings.Split(out, "\n") {
			line := stripControl(raw)
			if !strings.HasPrefix(line, boxV) && !strings.HasPrefix(line, boxTL) &&
				!strings.HasPrefix(line, boxML) && !strings.HasPrefix(line, boxBL) {
				continue // 跳过非盒子行（空行、导航提示）
			}
			widths[visibleLength(line)] = struct{}{}
		}
		if len(widths) != 1 {
			t.Errorf("%s: 盒子各行宽度不一致: %v", c.name, widths)
		}
	}
}

func TestNamedConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	nc := NamedConfig{
		Config: Config{
			BaseURL:     "https://www.dmxapi.cn",
			AuthToken:   "sk-test-123",
			Model:       "claude-opus-4-8-cc",
			HaikuModel:  "claude-haiku-4-5-20251001-cc",
			SonnetModel: "claude-sonnet-4-6-cc",
			OpusModel:   "claude-opus-4-8-cc",
		},
		Name:        "我的配置",
		AgentTeams:  "1",
		EffortLevel: "max",
		SavedAt:     "2026-05-29T10:00:00Z",
		AppVersion:  "1.6.5",
	}
	path, err := saveNamedConfigIn(dir, nc)
	if err != nil {
		t.Fatalf("saveNamedConfigIn() error = %v", err)
	}
	if filepath.Base(path) != "我的配置.json" {
		t.Errorf("file base = %q, want %q", filepath.Base(path), "我的配置.json")
	}
	// 权限应为 0600
	if info, err := os.Stat(path); err == nil {
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Errorf("file perm = %v, want 0600", info.Mode().Perm())
		}
	}
	// FilePath 不应出现在序列化结果中
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "FilePath") || strings.Contains(string(raw), "\"-\"") {
		t.Errorf("serialized JSON should not contain FilePath field: %s", raw)
	}

	got, err := readNamedConfig(path)
	if err != nil {
		t.Fatalf("readNamedConfig() error = %v", err)
	}
	if got.FilePath != path {
		t.Errorf("FilePath = %q, want %q", got.FilePath, path)
	}
	got.FilePath = ""
	nc.FilePath = ""
	if got != nc {
		t.Errorf("round trip mismatch:\n got = %+v\nwant = %+v", got, nc)
	}
}

func TestListNamedConfigsIn(t *testing.T) {
	// 目录不存在 → 空切片，无错误
	missing := filepath.Join(t.TempDir(), "nope")
	if got, err := listNamedConfigsIn(missing); err != nil || got != nil {
		t.Errorf("listNamedConfigsIn(missing) = %v, %v; want nil, nil", got, err)
	}

	dir := t.TempDir()
	mustSave := func(name string) {
		if _, err := saveNamedConfigIn(dir, NamedConfig{Name: name, Config: Config{Model: "m-" + name}}); err != nil {
			t.Fatal(err)
		}
	}
	mustSave("zebra")
	mustSave("alpha")
	mustSave("mango")
	// 混入非 .json 与坏 json，应被跳过
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := listNamedConfigsIn(dir)
	if err != nil {
		t.Fatalf("listNamedConfigsIn() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d configs, want 3", len(got))
	}
	// 按 Name 升序
	wantOrder := []string{"alpha", "mango", "zebra"}
	for i, w := range wantOrder {
		if got[i].Name != w {
			t.Errorf("config[%d].Name = %q, want %q", i, got[i].Name, w)
		}
		if got[i].FilePath == "" {
			t.Errorf("config[%d].FilePath not populated", i)
		}
	}
}

func TestDeleteAllNamedConfigsIn(t *testing.T) {
	// 目录不存在 → 0，无错误
	missing := filepath.Join(t.TempDir(), "nope")
	if n, err := deleteAllNamedConfigsIn(missing); err != nil || n != 0 {
		t.Errorf("deleteAllNamedConfigsIn(missing) = %d, %v; want 0, nil", n, err)
	}

	dir := t.TempDir()
	if _, err := saveNamedConfigIn(dir, NamedConfig{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := saveNamedConfigIn(dir, NamedConfig{Name: "b"}); err != nil {
		t.Fatal(err)
	}
	// 非 .json 文件应保留
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	n, err := deleteAllNamedConfigsIn(dir)
	if err != nil {
		t.Fatalf("deleteAllNamedConfigsIn() error = %v", err)
	}
	if n != 2 {
		t.Errorf("deleted %d, want 2", n)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("non-json file should be kept, stat err = %v", err)
	}
	remaining, _ := listNamedConfigsIn(dir)
	if len(remaining) != 0 {
		t.Errorf("remaining configs = %d, want 0", len(remaining))
	}
}

func TestFitWidth(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"}, // 未超宽，原样返回
		{"hello", 5, "hello"},  // 恰好等于上限
		{"hello", 4, "h..."},   // 超宽，省略号
		{"hello", 3, "..."},    // 上限恰为 3，只剩省略号
		{"hello", 2, "he"},     // 上限 <3，硬截断不留省略号
		{"hello", 0, ""},       // 上限 0
		{"hello", -1, ""},      // 负上限
		{"你好世界", 2, "你"},       // CJK：上限 2 容纳一个全宽字符
		{"你好世界", 3, "..."},     // CJK：上限 3 放不下「你」+省略号，只剩省略号
		{"你好世界", 5, "你..."},    // CJK：「你」(2) + "..."(3) = 5
	}
	for _, c := range cases {
		if got := fitWidth(c.in, c.max); got != c.want {
			t.Errorf("fitWidth(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
		if w := visibleLength(fitWidth(c.in, c.max)); c.max > 0 && w > c.max {
			t.Errorf("fitWidth(%q, %d) width = %d exceeds max", c.in, c.max, w)
		}
	}
}

// TestRenderItemMenuAlignment 校验 renderItemMenu 渲染的每一行（含超长项、各选中态、
// CJK / 非 CJK 两种 locale）右边框 │ 始终对齐到同一列。
func TestRenderItemMenuAlignment(t *testing.T) {
	orig := cjkAmbiguous
	t.Cleanup(func() { cjkAmbiguous = orig })

	items := []MenuItem{
		{"1", "dmxapi 推荐配置", "Claude Opus 4.8 一键配置"},
		{"2", "cn站(国产模型使用)", "deepseek-v4-pro-guan-cc | www.dmxapi.cn"},
		{"3", "新增配置", "手动配置 URL / Token / 模型等"},
		{"4", "这是一个非常非常长的自定义配置名称用于测试对齐", "x | www.dmxapi.cn"},
	}
	// 普通 title 与含用户输入的超长 title（如「管理配置「name」」）都要对齐且不 panic。
	titles := []string{
		"请选择配置方式",
		"管理配置「这是一个非常非常非常长的自定义配置名称超过六十列宽度啊啊啊啊」",
	}
	// 两种 locale 都要对齐：iconPrompt 已改为 ASCII ">"（宽度恒 1），
	// 不再受 cjkAmbiguous 影响；保留两分支以防未来回归。
	for _, cjk := range []bool{false, true} {
		cjkAmbiguous = cjk
		for _, title := range titles {
			for sel := range items {
				out := captureStdout(t, func() {
					renderItemMenu(title, items, sel, 0, true)
				})
				widths := map[int]struct{}{}
				for _, raw := range strings.Split(out, "\n") {
					line := stripControl(raw)
					if !strings.HasPrefix(line, boxV) || !strings.HasSuffix(line, boxV) {
						continue // 跳过非盒子行（空行、导航提示）
					}
					widths[visibleLength(line)] = struct{}{}
				}
				if len(widths) != 1 {
					t.Errorf("cjk=%v title=%q sel=%d: 盒子各行宽度不一致: %v", cjk, title, sel, widths)
				}
			}
		}
	}
}

// TestLongTitleNoPanic 校验含用户输入的超长 title（如「管理配置「name」」）在主路径
// 与降级路径（printMenu）下都不会因 negative Repeat 而 panic。
func TestLongTitleNoPanic(t *testing.T) {
	longTitle := "管理配置「这是一个非常非常非常长的自定义配置名称超过六十列宽度啊啊啊啊啊」"
	items := []MenuItem{{"1", "应用此配置", "claude-opus-4-8"}, {"2", "删除", "x"}}
	t.Run("renderItemMenu", func(t *testing.T) {
		_ = captureStdout(t, func() { renderItemMenu(longTitle, items, 0, 0, true) })
	})
	t.Run("printMenu", func(t *testing.T) {
		_ = captureStdout(t, func() { printMenu(longTitle, items) })
	})
	t.Run("printBox", func(t *testing.T) {
		_ = captureStdout(t, func() { printBox(longTitle, colorBrightWhite, []string{"line"}) })
	})
}

// captureStdout 捕获 fn 执行期间写入 os.Stdout 的内容。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		buf.ReadFrom(r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

// stripControl 去除光标移动 / 清行 / SGR 等 CSI 序列与回车，便于按可见宽度比较。
func stripControl(s string) string {
	var b strings.Builder
	inEscape, csiStarted := false, false
	for _, r := range s {
		if r == '\r' {
			continue
		}
		if r == '\033' {
			inEscape, csiStarted = true, false
			continue
		}
		if inEscape {
			if !csiStarted {
				if r == '[' {
					csiStarted = true
				} else {
					inEscape = false
				}
				continue
			}
			if r >= 0x40 && r <= 0x7E {
				inEscape, csiStarted = false, false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ptr 返回指向给定字符串的指针，便于构造 Attribution 三态。
func ptr(s string) *string { return &s }

func TestMergeClaudeAttribution(t *testing.T) {
	t.Run("自定义文本写入顶层", func(t *testing.T) {
		out, err := mergeClaudeAttribution([]byte(`{}`), Attribution{Commit: ptr("by me"), PR: ptr("pr text")})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		attr, ok := result[claudeSettingsAttributionKey].(map[string]interface{})
		if !ok {
			t.Fatal("attribution 应为对象")
		}
		if attr[attributionCommitKey] != "by me" || attr[attributionPRKey] != "pr text" {
			t.Errorf("attribution 内容不符: %v", attr)
		}
	})

	t.Run("空串表示关闭署名", func(t *testing.T) {
		out, err := mergeClaudeAttribution([]byte(`{}`), Attribution{Commit: ptr("")})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		attr := result[claudeSettingsAttributionKey].(map[string]interface{})
		v, exists := attr[attributionCommitKey]
		if !exists {
			t.Fatal("commit 子键应存在（空串）")
		}
		if v != "" {
			t.Errorf("commit 应为空串，得 %v", v)
		}
		if _, exists := attr[attributionPRKey]; exists {
			t.Error("未设置的 pr 子键不应出现")
		}
	})

	t.Run("nil 字段删除子键，全删后移除顶层", func(t *testing.T) {
		existing := []byte(`{"attribution": {"commit": "old", "pr": "oldpr"}}`)
		out, err := mergeClaudeAttribution(existing, Attribution{Commit: nil, PR: nil})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if _, exists := result[claudeSettingsAttributionKey]; exists {
			t.Error("两子键都 nil 时应移除整个 attribution 顶层键")
		}
	})

	t.Run("commit 与 pr 同时写入，不破坏 env 与其他键", func(t *testing.T) {
		existing := []byte(`{
			"env": {"ANTHROPIC_BASE_URL": "https://x"},
			"permissions": {"allow": []},
			"attribution": {"pr": "old-pr"}
		}`)
		// 完整快照语义：传入 commit+pr，两者都落定；env/permissions 不受影响
		out, err := mergeClaudeAttribution(existing, Attribution{Commit: ptr("new-commit"), PR: ptr("keep-pr")})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if _, ok := result["env"]; !ok {
			t.Error("env 应保留")
		}
		if _, ok := result["permissions"]; !ok {
			t.Error("permissions 应保留")
		}
		attr := result[claudeSettingsAttributionKey].(map[string]interface{})
		if attr[attributionCommitKey] != "new-commit" {
			t.Errorf("commit 应被写入，得 %v", attr[attributionCommitKey])
		}
		if attr[attributionPRKey] != "keep-pr" {
			t.Errorf("pr 应被写入，得 %v", attr[attributionPRKey])
		}
	})

	t.Run("nil 字段删除对应子键（完整覆盖语义）", func(t *testing.T) {
		existing := []byte(`{"attribution": {"commit": "old-c", "pr": "old-p"}}`)
		// 只给 commit 指定值，pr=nil → pr 子键应被删除（快照未管理则清除）
		out, err := mergeClaudeAttribution(existing, Attribution{Commit: ptr("new-c"), PR: nil})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		attr := result[claudeSettingsAttributionKey].(map[string]interface{})
		if attr[attributionCommitKey] != "new-c" {
			t.Errorf("commit 应更新为 new-c，得 %v", attr[attributionCommitKey])
		}
		if _, exists := attr[attributionPRKey]; exists {
			t.Error("nil 的 pr 字段应删除对应子键")
		}
	})

	t.Run("保留 attribution 中本工具不管理的子键", func(t *testing.T) {
		existing := []byte(`{"attribution": {"custom": "user-value"}}`)
		out, err := mergeClaudeAttribution(existing, Attribution{Commit: ptr("c")})
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		attr := result[claudeSettingsAttributionKey].(map[string]interface{})
		if attr["custom"] != "user-value" {
			t.Errorf("用户自填子键应保留，得 %v", attr["custom"])
		}
		if attr[attributionCommitKey] != "c" {
			t.Error("commit 应写入")
		}
	})
}

func TestLoadAttributionFromClaudeSettings(t *testing.T) {
	withSettings := func(t *testing.T, content string) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if runtime.GOOS == "windows" {
			t.Setenv("USERPROFILE", home)
		}
		settingsPath := claudeSettingsPathFor(home)
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatal(err)
		}
		if content != "" {
			if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("缺 attribution 顶层返回全 nil", func(t *testing.T) {
		withSettings(t, `{"env": {"ANTHROPIC_BASE_URL": "https://x"}}`)
		attr := loadAttributionFromClaudeSettings()
		if attr.Commit != nil || attr.PR != nil {
			t.Errorf("应全 nil，得 %v", attr)
		}
	})

	t.Run("空串子键读回非 nil 空串", func(t *testing.T) {
		withSettings(t, `{"attribution": {"commit": ""}}`)
		attr := loadAttributionFromClaudeSettings()
		if attr.Commit == nil {
			t.Fatal("commit 应为非 nil")
		}
		if *attr.Commit != "" {
			t.Errorf("commit 应为空串，得 %q", *attr.Commit)
		}
		if attr.PR != nil {
			t.Error("缺失的 pr 应为 nil")
		}
	})

	t.Run("attribution 非对象时容错返回全 nil", func(t *testing.T) {
		withSettings(t, `{"attribution": "oops"}`)
		attr := loadAttributionFromClaudeSettings()
		if attr.Commit != nil || attr.PR != nil {
			t.Errorf("非对象应容错为全 nil，得 %v", attr)
		}
	})
}

func TestClearClaudeSettingsManagedKeys_RemovesAttribution(t *testing.T) {
	t.Run("env 与 attribution 同时清除，保留注释与非受管键", func(t *testing.T) {
		existing := []byte(`{
    // 顶层注释
    "permissions": {"allow": []},
    "env": {
        "FOO": "bar",
        "ANTHROPIC_BASE_URL": "https://x"
    },
    "attribution": {
        "commit": "c",
        "pr": "p"
    }
}`)
		out, removed, err := clearClaudeSettingsManagedKeys(existing)
		if err != nil {
			t.Fatal(err)
		}
		if !removed {
			t.Fatal("应判定为有改动")
		}
		s := string(out)
		if !strings.Contains(s, "// 顶层注释") {
			t.Error("注释应保留")
		}
		cleaned := stripJSONC(out)
		var result map[string]interface{}
		if err := json.Unmarshal(cleaned, &result); err != nil {
			t.Fatalf("结果非法: %v\n%s", err, s)
		}
		if _, ok := result[claudeSettingsAttributionKey]; ok {
			t.Error("attribution 顶层键应被移除")
		}
		env := result[claudeSettingsEnvKey].(map[string]interface{})
		if _, ok := env[envBaseURL]; ok {
			t.Error("受管 env 键应被移除")
		}
		if env["FOO"] != "bar" {
			t.Error("非受管 env 键应保留")
		}
	})

	t.Run("仅有 attribution 无 env 受管键也判定为有改动", func(t *testing.T) {
		existing := []byte(`{"attribution": {"commit": "c"}}`)
		out, removed, err := clearClaudeSettingsManagedKeys(existing)
		if err != nil {
			t.Fatal(err)
		}
		if !removed {
			t.Fatal("仅有 attribution 时也应判定为有改动")
		}
		cleaned := stripJSONC(out)
		var result map[string]interface{}
		if err := json.Unmarshal(cleaned, &result); err != nil {
			t.Fatal(err)
		}
		if _, ok := result[claudeSettingsAttributionKey]; ok {
			t.Error("attribution 应被移除")
		}
	})

	t.Run("attribution 中只删受管子键，保留用户自填子键", func(t *testing.T) {
		existing := []byte(`{"attribution": {"commit": "c", "custom": "keep"}}`)
		out, removed, err := clearClaudeSettingsManagedKeys(existing)
		if err != nil {
			t.Fatal(err)
		}
		if !removed {
			t.Fatal("应有改动")
		}
		cleaned := stripJSONC(out)
		var result map[string]interface{}
		if err := json.Unmarshal(cleaned, &result); err != nil {
			t.Fatal(err)
		}
		attr, ok := result[claudeSettingsAttributionKey].(map[string]interface{})
		if !ok {
			t.Fatal("仍有用户子键时 attribution 顶层应保留")
		}
		if attr["custom"] != "keep" {
			t.Error("用户自填子键应保留")
		}
		if _, ok := attr[attributionCommitKey]; ok {
			t.Error("受管 commit 子键应被删除")
		}
	})
}

func TestNamedConfigAttributionRoundTrip(t *testing.T) {
	t.Run("nil Attribution 序列化时省略字段", func(t *testing.T) {
		nc := NamedConfig{Name: "x", Config: Config{BaseURL: "https://x"}}
		data, err := json.Marshal(nc)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "attribution") {
			t.Errorf("nil Attribution 不应序列化 attribution 字段: %s", data)
		}
	})

	t.Run("空串子键往返保持非 nil 空串", func(t *testing.T) {
		nc := NamedConfig{Name: "x", Attribution: &Attribution{Commit: ptr("")}}
		data, err := json.Marshal(nc)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"commit":""`) {
			t.Errorf(`应序列化为 "commit":""，得 %s`, data)
		}
		var back NamedConfig
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if back.Attribution == nil || back.Attribution.Commit == nil {
			t.Fatal("往返后 Commit 应为非 nil")
		}
		if *back.Attribution.Commit != "" {
			t.Errorf("往返后应为空串，得 %q", *back.Attribution.Commit)
		}
	})
}

func TestSaveClaudeSettingsConfigWithAgentTeams_PreservesAttribution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	// 清空可能影响 buildManagedEnvMap 的开关
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "")

	settingsPath := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	// 预置一个含 attribution 的 settings.json
	content := `{"attribution": {"commit": "preset", "pr": "preset-pr"}}`
	if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 写 env 受管键，不应触碰 attribution（内部回读 getManagedAttribution 原样写回）
	cfg := Config{BaseURL: "https://x", AuthToken: "sk-1", Model: "m"}
	if err := saveClaudeSettingsConfigWithAgentTeams(cfg, ""); err != nil {
		t.Fatal(err)
	}

	attr := loadAttributionFromClaudeSettings()
	if attr.Commit == nil || *attr.Commit != "preset" {
		t.Errorf("commit 署名应原样保留，得 %v", attr.Commit)
	}
	if attr.PR == nil || *attr.PR != "preset-pr" {
		t.Errorf("pr 署名应原样保留，得 %v", attr.PR)
	}
	// 同时确认 env 受管键确实写入了
	loaded := loadConfigFromClaudeSettings()
	if loaded.BaseURL != "https://x" {
		t.Errorf("env 受管键应写入，得 BaseURL=%q", loaded.BaseURL)
	}
}

func TestDmxapiDefaultAttribution(t *testing.T) {
	attr := dmxapiDefaultAttribution()
	if attr.Commit == nil || attr.PR == nil {
		t.Fatal("commit 与 pr 都应为非 nil")
	}
	if *attr.Commit != recommendedAttributionText {
		t.Errorf("commit 应为 %q，得 %q", recommendedAttributionText, *attr.Commit)
	}
	if *attr.PR != recommendedAttributionText {
		t.Errorf("pr 应为 %q，得 %q", recommendedAttributionText, *attr.PR)
	}
	// 两指针独立：改 commit 不应影响 pr
	*attr.Commit = "changed"
	if *attr.PR == "changed" {
		t.Error("commit 与 pr 应使用各自独立的指针")
	}
}

func TestDmxapiDefaultAttribution_EndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "")

	sp := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sp, []byte(`{"permissions": {"allow": []}}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{BaseURL: "https://www.dmxapi.cn", AuthToken: "sk-x", Model: "m"}
	if err := saveClaudeSettingsConfigWithAttribution(cfg, "", dmxapiDefaultAttribution()); err != nil {
		t.Fatal(err)
	}

	attr := loadAttributionFromClaudeSettings()
	if attr.Commit == nil || *attr.Commit != recommendedAttributionText {
		t.Errorf("commit 署名应为 %q，得 %v", recommendedAttributionText, attr.Commit)
	}
	if attr.PR == nil || *attr.PR != recommendedAttributionText {
		t.Errorf("pr 署名应为 %q，得 %v", recommendedAttributionText, attr.PR)
	}
	// env 受管键也应写入，permissions 保留
	if loadConfigFromClaudeSettings().BaseURL != "https://www.dmxapi.cn" {
		t.Error("env BaseURL 未写入")
	}
}

// ── Effort Level 顶层 effortLevel 同步与归一 ─────────────────────────────────

func TestNormalizeEffortLevel(t *testing.T) {
	cases := map[string]string{
		"ultracode": "xhigh", // 历史非法值归一
		"low":       "low",
		"medium":    "medium",
		"high":      "high",
		"xhigh":     "xhigh",
		"max":       "max", // env 合法、顶层不接受，但归一阶段不动它
		"auto":      "auto",
		"":          "",
	}
	for in, want := range cases {
		if got := normalizeEffortLevel(in); got != want {
			t.Errorf("normalizeEffortLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMergeClaudeEffortLevel(t *testing.T) {
	base := []byte(`{"model":"sonnet"}`)

	// 每个用例用独立 map：json.Unmarshal 对复用 map 是合并语义，不会清除上一轮的键。
	hasTop := func(b []byte) (string, bool) {
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		v, ok := m["effortLevel"]
		s, _ := v.(string)
		return s, ok
	}

	// 合法值 → 写顶层
	out, err := mergeClaudeEffortLevel(base, "xhigh")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := hasTop(out); v != "xhigh" {
		t.Errorf("xhigh 应写入顶层 effortLevel，得 %v", v)
	}

	// 入参 ultracode → 归一为 xhigh 写入
	out, _ = mergeClaudeEffortLevel(base, "ultracode")
	if v, _ := hasTop(out); v != "xhigh" {
		t.Errorf("ultracode 应归一为 xhigh，得 %v", v)
	}

	// 空 → 删除顶层
	withTop := []byte(`{"model":"sonnet","effortLevel":"high"}`)
	out, _ = mergeClaudeEffortLevel(withTop, "")
	if _, ok := hasTop(out); ok {
		t.Error("空值应删除顶层 effortLevel，仍存在")
	}

	// max（已加入 validTopEffortLevels）→ 覆盖现有值写入顶层
	out, _ = mergeClaudeEffortLevel(withTop, "max")
	if v, _ := hasTop(out); v != "max" {
		t.Errorf("max 应写入顶层 effortLevel，得 %v", v)
	}
}

func TestLoadConfigFromClaudeSettings_TopLevelEffort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	sp := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		t.Fatal(err)
	}

	// 有 env 块 + 顶层 effortLevel
	if err := os.WriteFile(sp, []byte(`{"effortLevel":"xhigh","env":{"CLAUDE_CODE_EFFORT_LEVEL":"high"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	loaded := loadConfigFromClaudeSettings()
	if loaded.EffortLevel != "high" {
		t.Errorf("env 块 EffortLevel = %q, want high", loaded.EffortLevel)
	}
	if loaded.TopLevelEffortLevel != "xhigh" {
		t.Errorf("TopLevelEffortLevel = %q, want xhigh", loaded.TopLevelEffortLevel)
	}

	// 无 env 块、仅顶层 → 仍能读出顶层（early-return 移出验证）
	if err := os.WriteFile(sp, []byte(`{"effortLevel":"xhigh"}`), 0644); err != nil {
		t.Fatal(err)
	}
	loaded = loadConfigFromClaudeSettings()
	if loaded.TopLevelEffortLevel != "xhigh" {
		t.Errorf("无 env 块时 TopLevelEffortLevel = %q, want xhigh", loaded.TopLevelEffortLevel)
	}
}

func TestGetManagedEffortLevelValue_FallbackAndNormalize(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	sp := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		t.Fatal(err)
	}

	// 系统 env 优先，且归一 ultracode→xhigh
	t.Setenv(envEffortLevel, "ultracode")
	if got := getManagedEffortLevelValue(); got != "xhigh" {
		t.Errorf("系统 env=ultracode 应归一为 xhigh，得 %q", got)
	}

	// env 空 → 回退 env 块
	t.Setenv(envEffortLevel, "")
	if err := os.WriteFile(sp, []byte(`{"env":{"CLAUDE_CODE_EFFORT_LEVEL":"high"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := getManagedEffortLevelValue(); got != "high" {
		t.Errorf("应回退 env 块 high，得 %q", got)
	}

	// env 空、env 块无 → 回退顶层 effortLevel
	if err := os.WriteFile(sp, []byte(`{"effortLevel":"xhigh"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := getManagedEffortLevelValue(); got != "xhigh" {
		t.Errorf("应回退顶层 effortLevel xhigh，得 %q", got)
	}
}

func TestSaveClaudeSettings_TopAndEnvEffortConsistent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "xhigh")

	sp := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		t.Fatal(err)
	}
	// 预置一个含旧顶层 high 的文件，验证写入后被同步为 xhigh
	if err := os.WriteFile(sp, []byte(`{"effortLevel":"high","model":"sonnet"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{BaseURL: "https://x", AuthToken: "sk-1", Model: "m"}
	if err := saveClaudeSettingsConfigWithAgentTeams(cfg, ""); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(sp)
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["effortLevel"] != "xhigh" {
		t.Errorf("顶层 effortLevel = %v, want xhigh", m["effortLevel"])
	}
	envMap, _ := m["env"].(map[string]interface{})
	if envMap["CLAUDE_CODE_EFFORT_LEVEL"] != "xhigh" {
		t.Errorf("env 块 effort = %v, want xhigh", envMap["CLAUDE_CODE_EFFORT_LEVEL"])
	}
	if m["model"] != "sonnet" {
		t.Errorf("无关键 model 应保留，得 %v", m["model"])
	}
}

// TestSaveClaudeSettings_MigratesLegacyUltracode 精确复现用户报告的真实文件状态：
// env 块写着历史非法值 ultracode、顶层残留旧 high、系统 env 也是 ultracode。
// 走一次保存后，env 块与顶层应同时归一为 xhigh，消除“两处不一致”。
func TestSaveClaudeSettings_MigratesLegacyUltracode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "ultracode") // 系统 env 残留非法值

	sp := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		t.Fatal(err)
	}
	// 复现：env 块 ultracode + 顶层 high（两处不一致）
	preset := `{"effortLevel":"high","env":{"CLAUDE_CODE_EFFORT_LEVEL":"ultracode"}}`
	if err := os.WriteFile(sp, []byte(preset), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{BaseURL: "https://x", AuthToken: "sk-1", Model: "m"}
	if err := saveClaudeSettingsConfigWithAgentTeams(cfg, ""); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(sp)
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	env, _ := m["env"].(map[string]interface{})
	if m["effortLevel"] != "xhigh" || env["CLAUDE_CODE_EFFORT_LEVEL"] != "xhigh" {
		t.Errorf("迁移后两处应均为 xhigh，得 顶层=%v env=%v", m["effortLevel"], env["CLAUDE_CODE_EFFORT_LEVEL"])
	}
}

func TestClearEffortFromClaudeSettings_Combinations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv(envEffortLevel, "")
	sp := claudeSettingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		t.Fatal(err)
	}

	readTop := func() (map[string]interface{}, map[string]interface{}) {
		raw, _ := os.ReadFile(sp)
		var m map[string]interface{}
		_ = json.Unmarshal(raw, &m)
		env, _ := m["env"].(map[string]interface{})
		return m, env
	}

	// env 有 + 顶层有 → 两处都删
	_ = os.WriteFile(sp, []byte(`{"effortLevel":"xhigh","env":{"CLAUDE_CODE_EFFORT_LEVEL":"xhigh","OTHER":"keep"}}`), 0644)
	if err := clearEffortFromClaudeSettings(); err != nil {
		t.Fatal(err)
	}
	m, env := readTop()
	if _, ok := m["effortLevel"]; ok {
		t.Error("env有+顶层有：顶层 effortLevel 应删除")
	}
	if _, ok := env["CLAUDE_CODE_EFFORT_LEVEL"]; ok {
		t.Error("env有+顶层有：env effort 应删除")
	}
	if env["OTHER"] != "keep" {
		t.Error("env有+顶层有：其他 env 键应保留")
	}

	// env 无 + 顶层有 → 必须删顶层
	_ = os.WriteFile(sp, []byte(`{"effortLevel":"xhigh","model":"sonnet"}`), 0644)
	if err := clearEffortFromClaudeSettings(); err != nil {
		t.Fatal(err)
	}
	m, _ = readTop()
	if _, ok := m["effortLevel"]; ok {
		t.Error("env无+顶层有：顶层 effortLevel 必须删除")
	}
	if m["model"] != "sonnet" {
		t.Error("env无+顶层有：无关键应保留")
	}

	// env 无 + 顶层无 → 幂等不改文件
	content := []byte(`{"model":"sonnet"}` + "\n")
	_ = os.WriteFile(sp, content, 0644)
	if err := clearEffortFromClaudeSettings(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(sp)
	if !bytes.Equal(raw, content) {
		t.Errorf("env无+顶层无：文件应原样不动\n得 %q\n原 %q", raw, content)
	}
}

func TestClearClaudeSettingsManagedKeys_DropsTopEffortOnEnvHit(t *testing.T) {
	// envHit：本工具配过 env → 连顶层 effortLevel 一起清
	withEnv := []byte(`{"effortLevel":"xhigh","env":{"ANTHROPIC_BASE_URL":"https://x","CLAUDE_CODE_EFFORT_LEVEL":"xhigh"}}`)
	out, removed, err := clearClaudeSettingsManagedKeys(withEnv)
	if err != nil || !removed {
		t.Fatalf("envHit 应判定有受管键，err=%v removed=%v", err, removed)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(out, &m)
	if _, ok := m["effortLevel"]; ok {
		t.Error("envHit：顶层 effortLevel 应连带清除")
	}

	// 仅顶层 effortLevel、无 env 受管键 → 不命中、不删（视为用户 /effort 自设）
	onlyTop := []byte(`{"effortLevel":"xhigh"}`)
	_, removed, _ = clearClaudeSettingsManagedKeys(onlyTop)
	if removed {
		t.Error("仅顶层 effortLevel 不应被判为受管配置（避免误删用户自设）")
	}
}

// TestClearClaudeSettingsManagedKeys_KeepsTopEffortWhenEnvHitIsUnrelated 回归测试：envHit
// 此前的判定粒度是"env 块里命中任意一个受管键即真"，与顶层 effortLevel 是否真的来自本工具
// 无关。用户只用本工具配过 baseURL/authToken（env 里没有 CLAUDE_CODE_EFFORT_LEVEL），又单独
// 用 /effort 命令设置了顶层值时，清除配置不应连带清掉这个跟本工具无关的顶层 effortLevel。
func TestClearClaudeSettingsManagedKeys_KeepsTopEffortWhenEnvHitIsUnrelated(t *testing.T) {
	input := []byte(`{"effortLevel":"medium","env":{"ANTHROPIC_BASE_URL":"https://x","ANTHROPIC_AUTH_TOKEN":"sk-1"}}`)
	out, removed, err := clearClaudeSettingsManagedKeys(input)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("env 里的 baseURL/authToken 命中，removed 应为 true")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("结果不是合法 JSON: %v\n%s", err, out)
	}
	if v, ok := m["effortLevel"]; !ok || v != "medium" {
		t.Errorf("顶层 effortLevel 与本工具受管 env 键无关，应保留用户 /effort 自设值，got=%v", m["effortLevel"])
	}
	if _, exists := m["env"]; exists {
		t.Errorf("env 块应被清空后整体移除，got=%s", out)
	}
}

func TestApplyNamedConfig_LegacyUltracodeNormalized(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "")
	// 隔离系统环境变量写入路径（避免污染开发机 shell profile）
	t.Setenv("SHELL", "/bin/bash")

	nc := NamedConfig{
		Config:      Config{BaseURL: "https://www.dmxapi.cn", AuthToken: "sk-1", Model: "m"},
		Name:        "legacy",
		EffortLevel: "ultracode", // 老快照里的历史非法值
	}
	if err := applyNamedConfig(nc); err != nil {
		t.Fatalf("applyNamedConfig() error = %v", err)
	}

	// 进程 env 应被归一为 xhigh（不是 ultracode）
	if got := os.Getenv(envEffortLevel); got != "xhigh" {
		t.Errorf("进程 env effort = %q, want xhigh（归一）", got)
	}
	// settings.json 顶层与 env 块都应是 xhigh
	raw, _ := os.ReadFile(claudeSettingsPathFor(home))
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	if m["effortLevel"] != "xhigh" {
		t.Errorf("顶层 effortLevel = %v, want xhigh", m["effortLevel"])
	}
	if env, _ := m["env"].(map[string]interface{}); env["CLAUDE_CODE_EFFORT_LEVEL"] != "xhigh" {
		t.Errorf("env 块 effort = %v, want xhigh", env["CLAUDE_CODE_EFFORT_LEVEL"])
	}
}

// ==================== 外部命令超时 / spinner 兜底超时 ====================

func TestExecCombinedWithTimeoutHangingCommand(t *testing.T) {
	start := time.Now()
	_, err := execCombinedWithTimeout(300*time.Millisecond, "/bin/sh", "-c", "sleep 30")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for hanging command")
	}
	if !errors.Is(err, errCommandTimeout) {
		t.Fatalf("expected errCommandTimeout, got %v", err)
	}
	// 关键回归点：挂起的命令必须在超时附近返回，而不是无限阻塞
	if elapsed > 5*time.Second {
		t.Fatalf("execCombinedWithTimeout blocked for %v, expected ~300ms", elapsed)
	}
}

// TestExecCombinedWithTimeoutOrphanKeepsPipe 覆盖 WaitDelay 的存在理由：
// 子进程已退出，但它派生的孙进程仍继承着 stdout 管道。没有 WaitDelay 时
// CombinedOutput 会一直读到管道 EOF（即孙进程 30 秒后结束）才返回；
// 有 WaitDelay 时最多多等 commandWaitDelay 就关掉管道返回，且不算作失败。
func TestExecCombinedWithTimeoutOrphanKeepsPipe(t *testing.T) {
	// 上界取 sleep 时长的一半：只要没退化成"等孙进程 30 秒"，这个测试就成立；
	// 同时远大于 commandWaitDelay，避免在过载机器上因调度抖动 flaky。
	timeout := 15 * time.Second
	start := time.Now()
	_, err := execCombinedWithTimeout(timeout, "/bin/sh", "-c", "sleep 30 & exit 0")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("命令自身已正常退出，ErrWaitDelay 不该被当成失败，got %v", err)
	}
	if elapsed >= timeout {
		t.Fatalf("execCombinedWithTimeout blocked for %v waiting on an orphan's pipe", elapsed)
	}
}

func TestExecCombinedWithTimeoutSuccess(t *testing.T) {
	out, err := execCombinedWithTimeout(5*time.Second, "/bin/sh", "-c", "printf 'hello'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "hello" {
		t.Fatalf("output = %q, want %q", string(out), "hello")
	}
}

func TestExecOutputWithTimeoutExcludesStderr(t *testing.T) {
	out, err := execOutputWithTimeout(5*time.Second, "/bin/sh", "-c", "printf 'C:\\Users\\me'; printf 'UNC warning' 1>&2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "UNC warning") {
		t.Fatalf("stdout-only variant leaked stderr: %q", string(out))
	}
}

// TestClassifyRegQueryResult 覆盖 Windows 专属陷阱：TerminateProcess(handle, 1) 让被超时
// 杀掉的子进程退出码恰好是 1，与 REG QUERY 表示"变量不存在"的退出码相同。必须先判超时。
// 这个撞车在 Unix 上复现不出来（那里 Kill 后 ExitCode 是 -1），所以只能靠这个平台无关的纯函数测。
func TestClassifyRegQueryResult(t *testing.T) {
	exitOne := exitErrWithCode(t, 1)

	t.Run("退出码1且未超时=变量不存在", func(t *testing.T) {
		val, exists, err := classifyRegQueryResult("ANTHROPIC_MODEL", nil, exitOne)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if exists || val != "" {
			t.Fatalf("expected not-exists, got val=%q exists=%v", val, exists)
		}
	})

	t.Run("退出码1但已超时=超时错误而非变量不存在", func(t *testing.T) {
		wrapped := fmt.Errorf("执行 REG 超过 30s 仍未返回: %w", errCommandTimeout)
		_, exists, err := classifyRegQueryResult("ANTHROPIC_MODEL", nil, wrapped)
		if err == nil {
			t.Fatal("超时必须报错，不能被当成'变量不存在'——否则用户会拿到一条完全错误的诊断")
		}
		if !errors.Is(err, errCommandTimeout) {
			t.Fatalf("expected errCommandTimeout, got %v", err)
		}
		if exists {
			t.Fatal("expected exists=false on timeout")
		}
	})

	t.Run("成功时解析值", func(t *testing.T) {
		output := []byte("\r\nHKEY_CURRENT_USER\\Environment\r\n    ANTHROPIC_MODEL    REG_SZ    claude-opus-5\r\n\r\n")
		val, exists, err := classifyRegQueryResult("ANTHROPIC_MODEL", output, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists || val != "claude-opus-5" {
			t.Fatalf("got val=%q exists=%v, want claude-opus-5/true", val, exists)
		}
	})

	t.Run("其他错误原样透出并附带输出", func(t *testing.T) {
		_, _, err := classifyRegQueryResult("ANTHROPIC_MODEL", []byte("ACCESS DENIED"), exitErrWithCode(t, 5))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "ACCESS DENIED") {
			t.Fatalf("expected command output in error, got %q", err.Error())
		}
	})
}

// exitErrWithCode 构造一个真实的 *exec.ExitError，其退出码为 code。
func exitErrWithCode(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("/bin/sh", "-c", fmt.Sprintf("exit %d", code)).Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %v", err)
	}
	if exitErr.ExitCode() != code {
		t.Fatalf("exit code = %d, want %d", exitErr.ExitCode(), code)
	}
	return err
}

func TestRunWithSpinnerTimeoutHangingTask(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	var err error
	start := time.Now()
	_ = captureStdout(t, func() {
		err = runWithSpinnerTimeout("正在保存配置...", 300*time.Millisecond, func() error {
			<-release // 永不主动返回，模拟被外部命令卡死的保存
			return nil
		})
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, errSpinnerTimeout) {
		t.Fatalf("expected errSpinnerTimeout, got %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("runWithSpinnerTimeout blocked for %v, expected ~300ms", elapsed)
	}
	// 用户必须能从错误里看出是哪一步超时
	if !strings.Contains(err.Error(), "正在保存配置") {
		t.Fatalf("timeout error should name the step, got %q", err.Error())
	}
}

func TestRunWithSpinnerTimeoutPassesThrough(t *testing.T) {
	sentinel := errors.New("boom")

	t.Run("透传 task 的错误", func(t *testing.T) {
		var err error
		_ = captureStdout(t, func() {
			err = runWithSpinnerTimeout("正在保存配置...", 5*time.Second, func() error { return sentinel })
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("expected sentinel error, got %v", err)
		}
	})

	t.Run("成功时返回 nil", func(t *testing.T) {
		var err error
		_ = captureStdout(t, func() {
			err = runWithSpinnerTimeout("正在保存配置...", 5*time.Second, func() error { return nil })
		})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
}

// TestRunWithSpinnerTimeoutRecoversPanic 防回归：原实现下 task panic 会跳过 done 发送，
// 动画协程一路画到进程崩溃。
func TestRunWithSpinnerTimeoutRecoversPanic(t *testing.T) {
	var err error
	_ = captureStdout(t, func() {
		err = runWithSpinnerTimeout("正在保存配置...", 5*time.Second, func() error {
			panic("kaboom")
		})
	})
	if err == nil {
		t.Fatal("expected error from panicking task")
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected panic value in error, got %q", err.Error())
	}
}

// TestRunWithSpinnerTimeoutClearsLine 防回归：动画协程必须在主协程清行之前真正退出，
// 否则残帧会被打印在清行之后，屏幕上留下一行不再转动的"正在保存配置..."。
func TestRunWithSpinnerTimeoutClearsLine(t *testing.T) {
	out := captureStdout(t, func() {
		_ = runWithSpinnerTimeout("正在保存配置...", 5*time.Second, func() error {
			time.Sleep(250 * time.Millisecond) // 让动画至少画几帧
			return nil
		})
	})
	// 清行序列之后不得再有可见字符
	idx := strings.LastIndex(out, "\r")
	if idx < 0 {
		t.Fatalf("expected carriage returns in spinner output, got %q", out)
	}
	if tail := strings.TrimSpace(stripControl(out[idx:])); tail != "" {
		t.Fatalf("残帧未被清除，清行后仍有可见内容: %q", tail)
	}
}

// ==================== saveConfigError ====================

// TestBuildSaveConfigErrorTypedNil 防回归 typed-nil 陷阱：两个子错误都为 nil 时必须返回
// 无类型 nil。若返回 (*saveConfigError)(nil)，err != nil 会成立，每次成功保存都被判为失败。
func TestBuildSaveConfigErrorTypedNil(t *testing.T) {
	if err := buildSaveConfigError(nil, nil); err != nil {
		t.Fatalf("成功路径必须返回无类型 nil，got %#v（typed-nil 陷阱）", err)
	}
}

func TestSaveConfigErrorMessageAndEnvOnly(t *testing.T) {
	envErr := errors.New("setx 挂了")
	settingsErr := errors.New("settings 挂了")

	t.Run("仅 env 失败", func(t *testing.T) {
		err := buildSaveConfigError(envErr, nil)
		var sce *saveConfigError
		if !errors.As(err, &sce) {
			t.Fatalf("expected *saveConfigError, got %T", err)
		}
		if !sce.envOnly() {
			t.Fatal("expected envOnly()=true —— 这是'跳过系统环境变量'选项出现的前提")
		}
		if !strings.Contains(err.Error(), "系统环境变量写入失败") || strings.Contains(err.Error(), "settings.json 写入失败") {
			t.Fatalf("unexpected message: %q", err.Error())
		}
	})

	t.Run("两者都失败时不得提供跳过", func(t *testing.T) {
		err := buildSaveConfigError(envErr, settingsErr)
		var sce *saveConfigError
		if !errors.As(err, &sce) {
			t.Fatalf("expected *saveConfigError, got %T", err)
		}
		if sce.envOnly() {
			t.Fatal("settings.json 也失败时不能声称'仅 settings.json 生效'")
		}
		if !strings.Contains(err.Error(), "；") {
			t.Fatalf("expected both failures joined, got %q", err.Error())
		}
	})

	t.Run("超时能穿透 Unwrap 被 errors.Is 命中", func(t *testing.T) {
		err := buildSaveConfigError(fmt.Errorf("wrap: %w", errCommandTimeout), nil)
		if !errors.Is(err, errCommandTimeout) {
			t.Fatalf("errors.Is 未能穿透 saveConfigError: %v", err)
		}
	})
}

// TestRunWithSpinnerTimeoutShowsElapsed 验证动画在超过阈值后显示已耗时——
// 这正是用户"看不出程序还在不在动"的痛点：慢和卡死必须能一眼分开。
func TestRunWithSpinnerTimeoutShowsElapsed(t *testing.T) {
	orig := spinnerElapsedThreshold
	spinnerElapsedThreshold = 100 * time.Millisecond
	defer func() { spinnerElapsedThreshold = orig }()

	out := captureStdout(t, func() {
		_ = runWithSpinnerTimeout("正在保存配置...", 30*time.Second, func() error {
			time.Sleep(500 * time.Millisecond)
			return nil
		})
	})
	// 不写死秒数：机器负载会影响阈值后第一帧落在第几秒
	if !regexp.MustCompile(`正在保存配置\.\.\. \(\d+s\)`).MatchString(out) {
		t.Fatalf("动画未显示已耗时，输出: %q", stripControl(out))
	}
}

// ==================== 保存失败处置菜单 ====================

// TestBuildSaveFailureMenu 覆盖用户在规划阶段亲自选定的行为：超时后能拿到「重试 / 跳过 / 退出」。
// 关键在于两个条件分支——spinner 兜底超时时不得提供「重试」（task 已被遗弃且仍在后台写文件），
// settings.json 也失败时不得提供「跳过」（那样等于承诺一份并不存在的可用配置）。
func TestBuildSaveFailureMenu(t *testing.T) {
	spinnerTimeout := fmt.Errorf("正在保存配置: %w", errSpinnerTimeout)
	envOnlyErr := buildSaveConfigError(errors.New("setx 挂了"), nil)
	bothErr := buildSaveConfigError(errors.New("setx 挂了"), errors.New("settings 挂了"))

	cases := []struct {
		name      string
		err       error
		allowSkip bool
		want      []saveFailureAction
	}{
		{"命令超时+允许跳过=重试/跳过/退出", envOnlyErr, true,
			[]saveFailureAction{saveActionRetry, saveActionSkipEnv, saveActionAbort}},
		{"命令超时+不允许跳过=重试/退出", envOnlyErr, false,
			[]saveFailureAction{saveActionRetry, saveActionAbort}},
		{"两侧都失败=不给跳过", bothErr, true,
			[]saveFailureAction{saveActionRetry, saveActionAbort}},
		{"spinner兜底超时=只剩退出", spinnerTimeout, true,
			[]saveFailureAction{saveActionAbort}},
		{"普通错误+允许跳过=不给跳过", errors.New("其他失败"), true,
			[]saveFailureAction{saveActionRetry, saveActionAbort}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			items, actions := buildSaveFailureMenu(c.err, c.allowSkip)
			if len(items) != len(actions) {
				t.Fatalf("items(%d) 与 actions(%d) 必须一一对应", len(items), len(actions))
			}
			if len(actions) != len(c.want) {
				t.Fatalf("actions = %v, want %v", actions, c.want)
			}
			for i := range c.want {
				if actions[i] != c.want[i] {
					t.Fatalf("actions[%d] = %v, want %v（完整: %v）", i, actions[i], c.want[i], actions)
				}
				// 菜单键必须是 1-based 连续编号，runItemMenu 的降级数字输入分支依赖它
				if items[i].Key != strconv.Itoa(i+1) {
					t.Fatalf("items[%d].Key = %q, want %q", i, items[i].Key, strconv.Itoa(i+1))
				}
			}
		})
	}
}

// TestSaveSpinnerTimeoutCoversCommandBudget 防回归：spinner 兜底必须大于同一路径上
// 命令级超时之和的上界，否则慢（而非卡死）的保存会先撞 spinner 超时，而那条路径上
// 「重试」和「跳过」都不可用，用户白输一遍配置。
func TestSaveSpinnerTimeoutCoversCommandBudget(t *testing.T) {
	// Windows 最坏：每个受管变量 setx + REG QUERY 两个进程
	worst := time.Duration(2*len(allEnvVarKeys)) * defaultCommandTimeout
	if saveSpinnerTimeout <= worst {
		t.Fatalf("saveSpinnerTimeout=%v 未覆盖命令级预算上界 %v", saveSpinnerTimeout, worst)
	}
}

// ==================== VSCode settings.json 交互移出 spinner ====================

// TestSaveVSCodeConfigInvalidJSONReturnsSentinel 防回归本次修复的首要目标：
// saveVSCodeConfig 曾在这条分支里直接 styledConfirm 读键盘，而它被包在 spinner 内——
// 动画每 80ms 重绘会把菜单冲掉，程序停在 os.Stdin.Read 上等一个看不见的按键。
// 现在它必须只返回哨兵错误，把询问留给 spinner 外的调用方。
func TestSaveVSCodeConfigInvalidJSONReturnsSentinel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "")

	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		t.Fatalf("getVSCodeSettingsPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("{ this is not json"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{BaseURL: "https://example.com", AuthToken: "sk-test", Model: "claude-opus-5"}

	// 把 stdin 换成空文件：若实现里还残留 styledConfirm 之类的读键盘调用，
	// 它会立刻读到 EOF 而不是挂起，测试仍能跑完——但真正的保障是下面的哨兵断言。
	origStdin := os.Stdin
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = devNull
	defer func() { os.Stdin = origStdin; devNull.Close() }()

	saveErr := saveVSCodeConfig(cfg)
	if !errors.Is(saveErr, errVSCodeSettingsInvalid) {
		t.Fatalf("expected errVSCodeSettingsInvalid, got %v", saveErr)
	}

	// 原文件必须原封不动（重建只能在用户确认后发生）
	raw, _ := os.ReadFile(settingsPath)
	if string(raw) != "{ this is not json" {
		t.Fatalf("saveVSCodeConfig 不该在未确认时改写原文件，现内容: %q", string(raw))
	}
	if _, err := os.Stat(settingsPath + ".bak"); err == nil {
		t.Fatal("saveVSCodeConfig 不该在未确认时产生备份")
	}
}

func TestRebuildVSCodeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv(envAgentTeams, "")
	t.Setenv(envEffortLevel, "")

	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		t.Fatalf("getVSCodeSettingsPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("{ this is not json"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{BaseURL: "https://example.com", AuthToken: "sk-test", Model: "claude-opus-5"}
	backupPath, err := rebuildVSCodeConfig(cfg)
	if err != nil {
		t.Fatalf("rebuildVSCodeConfig: %v", err)
	}
	if backupPath != settingsPath+".bak" {
		t.Fatalf("backupPath = %q, want %q", backupPath, settingsPath+".bak")
	}
	if raw, err := os.ReadFile(backupPath); err != nil || string(raw) != "{ this is not json" {
		t.Fatalf("备份内容不对: err=%v raw=%q", err, string(raw))
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("重建后的 settings.json 不是合法 JSON: %v", err)
	}
	if _, ok := m[vscodeEnvKey]; !ok {
		t.Fatalf("重建后缺少 %s 键: %v", vscodeEnvKey, m)
	}
}
