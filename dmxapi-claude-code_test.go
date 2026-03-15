package main

import (
	"encoding/json"
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
		{"hello", 5},                        // 普通 ASCII
		{"\033[31mhello\033[0m", 5},          // SGR 颜色序列（原来就支持）
		{"\033[2Khello", 5},                  // 清行序列 \033[2K（修复前会多算1）
		{"\033[1;32mOK\033[0m", 2},           // 多参数 SGR
		{"你好", 4},                          // CJK 双宽字符
		{"\033[Ahello\033[B", 5},             // 光标移动序列（\033[A 上移，\033[B 下移）
		{"", 0},                              // 空字符串
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
		{"Linux version 5.15.0-microsoft-standard-WSL2", true},    // WSL2 微软内核
		{"Linux version 4.4.0-19041-Microsoft", true},             // WSL1 旧格式
		{"Linux version 5.4.0-generic #1 Ubuntu", false},          // 普通 Linux
		{"Darwin Kernel Version 23.0.0", false},                   // macOS
		{"", false},                                               // 空内容
		{"some wsl mention", true},                                // 包含 wsl 关键字
	}
	for _, c := range cases {
		got := wslContentMatches(c.input)
		if got != c.want {
			t.Errorf("wslContentMatches(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestSetxOrRegAdd(t *testing.T) {
	// 提取"选择命令"逻辑为可测试的纯函数
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
		shellEnv string
		goos     string
		wantFile string   // configFiles[0] 的期望值
		wantSrc  string   // sourceCmd 期望值
		wantFish bool
	}{
		// macOS 场景
		{"/bin/zsh", "darwin", ".zshrc", "source ~/.zshrc", false},
		{"/bin/bash", "darwin", ".bash_profile", "source ~/.bash_profile", false},
		{"/usr/local/bin/fish", "darwin", ".config/fish/config.fish", "", true},
		// Linux 场景
		{"/bin/zsh", "linux", ".zshrc", "source ~/.zshrc", false},
		{"/bin/bash", "linux", ".bashrc", "source ~/.bashrc", false},
		{"/usr/bin/fish", "linux", ".config/fish/config.fish", "", true},
		// 非标准 shell 路径
		{"/opt/homebrew/bin/zsh", "darwin", ".zshrc", "source ~/.zshrc", false},
		// 空 SHELL 变量回退
		{"", "darwin", ".zshrc", "", false},
		{"", "linux", ".bashrc", "", false},
	}

	for _, c := range cases {
		t.Setenv("SHELL", c.shellEnv)
		profile := detectShellProfile(c.goos)
		if len(profile.configFiles) == 0 {
			t.Errorf("SHELL=%q GOOS=%q: configFiles 为空", c.shellEnv, c.goos)
			continue
		}
		if profile.configFiles[0] != c.wantFile {
			t.Errorf("SHELL=%q GOOS=%q: configFiles[0]=%q, want %q",
				c.shellEnv, c.goos, profile.configFiles[0], c.wantFile)
		}
		if c.shellEnv != "" && profile.sourceCmd != c.wantSrc {
			t.Errorf("SHELL=%q GOOS=%q: sourceCmd=%q, want %q",
				c.shellEnv, c.goos, profile.sourceCmd, c.wantSrc)
		}
		if profile.isFish != c.wantFish {
			t.Errorf("SHELL=%q GOOS=%q: isFish=%v, want %v",
				c.shellEnv, c.goos, profile.isFish, c.wantFish)
		}
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
			t.Errorf("vscodeSettingsPathFor(%q,%q,%q,%q)\ngot  %q\nwant %q",
				c.goos, c.homeDir, c.appData, c.wslWindowsHome, got, c.want)
		}
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

	// 不含 Agent Teams
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

	// 含 Agent Teams
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
	envVars := []map[string]string{
		{"name": "ANTHROPIC_BASE_URL", "value": "https://api.example.com"},
	}

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
