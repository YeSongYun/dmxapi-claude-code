package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
