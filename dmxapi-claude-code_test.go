package main

import "testing"

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
