package main

import (
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
	// 提取 isWSL 的检测逻辑为可测试的纯函数
	detectWSL := func(content string) bool {
		lower := strings.ToLower(content)
		return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
	}

	cases := []struct {
		content string
		want    bool
	}{
		{"Linux version 5.15.0-microsoft-standard-WSL2", true},   // WSL2
		{"Linux version 4.4.0-19041-Microsoft (gcc ...)", true},  // WSL1
		{"Linux version 5.15.0 (Ubuntu 22.04)", false},           // 普通 Linux
		{"Darwin Kernel Version 23.0.0", false},                   // macOS（极端情况）
		{"", false},                                               // 空内容
		{"WSL kernel release", true},                              // 包含 WSL 关键字
	}
	for _, c := range cases {
		got := detectWSL(c.content)
		if got != c.want {
			t.Errorf("detectWSL(%q) = %v, want %v", c.content, got, c.want)
		}
	}
}
