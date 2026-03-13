// claude-cli-setup.go - Interactive setup for Anthropic Claude Code CLI
// 跨平台配置工具，支持 Windows/Linux/macOS

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

// ==================== 常量定义 ====================

const (
	// 应用名称
	appName = "Anthropic Claude Code CLI"

	// 环境变量名
	envBaseURL     = "ANTHROPIC_BASE_URL"
	envAuthToken   = "ANTHROPIC_AUTH_TOKEN"
	envModel       = "ANTHROPIC_MODEL"
	envHaikuModel  = "ANTHROPIC_DEFAULT_HAIKU_MODEL"
	envSonnetModel = "ANTHROPIC_DEFAULT_SONNET_MODEL"
	envOpusModel               = "ANTHROPIC_DEFAULT_OPUS_MODEL"
	envDisableExperimentalBetas = "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"
	envAgentTeams               = "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"

	// 默认模型值
	defaultModel       = "claude-sonnet-4-6-cc"
	defaultHaikuModel  = "claude-haiku-4-5-20251001-cc"
	defaultSonnetModel = "claude-sonnet-4-6-cc"
	defaultOpusModel   = "claude-opus-4-6-cc"

	fixedDisableExperimentalBetas = "1"
)

var presetModels = []string{
	"claude-opus-4-6-cc",
	"claude-sonnet-4-6-cc",
	"claude-haiku-4-5-20251001-cc",
	"MiniMax-M2.5-cc",
	"glm-5-cc",
	"kimi-k2.5-cc",
	"mimo-v2-flash-cc",
	"hunyuan-2.0-thinking-20251109-cc",
	"qwen3.5-plus-cc",
	"qwen3.5-flash-cc",
	"DeepSeek-V3.2-cc",
	"hunyuan-2.0-instruct-20251111-cc",
	"claude-opus-4-6",
	"claude-sonnet-4-6",
	"claude-haiku-4-5-20251001",
}

// 颜色代码
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	// 亮色系
	colorBrightRed     = "\033[91m"
	colorBrightGreen   = "\033[92m"
	colorBrightYellow  = "\033[93m"
	colorBrightBlue    = "\033[94m"
	colorBrightMagenta = "\033[95m"
	colorBrightCyan    = "\033[96m"
	colorBrightWhite   = "\033[97m"
	colorMagenta       = "\033[35m"
	colorWhite         = "\033[37m"
	// 文字样式
	styleBold = "\033[1m"
	styleDim  = "\033[2m"
	// 版本号
	appVersion = "1.4.4"
	// 统一盒子内容宽度（不含左右边框字符）
	boxWidth = 60
)

// rawModeState 保存终端 raw 模式前的状态，用于 Ctrl+C 时恢复
var rawModeState *term.State

// Config 存储所有配置项
type Config struct {
	BaseURL     string
	AuthToken   string
	Model       string
	HaikuModel  string
	SonnetModel string
	OpusModel   string
}

// ==================== 工具函数 ====================

// printColor 打印带颜色的文本
func printColor(color, text string) {
	fmt.Print(color + text + colorReset)
}

// printSuccess 打印成功信息
func printSuccess(text string) {
	fmt.Printf("%s%s✔%s %s\n", colorReset, colorBrightGreen, colorReset, text)
}

// printError 打印错误信息
func printError(text string) {
	fmt.Printf("%s%s✘%s %s%s%s\n", colorReset, colorBrightRed, colorReset, colorBrightRed, text, colorReset)
}

// printWarning 打印警告信息
func printWarning(text string) {
	fmt.Printf("%s%s⚠%s %s%s%s\n", colorReset, colorBrightYellow, colorReset, colorBrightYellow, text, colorReset)
}

// printInfo 打印信息
func printInfo(text string) {
	fmt.Printf("%s%s→%s %s\n", colorReset, colorBrightCyan, colorReset, text)
}

// runWithSpinner 带旋转动画执行任务
func runWithSpinner(message string, task func() error) error {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan bool, 1) // 带缓冲，防止 task panic 时 goroutine 阻塞
	var err error

	go func() {
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r  %s%s%s %s%s%s", styleBold+colorBrightCyan, spinner[i], colorReset, colorBrightWhite, message, colorReset)
				i = (i + 1) % len(spinner)
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	err = task()
	done <- true

	clearLen := visibleLength(message) + 6
	fmt.Print("\r" + strings.Repeat(" ", clearLen) + "\r")
	return err
}

// ==================== 终端 UI 组件 ====================

// runeWidth 返回单个 rune 在终端中的显示宽度（1 或 2）
func runeWidth(r rune) int {
	// 东亚双宽字符完整范围
	if (r >= 0x2E80 && r <= 0x2FFF) || // CJK Radicals / 康熙部首
		(r >= 0x3000 && r <= 0x303F) || // CJK 符号和标点
		(r >= 0x3040 && r <= 0x30FF) || // 日文平假名 + 片假名
		(r >= 0x3100 && r <= 0x312F) || // 注音符号
		(r >= 0x3400 && r <= 0x4DBF) || // CJK 统一汉字扩展 A
		(r >= 0x4E00 && r <= 0x9FFF) || // CJK 统一汉字
		(r >= 0xAC00 && r <= 0xD7AF) || // 韩文音节
		(r >= 0xF900 && r <= 0xFAFF) || // CJK 兼容汉字
		(r >= 0xFE30 && r <= 0xFE4F) || // CJK 兼容形式
		(r >= 0xFF01 && r <= 0xFF60) || // 全宽 ASCII + 全宽标点
		(r >= 0xFFE0 && r <= 0xFFE6) || // 全宽货币符号等
		(r >= 0x20000 && r <= 0x2FA1F) { // CJK 扩展 B~F + 兼容补充
		return 2
	}
	return 1
}

// visibleLength 计算字符串在终端中的可见宽度（ANSI 感知 + CJK 双宽度）
func visibleLength(s string) int {
	inEscape := false
	csiStarted := false // 已消耗 ESC，等待 '[' 来确认 CSI 序列
	count := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			csiStarted = false
			continue
		}
		if inEscape {
			if !csiStarted {
				// 等待 '[' 以确认 CSI 序列
				if r == '[' {
					csiStarted = true
				} else {
					// 非 CSI 序列（如 ESC c），直接结束转义
					inEscape = false
				}
				continue
			}
			// 在 CSI 序列中：终止字节范围 0x40–0x7E（任意 CSI 终止字节）
			if r >= 0x40 && r <= 0x7E {
				inEscape = false
				csiStarted = false
			}
			continue
		}
		count += runeWidth(r)
	}
	return count
}

// printLogo 打印 ASCII Art Logo
func printLogo() {
	if runtime.GOOS == "windows" {
		fmt.Println()
		fmt.Println(colorCyan + styleBold + "  === DMXAPI ===" + colorReset)
		fmt.Println(styleDim + "  Claude Code CLI 配置工具" + colorReset)
		fmt.Printf("  %s%s/%s%s\n\n", colorMagenta, runtime.GOOS, runtime.GOARCH, colorReset)
		return
	}
	logo := []string{
		`██████╗ ███╗   ███╗██╗  ██╗ █████╗ ██████╗ ██╗`,
		`██╔══██╗████╗ ████║╚██╗██╔╝██╔══██╗██╔══██╗██║`,
		`██║  ██║██╔████╔██║ ╚███╔╝ ███████║██████╔╝██║`,
		`██║  ██║██║╚██╔╝██║ ██╔██╗ ██╔══██║██╔═══╝ ██║`,
		`██████╔╝██║ ╚═╝ ██║██╔╝ ██╗██║  ██║██║     ██║`,
		`╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝`,
	}
	colors := []string{
		colorBrightCyan, colorBrightCyan,
		colorCyan, colorCyan,
		colorBlue, colorBlue,
	}
	fmt.Println()
	for i, line := range logo {
		fmt.Println("  " + colors[i] + styleBold + line + colorReset)
	}
	fmt.Println()
	fmt.Println("  " + styleDim + colorBrightWhite +
		"Claude Code CLI 配置工具  ·  让 AI 触手可及" + colorReset)
	fmt.Printf("  %s%sv%s  %s%s/%s%s\n\n",
		styleDim, colorWhite, appVersion, colorReset,
		colorMagenta, runtime.GOOS+"/"+runtime.GOARCH, colorReset)
}

// printSectionHeader 打印章节标题
func printSectionHeader(title string) {
	fmt.Printf("\n%s┌─%s %s%s%s\n", colorBrightBlue, colorReset, styleBold, title, colorReset)
}

// printTip 打印提示信息
func printTip(text string) {
	fmt.Printf("  %s◆%s %s\n", colorBrightBlue, colorReset, text)
}

// printBox 打印双线边框盒子
func printBox(title, titleColor string, lines []string) {
	border := strings.Repeat("═", boxWidth)
	fmt.Printf("╔%s╗\n", border)

	// 标题居中
	titleVisible := visibleLength(title)
	padding := boxWidth - titleVisible
	left := padding / 2
	right := padding - left
	fmt.Printf("║%s%s%s%s%s║\n",
		strings.Repeat(" ", left), titleColor+styleBold, title, colorReset, strings.Repeat(" ", right))

	fmt.Printf("╠%s╣\n", border)

	for _, line := range lines {
		lineVisible := visibleLength(line)
		pad := boxWidth - lineVisible - 2 // 2 for leading spaces
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("║  %s%s║\n", line, strings.Repeat(" ", pad))
	}

	fmt.Printf("╚%s╝\n", border)
}

// MenuItem 菜单项
type MenuItem struct {
	Key   string
	Label string
	Desc  string
}

// KeyType 键盘输入类型
type KeyType int

const (
	KeyUp    KeyType = iota
	KeyDown
	KeyEnter
	KeyEsc
	KeyOther
)

// modelTypeEntry 模型类型条目
type modelTypeEntry struct {
	Label    string
	ValuePtr *string
}

// printMenu 打印圆角边框菜单
func printMenu(title string, items []MenuItem) {
	border := strings.Repeat("─", boxWidth)
	fmt.Printf("╭%s╮\n", border)

	// 标题居中
	titleVisible := visibleLength(title)
	padding := boxWidth - titleVisible
	left := padding / 2
	right := padding - left
	fmt.Printf("│%s%s%s%s%s│\n",
		strings.Repeat(" ", left), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", right))

	fmt.Printf("├%s┤\n", border)

	for _, item := range items {
		// 格式: │  [1]  主标签  暗色副描述  │
		content := fmt.Sprintf("%s[%s]%s  %s%s%s  %s%s%s",
			colorBrightYellow, item.Key, colorReset,
			styleBold+colorBrightWhite, item.Label, colorReset,
			styleDim, item.Desc, colorReset)
		contentVisible := 5 + visibleLength(item.Label) + 2 + visibleLength(item.Desc) // [X]+2sp+label+2sp+desc
		pad := boxWidth - contentVisible - 2
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("│  %s%s│\n", content, strings.Repeat(" ", pad))
	}

	fmt.Printf("╰%s╯\n", border)
}

// ==================== 样式输入函数 ====================

// styledInput 带样式提示符的文本输入
func styledInput(label string) string {
	fmt.Printf("  %s❯%s %s%s:%s ", colorBrightCyan, colorReset, styleBold, label, colorReset)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// styledPassword 带样式提示符的隐藏输入
func styledPassword(label string) string {
	fmt.Printf("  %s❯%s %s%s:%s ", colorBrightCyan, colorReset, styleBold, label, colorReset)
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		pw, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(pw))
	}
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// styledConfirm 带样式提示符的确认菜单
func styledConfirm(label string) bool {
	return runConfirmMenu(label)
}

// ==================== 输入处理 ====================

// readInput 读取用户输入
func readInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

// readPassword 读取密码输入（隐藏字符）
func readPassword(prompt string) string {
	fmt.Print(prompt)

	// 尝试从标准输入读取密码
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		fmt.Println() // 换行
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(password))
	}

	// 如果不是终端，回退到普通读取
	return readInput("")
}

// confirm 确认提示
func confirm(prompt string) bool {
	input := readInput(prompt + " (y/N): ")
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

// ==================== URL 处理 ====================

// ensureScheme 确保 URL 包含协议
func ensureScheme(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	// 检查是否已有协议
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	// 默认添加 https://
	return "https://" + rawURL
}

// extractHost 从 URL 提取主机名
func extractHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		// 简单提取
		rawURL = strings.TrimPrefix(rawURL, "https://")
		rawURL = strings.TrimPrefix(rawURL, "http://")
		parts := strings.SplitN(rawURL, "/", 2)
		return parts[0]
	}

	return parsed.Host
}

// validateURL 验证 URL 格式
func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL 不能为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 格式无效: %v", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL 必须使用 http 或 https 协议")
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL 必须包含主机名")
	}

	return nil
}

// ==================== 环境变量管理 ====================

// getEnvVar 获取环境变量
func getEnvVar(key string) string {
	return os.Getenv(key)
}

// shellProfile 描述用户当前 Shell 对应的配置文件信息
type shellProfile struct {
	configFiles []string // 相对于 HomeDir 的配置文件路径列表
	sourceCmd   string   // 提示用户执行的 source 命令（空串=回退模式）
	isFish      bool     // 是否为 fish shell（写法与 bash/zsh 不同）
}

// detectShellProfile 通过 $SHELL 环境变量检测用户的 shell，
// 返回对应的配置文件列表和 source 命令。
// goos 参数用于区分 darwin/linux 行为（通常传入 runtime.GOOS）。
// $SHELL 为空或未知时，sourceCmd 为空串（表示无法确定具体 source 命令），
// 但仍返回包含多个常见配置文件的 shellProfile（回退到兼容写入）。
func detectShellProfile(goos string) shellProfile {
	shell := os.Getenv("SHELL")

	switch {
	case strings.Contains(shell, "zsh"):
		return shellProfile{
			configFiles: []string{".zshrc"},
			sourceCmd:   "source ~/.zshrc",
		}
	case strings.Contains(shell, "fish"):
		return shellProfile{
			configFiles: []string{".config/fish/config.fish"},
			sourceCmd:   "source ~/.config/fish/config.fish",
			isFish:      true,
		}
	case strings.Contains(shell, "bash"):
		if goos == "darwin" {
			return shellProfile{
				configFiles: []string{".bash_profile"},
				sourceCmd:   "source ~/.bash_profile",
			}
		}
		return shellProfile{
			configFiles: []string{".bashrc"},
			sourceCmd:   "source ~/.bashrc",
		}
	default:
		// $SHELL 为空或未知 shell：回退到写全部常见文件（兼容旧行为）
		if goos == "darwin" {
			return shellProfile{configFiles: []string{".zshrc", ".bash_profile"}}
		}
		return shellProfile{configFiles: []string{".bashrc", ".profile"}}
	}
}

// setEnvVarsWindows 在 Windows 上批量设置用户环境变量（使用 SETX 并行执行）
func setEnvVarsWindows(vars map[string]string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(vars))

	for key, value := range vars {
		if value == "" {
			continue
		}
		wg.Add(1)
		go func(k, v string) {
			defer wg.Done()
			var err error
			if len(v) > 900 {
				// setx 有 1024 字节上限，超长值改用 REG ADD 直接写注册表
				err = runCommand("REG", "ADD", `HKCU\Environment`, "/V", k, "/T", "REG_SZ", "/D", v, "/F")
				if err != nil {
					errChan <- fmt.Errorf("设置环境变量 %s 失败（token 过长，注册表写入错误）：%v - 请重启终端后重试，或以管理员权限运行", k, err)
					return
				}
			} else {
				// 普通路径：使用 setx
				err = runCommand("setx", k, v)
				if err != nil {
					errChan <- fmt.Errorf("设置环境变量 %s 失败: %v", k, err)
				}
			}
		}(key, value)
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}

// setEnvVarsUnix 在 Unix 系统上批量设置环境变量（一次文件读写）
func setEnvVarsUnix(vars map[string]string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	profile := detectShellProfile(runtime.GOOS)

	// 写入配置文件
	for _, configFile := range profile.configFiles {
		configPath := filepath.Join(homeDir, configFile)

		if profile.isFish {
			// fish shell：确保目录存在，然后写入 set -Ux 语法
			if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
				return fmt.Errorf("创建 %s 目录失败: %v", filepath.Dir(configPath), err)
			}
			// 读取现有内容（文件不存在时从空内容开始）
			var existingContent []byte
			if _, statErr := os.Stat(configPath); statErr == nil {
				existingContent, _ = os.ReadFile(configPath)
			}
			normalized := strings.ReplaceAll(string(existingContent), "\r\n", "\n")
			normalized = strings.ReplaceAll(normalized, "\r", "\n")
			lines := strings.Split(normalized, "\n")
			newLines := make([]string, 0, len(lines))
			foundKeys := make(map[string]bool)

			for _, line := range lines {
				replaced := false
				for key, value := range vars {
					if value == "" {
						continue
					}
					marker := fmt.Sprintf("set -Ux %s ", key)
					if strings.HasPrefix(strings.TrimSpace(line), marker) {
						newLines = append(newLines, fmt.Sprintf("set -Ux %s %s", key, value))
						foundKeys[key] = true
						replaced = true
						break
					}
				}
				if !replaced {
					newLines = append(newLines, line)
				}
			}
			for key, value := range vars {
				if value == "" || foundKeys[key] {
					continue
				}
				newLines = append(newLines, fmt.Sprintf("set -Ux %s %s", key, value))
			}
			newContent := strings.Join(newLines, "\n")
			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}
			if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("写入 %s 失败: %v", configPath, err)
			}
			continue
		}

		// bash / zsh / 回退路径：使用 export KEY='VALUE' 语法
		// 主配置文件（如 .zshrc）不存在时自动创建；其余文件不存在则跳过
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if configFile == profile.configFiles[0] {
				if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
					return fmt.Errorf("创建 %s 失败: %v", configPath, err)
				}
			} else {
				continue
			}
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		lines := strings.Split(normalized, "\n")
		newLines := make([]string, 0, len(lines))
		foundKeys := make(map[string]bool)

		for _, line := range lines {
			replaced := false
			for key, value := range vars {
				if value == "" {
					continue
				}
				marker := fmt.Sprintf("export %s=", key)
				if strings.HasPrefix(strings.TrimSpace(line), marker) {
					exportLine := fmt.Sprintf("export %s='%s'", key, strings.ReplaceAll(value, "'", "'\\''"))
					newLines = append(newLines, exportLine)
					foundKeys[key] = true
					replaced = true
					break
				}
			}
			if !replaced {
				newLines = append(newLines, line)
			}
		}

		for key, value := range vars {
			if value == "" || foundKeys[key] {
				continue
			}
			exportLine := fmt.Sprintf("export %s='%s'", key, strings.ReplaceAll(value, "'", "'\\''"))
			newLines = append(newLines, exportLine)
		}

		newContent := strings.Join(newLines, "\n")
		if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("写入 %s 失败: %v", configPath, err)
		}
	}

	return nil
}

// removeEnvVarUnix 从 Unix shell 配置文件中删除指定环境变量（幂等）
func removeEnvVarUnix(key string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	profile := detectShellProfile(runtime.GOOS)

	// fish shell 用 set -e 语法删除变量
	fishMarker := fmt.Sprintf("set -Ux %s ", key)
	exportMarker := fmt.Sprintf("export %s=", key)

	for _, configFile := range profile.configFiles {
		configPath := filepath.Join(homeDir, configFile)
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue // 文件不存在，无需删除
		}
		content, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		info, statErr := os.Stat(configPath)
		perm := os.FileMode(0644)
		if statErr == nil {
			perm = info.Mode()
		}
		normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		lines := strings.Split(normalized, "\n")

		// 根据 shell 选择要删除的行标记
		var lineMarker string
		if profile.isFish {
			lineMarker = fishMarker
		} else {
			lineMarker = exportMarker
		}

		newLines := make([]string, 0, len(lines))
		prevBlank := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), lineMarker) {
				continue
			}
			isBlank := strings.TrimSpace(line) == ""
			if isBlank && prevBlank {
				continue
			}
			newLines = append(newLines, line)
			prevBlank = isBlank
		}

		newContent := strings.Join(newLines, "\n")
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		if err := os.WriteFile(configPath, []byte(newContent), perm); err != nil {
			return fmt.Errorf("写入 %s 失败: %v", configPath, err)
		}
	}
	return nil
}

// removeEnvVarWindows 从 Windows 用户环境变量中删除指定变量
func removeEnvVarWindows(key string) error {
	// 优先用 REG DELETE 删除注册表用户变量
	err := runCommand("REG", "DELETE", `HKCU\Environment`, "/V", key, "/F")
	if err != nil {
		// 降级：setx 设置为空（Windows 下可能不完全清除，打印警告）
		setxErr := runCommand("setx", key, "")
		printWarning(fmt.Sprintf("无法完全清除 %s，请手动在系统环境变量中删除该项", key))
		if setxErr != nil {
			return fmt.Errorf("删除 %s 失败: REG DELETE 和 setx 均出错", key)
		}
	}
	return nil
}

func wslContentMatches(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

// isWSL 检测当前是否运行在 Windows Subsystem for Linux (WSL) 环境中
// 通过读取 /proc/version 文件内容判断，失败时返回 false（安全静默）
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return wslContentMatches(string(data))
}

// runCommand 执行命令
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ==================== API 验证 ====================

// APIResponse API 响应结构
type APIResponse struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// validateAPIConnection 验证 API 连接
func validateAPIConnection(baseURL, authToken, model string) error {
	// 构建测试请求 URL
	testURL := strings.TrimSuffix(baseURL, "/") + "/v1/messages"

	// 创建一个简单的测试请求体
	requestBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", testURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", authToken)
	req.Header.Set("anthropic-version", "2023-06-01")

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	printInfo("正在验证 API 连接...")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查状态码
	switch resp.StatusCode {
	case 200:
		// 成功
		return nil
	case 401:
		return fmt.Errorf("认证失败: API Token 无效")
	case 403:
		return fmt.Errorf("权限被拒绝: 请检查 API Token 权限")
	case 404:
		return fmt.Errorf("API 端点不存在或模型名称不正确: 请检查 Base URL 和模型名")
	case 429:
		// 速率限制也表示认证成功
		return nil
	default:
		// 尝试解析错误信息
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Error != nil {
			return fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, apiResp.Error.Message)
		}
		return fmt.Errorf("API 返回错误状态码: %d", resp.StatusCode)
	}
}

// ==================== 配置流程 ====================

// loadExistingConfig 加载现有配置
func loadExistingConfig() Config {
	return Config{
		BaseURL:     getEnvVar(envBaseURL),
		AuthToken:   getEnvVar(envAuthToken),
		Model:       getEnvVar(envModel),
		HaikuModel:  getEnvVar(envHaikuModel),
		SonnetModel: getEnvVar(envSonnetModel),
		OpusModel:   getEnvVar(envOpusModel),
	}
}

// getNewBaseURL 获取新的 Base URL
func getNewBaseURL(existing string) string {
	printSectionHeader("配置 API 服务器地址")
	fmt.Println("  示例: https://www.dmxapi.cn")

	if existing != "" {
		fmt.Printf("  当前值: %s\n", existing)
		if !styledConfirm("是否修改 Base URL") {
			return existing
		}
	}

	for {
		input := styledInput("Base URL")
		if input == "" && existing != "" {
			return existing
		}

		input = ensureScheme(input)
		if err := validateURL(input); err != nil {
			printError(err.Error())
			continue
		}

		return input
	}
}

// getNewAuthToken 获取新的 Auth Token
func getNewAuthToken(existing, hostname string) string {
	printSectionHeader("配置 API 认证令牌")

	if hostname != "" {
		fmt.Printf("  获取地址: https://%s/token\n", hostname)
	}

	if existing != "" {
		fmt.Printf("  当前已配置 Token: %s\n", maskToken(existing))
		if !styledConfirm("是否更新 Token") {
			return existing
		}
	}

	for {
		input := styledInput("Auth Token")
		if input == "" {
			if existing != "" {
				return existing
			}
			printError("Token 不能为空")
			continue
		}

		return strings.TrimSpace(input)
	}
}

// selectConfigMode 选择配置模式
// 返回值: 1 = 从头配置, 2 = 仅配置模型, 3 = 解决 400 报错, 4 = 配置实验性功能
func selectConfigMode() int {
	return runItemMenu("配置模式选择", []MenuItem{
		{"1", "从头配置", "配置 URL、Token 和模型"},
		{"2", "仅配置模型", "跳过 URL 和 Token 配置"},
		{"3", "解决 400 报错", "禁用实验性请求头"},
		{"4", "配置实验性功能", "启用/禁用 Agent Teams"},
	})
}

// selectFixOption 让用户选择要修改的内容
func selectFixOption() int {
	return runItemMenu("选择要修改的内容", []MenuItem{
		{"1", "修改 URL", "Base URL 有问题"},
		{"2", "修改 Key", "API Key 有问题"},
		{"3", "都修改", "URL 和 Key 都有问题"},
		{"4", "修改模型名", "模型名称可能不正确"},
	})
}

// inputNewBaseURL 输入新的 Base URL（无需确认是否修改）
func inputNewBaseURL() string {
	for {
		input := styledInput("新 Base URL")
		if input == "" {
			printError("URL 不能为空")
			continue
		}
		input = ensureScheme(input)
		if err := validateURL(input); err != nil {
			printError(err.Error())
			continue
		}
		return input
	}
}

// inputNewAuthToken 输入新的 Auth Token（无需确认是否修改）
func inputNewAuthToken(hostname string) string {
	if hostname != "" {
		fmt.Printf("  获取地址: https://%s/token\n", hostname)
	}
	for {
		input := styledInput("新 Auth Token")
		if input == "" {
			printError("Token 不能为空")
			continue
		}
		return strings.TrimSpace(input)
	}
}

// renderItemMenu 渲染通用条目菜单，返回渲染行数（len(items)+6）
func renderItemMenu(title string, items []MenuItem, selectedIdx int, linesPrinted int) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	border := strings.Repeat("─", boxWidth)
	fmt.Printf("╭%s╮\033[K\r\n", border)
	titleW := visibleLength(title)
	lPad := (boxWidth - titleW) / 2
	rPad := boxWidth - titleW - lPad
	fmt.Printf("│%s%s%s%s%s│\033[K\r\n",
		strings.Repeat(" ", lPad), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", rPad))
	fmt.Printf("├%s┤\033[K\r\n", border)
	for i, item := range items {
		labelW := visibleLength(item.Label)
		descW := visibleLength(item.Desc)
		pad := boxWidth - 5 - labelW - descW
		if pad < 0 {
			pad = 0
		}
		if i == selectedIdx {
			fmt.Printf("│ %s❯ %s%s  %s%s%s%s│\033[K\r\n",
				colorBrightCyan+styleBold,
				item.Label, colorReset,
				colorBrightCyan, item.Desc, colorReset,
				strings.Repeat(" ", pad))
		} else {
			fmt.Printf("│ %s  %s%s  %s%s%s%s│\033[K\r\n",
				styleDim,
				item.Label, colorReset,
				styleDim, item.Desc, colorReset,
				strings.Repeat(" ", pad))
		}
	}
	fmt.Printf("╰%s╯\033[K\r\n", border)
	fmt.Printf("\033[K\r\n")
	fmt.Printf("  %s↑↓ 导航%s  %sEnter 确认%s\033[K\r\n",
		styleDim, colorReset, styleDim, colorReset)
	return len(items) + 6
}

// runItemMenu 运行通用条目菜单，返回1-based选中索引
func runItemMenu(title string, items []MenuItem) int {
	n := len(items)
	restore, err := enterRawMode()
	if err != nil {
		// 降级：数字输入
		printMenu(title, items)
		fmt.Println()
		validKeys := make([]string, n)
		for i := range items {
			validKeys[i] = items[i].Key
		}
		for {
			input := styledInput("选项")
			for i, k := range validKeys {
				if input == k {
					return i + 1
				}
			}
			printError(fmt.Sprintf("无效选项，请输入 %s", strings.Join(validKeys, "、")))
		}
	}
	defer restore()

	selectedIdx := 0
	linesPrinted := 0
	for {
		linesPrinted = renderItemMenu(title, items, selectedIdx, linesPrinted)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + n) % n
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % n
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			return selectedIdx + 1
		}
		// ESC 不允许退出（必须做出选择），忽略
	}
}

// ==================== 交互式模型选择 ====================

// enterRawMode 进入终端原始模式，返回恢复函数
func enterRawMode() (restoreFn func(), err error) {
	fd := int(syscall.Stdin)
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("not a terminal")
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	rawModeState = oldState
	return func() {
		term.Restore(fd, oldState)
		rawModeState = nil
	}, nil
}

// readRawKey 在已进入 raw 模式的终端中读取一个按键
func readRawKey() KeyType {
	if runtime.GOOS == "windows" {
		return readConsoleKey()
	}
	buf := make([]byte, 1)
	os.Stdin.Read(buf)
	switch buf[0] {
	case 0x0D, 0x0A:
		return KeyEnter
	case 'q', 'Q':
		return KeyEsc
	case 0x03: // Ctrl+C
		if rawModeState != nil {
			term.Restore(int(syscall.Stdin), rawModeState)
			fmt.Println()
		}
		os.Exit(0)
	case 0x1B: // ESC 序列（Linux/macOS/Windows Terminal）
		if !stdinDataReady(100) {
			return KeyEsc // 单独按下 ESC 键，无后续字节
		}
		rest := make([]byte, 2)
		n, _ := os.Stdin.Read(rest)
		if n == 0 {
			return KeyEsc
		}
		if n >= 2 && rest[0] == '[' {
			switch rest[1] {
			case 'A':
				return KeyUp
			case 'B':
				return KeyDown
			}
		} else if n == 1 && rest[0] == '[' {
			// 降级：仅读到 '[' 时，再读一字节
			buf3 := make([]byte, 1)
			if n2, _ := os.Stdin.Read(buf3); n2 > 0 {
				switch buf3[0] {
				case 'A':
					return KeyUp
				case 'B':
					return KeyDown
				}
			}
		}
		return KeyOther
	}
	return KeyOther
}

// truncateStr 截断字符串，超过 maxLen 显示宽度时末尾加 "..."
func truncateStr(s string, maxLen int) string {
	if visibleLength(s) <= maxLen {
		return s
	}
	width := 0
	var result []rune
	for _, r := range s {
		rw := runeWidth(r)
		if width+rw+3 > maxLen { // 预留 3 字符给 "..."
			break
		}
		result = append(result, r)
		width += rw
	}
	return string(result) + "..."
}

// findPresetIndex 在 presetModels 中查找，找不到返回 -1
func findPresetIndex(value string) int {
	for i, m := range presetModels {
		if m == value {
			return i
		}
	}
	return -1
}

// clearMenuLines 清除 n 行菜单内容（上移并清行）
func clearMenuLines(n int) {
	if n <= 0 {
		return
	}
	fmt.Printf("\033[%dA", n)
	for i := 0; i < n; i++ {
		fmt.Printf("\r\033[K\n")
	}
	fmt.Printf("\033[%dA", n)
}

// renderConfirmMenuCore 渲染确认菜单核心逻辑，返回渲染行数（固定8行）
// selectedIdx: 0=选项1, 1=选项2
func renderConfirmMenuCore(question string, labels [2]string, descs [2]string, selectedIdx int, linesPrinted int) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	const innerW = 44
	border := strings.Repeat("─", innerW)
	fmt.Printf("╭%s╮\033[K\r\n", border)

	questionW := visibleLength(question)
	lPad := (innerW - questionW) / 2
	if lPad < 0 {
		lPad = 0
	}
	rPad := innerW - questionW - lPad
	if rPad < 0 {
		rPad = 0
	}
	fmt.Printf("│%s%s%s%s%s│\033[K\r\n",
		strings.Repeat(" ", lPad), styleBold+colorBrightWhite, question, colorReset, strings.Repeat(" ", rPad))
	fmt.Printf("├%s┤\033[K\r\n", border)

	for i := 0; i < 2; i++ {
		label := labels[i]
		desc := descs[i]
		pad := innerW - 5 - visibleLength(label) - visibleLength(desc)
		if pad < 0 {
			pad = 0
		}
		if i == selectedIdx {
			fmt.Printf("│ %s❯ %s%s  %s%s%s%s│\033[K\r\n",
				colorBrightCyan+styleBold,
				label, colorReset,
				colorBrightCyan, desc, colorReset,
				strings.Repeat(" ", pad))
		} else {
			fmt.Printf("│ %s  %s%s  %s%s%s%s│\033[K\r\n",
				styleDim,
				label, colorReset,
				styleDim, desc, colorReset,
				strings.Repeat(" ", pad))
		}
	}

	fmt.Printf("╰%s╯\033[K\r\n", border)
	fmt.Printf("\033[K\r\n")
	fmt.Printf("  %s↑↓ 导航%s  %sEnter 确认%s\033[K\r\n",
		styleDim, colorReset, styleDim, colorReset)
	return 8
}

// renderConfirmMenu 渲染默认确认菜单（是/否），返回渲染行数（固定8行）
// selectedIdx: 0=是, 1=否
func renderConfirmMenu(question string, selectedIdx int, linesPrinted int) int {
	return renderConfirmMenuCore(
		question,
		[2]string{"是", "否"},
		[2]string{"确认修改", "保持当前值不变"},
		selectedIdx,
		linesPrinted,
	)
}

// runConfirmMenu 运行确认菜单，返回是否确认（true=是，false=否）
// 默认选中"否"（与原来 y/N 默认 No 行为一致）
func runConfirmMenu(question string) bool {
	restore, err := enterRawMode()
	if err != nil {
		// 降级：非终端时用数字选项
		printMenu(question, []MenuItem{
			{"1", "是", "确认修改"},
			{"2", "否", "保持当前值不变"},
		})
		fmt.Println()
		for {
			input := styledInput("选项")
			switch input {
			case "1":
				return true
			case "2":
				return false
			default:
				printError("请输入 1 或 2")
			}
		}
	}
	defer restore()

	selectedIdx := 1 // 默认"否"
	linesPrinted := 0

	for {
		linesPrinted = renderConfirmMenu(question, selectedIdx, linesPrinted)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + 2) % 2
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % 2
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			return selectedIdx == 0
		case KeyEsc:
			restore()
			clearMenuLines(linesPrinted)
			return false
		}
	}
}

// runEnableDisableMenu 运行启用/禁用确认菜单，返回是否启用（true=启用，false=禁用）
// 默认选中"禁用"（索引1）
func runEnableDisableMenu(question string) bool {
	restore, err := enterRawMode()
	if err != nil {
		// 降级：非终端时用数字选项
		printMenu(question, []MenuItem{
			{"1", "启用", "开启 Agent Teams 功能"},
			{"2", "禁用", "关闭 Agent Teams 功能"},
		})
		fmt.Println()
		for {
			input := styledInput("选项")
			switch input {
			case "1":
				return true
			case "2":
				return false
			default:
				printError("请输入 1 或 2")
			}
		}
	}
	defer restore()

	selectedIdx := 1 // 默认"禁用"
	linesPrinted := 0

	for {
		linesPrinted = renderConfirmMenuCore(
			question,
			[2]string{"启用", "禁用"},
			[2]string{"开启 Agent Teams 功能", "关闭 Agent Teams 功能"},
			selectedIdx,
			linesPrinted,
		)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + 2) % 2
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % 2
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			return selectedIdx == 0
		case KeyEsc:
			restore()
			clearMenuLines(linesPrinted)
			return false
		}
	}
}

// renderL1Menu 渲染一级菜单，返回渲染行数（固定10行）
func renderL1Menu(entries []modelTypeEntry, selectedIdx int, linesPrinted int) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	border := strings.Repeat("─", boxWidth)
	fmt.Printf("╭%s╮\033[K\r\n", border)
	title := "选择要配置的模型"
	titleW := visibleLength(title)
	lPad := (boxWidth - titleW) / 2
	rPad := boxWidth - titleW - lPad
	fmt.Printf("│%s%s%s%s%s│\033[K\r\n",
		strings.Repeat(" ", lPad), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", rPad))
	fmt.Printf("├%s┤\033[K\r\n", border)

	// 计算所有标签的最大显示宽度
	maxLabelW := 0
	for _, e := range entries {
		if w := visibleLength(e.Label); w > maxLabelW {
			maxLabelW = w
		}
	}

	for i, entry := range entries {
		label := entry.Label
		labelFill := strings.Repeat(" ", maxLabelW-visibleLength(label))
		val := truncateStr(*entry.ValuePtr, 35)
		pad := boxWidth - 5 - maxLabelW - visibleLength(val)
		if pad < 0 {
			pad = 0
		}
		if i == selectedIdx {
			fmt.Printf("│ %s❯ %s%s%s  %s%s%s%s│\033[K\r\n",
				colorBrightCyan+styleBold,
				label, labelFill, colorReset,
				colorBrightCyan, val, colorReset,
				strings.Repeat(" ", pad))
		} else {
			fmt.Printf("│ %s  %s%s%s  %s%s%s%s│\033[K\r\n",
				styleDim,
				label, labelFill, colorReset,
				styleDim, val, colorReset,
				strings.Repeat(" ", pad))
		}
	}

	fmt.Printf("╰%s╯\033[K\r\n", border)
	fmt.Printf("\033[K\r\n")
	fmt.Printf("  %s↑↓ 导航%s  %sEnter 配置%s  %sq/Esc 保存退出%s\033[K\r\n",
		styleDim, colorReset, styleDim, colorReset, styleDim, colorReset)
	return len(entries) + 6
}

// renderL2Menu 渲染二级菜单，返回渲染行数（len(presetModels)+7）
func renderL2Menu(typeName string, currentValue string, selectedIdx int, linesPrinted int) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	border := strings.Repeat("─", boxWidth)
	fmt.Printf("╭%s╮\033[K\r\n", border)
	title := fmt.Sprintf("选择 %s", typeName)
	titleW := visibleLength(title)
	lPad := (boxWidth - titleW) / 2
	rPad := boxWidth - titleW - lPad
	fmt.Printf("│%s%s%s%s%s│\033[K\r\n",
		strings.Repeat(" ", lPad), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", rPad))
	fmt.Printf("├%s┤\033[K\r\n", border)

	for i, m := range presetModels {
		isCurrent := (m == currentValue)
		isSelected := (i == selectedIdx)
		name := truncateStr(m, boxWidth-6)
		nameW := visibleLength(name)
		var check string
		checkW := 2
		if isCurrent {
			check = fmt.Sprintf("%s✓%s", colorBrightGreen, colorReset)
			checkW = 1
		} else {
			check = "  "
		}
		pad := boxWidth - 4 - nameW - checkW
		if pad < 0 {
			pad = 0
		}
		if isSelected {
			fmt.Printf("│ %s❯%s %s%s%s%s %s│\033[K\r\n",
				colorBrightCyan+styleBold, colorReset,
				colorBrightCyan, name, colorReset,
				strings.Repeat(" ", pad),
				check)
		} else {
			fmt.Printf("│   %s%s%s%s %s│\033[K\r\n",
				styleDim, name, colorReset,
				strings.Repeat(" ", pad),
				check)
		}
	}

	// 自定义选项（索引 len(presetModels)）
	customText := "✏ 自定义输入..."
	customPad := boxWidth - 3 - visibleLength(customText)
	if selectedIdx == len(presetModels) {
		fmt.Printf("│ %s❯%s %s%s%s%s│\033[K\r\n",
			colorBrightCyan+styleBold, colorReset,
			colorBrightYellow, customText, colorReset,
			strings.Repeat(" ", customPad))
	} else {
		fmt.Printf("│   %s%s%s%s│\033[K\r\n",
			styleDim, customText, colorReset,
			strings.Repeat(" ", customPad))
	}

	fmt.Printf("╰%s╯\033[K\r\n", border)
	fmt.Printf("\033[K\r\n")
	fmt.Printf("  %s↑↓ 导航%s  %sEnter 确认%s  %sq/Esc 返回%s\033[K\r\n",
		styleDim, colorReset, styleDim, colorReset, styleDim, colorReset)
	return len(presetModels) + 7
}

// runL2Menu 运行二级菜单，返回选中的模型名
func runL2Menu(typeName, currentValue string) string {
	restore, err := enterRawMode()
	if err != nil {
		// 降级：直接文本输入
		val := styledInput(typeName + " (输入模型名，留空不改)")
		if val == "" {
			return currentValue
		}
		return val
	}
	defer restore()

	idx := findPresetIndex(currentValue)
	if idx < 0 {
		idx = 0
	}
	linesPrinted := 0

	for {
		linesPrinted = renderL2Menu(typeName, currentValue, idx, linesPrinted)
		key := readRawKey()
		switch key {
		case KeyUp:
			idx = (idx - 1 + len(presetModels)+1) % (len(presetModels) + 1)
		case KeyDown:
			idx = (idx + 1) % (len(presetModels) + 1)
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			if idx == len(presetModels) {
				// 自定义输入
				val := styledInput(typeName + " (自定义)")
				if val == "" {
					return currentValue
				}
				return val
			}
			return presetModels[idx]
		case KeyEsc:
			restore()
			clearMenuLines(linesPrinted)
			return currentValue
		}
	}
}

// runL1Menu 运行一级菜单
func runL1Menu(cfg *Config) {
	entries := []modelTypeEntry{
		{"默认模型", &cfg.Model},
		{"Haiku 模型", &cfg.HaikuModel},
		{"Sonnet 模型", &cfg.SonnetModel},
		{"Opus 模型", &cfg.OpusModel},
	}

	restore, err := enterRawMode()
	if err != nil {
		configureModelsFallback(cfg)
		return
	}
	defer restore()

	selectedIdx := 0
	linesPrinted := 0

	for {
		linesPrinted = renderL1Menu(entries, selectedIdx, linesPrinted)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + 4) % 4
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % 4
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			newVal := runL2Menu(entries[selectedIdx].Label, *entries[selectedIdx].ValuePtr)
			*entries[selectedIdx].ValuePtr = newVal
			// 重进 raw 模式
			var rerr error
			restore, rerr = enterRawMode()
			if rerr != nil {
				restore = func() {}
			}
			linesPrinted = 0
		case KeyEsc:
			restore()
			return
		}
	}
}

// configureModelsFallback 降级模型配置（文本输入模式）
func configureModelsFallback(cfg *Config) {
	fmt.Println()
	fmt.Println("当前模型配置:")
	fmt.Printf("  %-35s = %s\n", envModel, cfg.Model)
	fmt.Printf("  %-35s = %s\n", envHaikuModel, cfg.HaikuModel)
	fmt.Printf("  %-35s = %s\n", envSonnetModel, cfg.SonnetModel)
	fmt.Printf("  %-35s = %s\n", envOpusModel, cfg.OpusModel)

	if !styledConfirm("是否修改模型配置") {
		return
	}

	fmt.Println()
	input := styledInput("默认模型")
	if input != "" {
		cfg.Model = input
	}

	input = styledInput("Haiku 模型")
	if input != "" {
		cfg.HaikuModel = input
	}

	input = styledInput("Sonnet 模型")
	if input != "" {
		cfg.SonnetModel = input
	}

	input = styledInput("Opus 模型")
	if input != "" {
		cfg.OpusModel = input
	}
}

// configureModels 配置模型
func configureModels(cfg *Config) {
	// 填充默认值
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.HaikuModel == "" {
		cfg.HaikuModel = defaultHaikuModel
	}
	if cfg.SonnetModel == "" {
		cfg.SonnetModel = defaultSonnetModel
	}
	if cfg.OpusModel == "" {
		cfg.OpusModel = defaultOpusModel
	}

	printSectionHeader("配置模型设置")
	fmt.Println()

	runL1Menu(cfg)

	fmt.Println()
	printSuccess("模型配置已完成")
}

// saveConfig 保存配置（批量设置，一次系统调用）
func saveConfig(cfg Config) error {
	vars := map[string]string{
		envBaseURL:     cfg.BaseURL,
		envAuthToken:   cfg.AuthToken,
		envModel:       cfg.Model,
		envHaikuModel:  cfg.HaikuModel,
		envSonnetModel: cfg.SonnetModel,
		envOpusModel:               cfg.OpusModel,
		envDisableExperimentalBetas: fixedDisableExperimentalBetas,
	}

	// 设置当前进程环境变量
	for key, value := range vars {
		if value != "" {
			os.Setenv(key, value)
		}
	}

	// 批量持久化到系统
	switch runtime.GOOS {
	case "windows":
		return setEnvVarsWindows(vars)
	default:
		return setEnvVarsUnix(vars)
	}
}

// configureAgentTeams 配置实验性 Agent Teams 功能环境变量
func configureAgentTeams() {
	printSectionHeader("配置实验性 Agent Teams 功能")
	fmt.Println()

	currentVal := getEnvVar(envAgentTeams)
	if currentVal == "1" {
		printInfo(fmt.Sprintf("当前状态: %s已启用%s", colorBrightGreen, colorReset))
	} else {
		printInfo(fmt.Sprintf("当前状态: %s未开启%s", colorRed, colorReset))
	}
	fmt.Println()
	fmt.Printf("  Agent Teams 是 Claude Code 的实验性多智能体协作功能，\n")
	fmt.Printf("  允许多个 AI 代理并行处理复杂任务。\n")
	fmt.Println()
	fmt.Printf("  关闭后将移除 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS\n")
	fmt.Printf("  环境变量，Agent Teams 功能将停止工作。\n")
	fmt.Println()

	enable := runEnableDisableMenu("是否启用 Agent Teams 功能")

	fmt.Println()
	var err error
	if enable {
		vars := map[string]string{envAgentTeams: "1"}
		switch runtime.GOOS {
		case "windows":
			err = setEnvVarsWindows(vars)
		default:
			err = setEnvVarsUnix(vars)
		}
		if err != nil {
			printError(fmt.Sprintf("设置失败: %v", err))
		} else {
			os.Setenv(envAgentTeams, "1")
			printSuccess(fmt.Sprintf("已启用 %s=1", envAgentTeams))
		}
	} else {
		if currentVal == "" {
			printInfo("当前未设置该变量，无需操作")
		} else {
			switch runtime.GOOS {
			case "windows":
				err = removeEnvVarWindows(envAgentTeams)
			default:
				err = removeEnvVarUnix(envAgentTeams)
			}
			if err != nil {
				printError(fmt.Sprintf("删除失败: %v", err))
			} else {
				os.Unsetenv(envAgentTeams)
				printSuccess(fmt.Sprintf("已禁用并删除 %s", envAgentTeams))
			}
		}
	}

	fmt.Println()
	switch runtime.GOOS {
	case "windows":
		printTip("请重新打开终端窗口使配置生效")
	default:
		profile := detectShellProfile(runtime.GOOS)
		if profile.sourceCmd != "" {
			printTip(fmt.Sprintf("执行 %s 或重启终端使配置生效", profile.sourceCmd))
		} else {
			printTip("重启终端使配置生效")
		}
		if isWSL() {
			fmt.Println()
			printTip("注意：WSL 环境下，环境变量仅在当前 WSL 会话有效")
			printTip("若需要 Windows 侧程序读取，请在 Windows 侧单独配置")
		}
	}
	fmt.Println()
	styledInput("按回车键退出")
}

// printSummary 打印配置摘要
func printSummary(cfg Config) {
	fmt.Println()
	printSuccess("配置完成！")
	fmt.Println()

	// 构建表格行，标签列固定 14 字符
	makeRow := func(label, value, valueColor string) string {
		pad := 14 - visibleLength(label)
		if pad < 0 {
			pad = 0
		}
		return fmt.Sprintf("%s%s%s%s│ %s%s%s",
			styleBold+colorBrightWhite, label, colorReset,
			strings.Repeat(" ", pad),
			valueColor, value, colorReset)
	}

	lines := []string{
		makeRow("Base URL", cfg.BaseURL, colorBrightGreen),
		makeRow("Auth Token", maskToken(cfg.AuthToken), colorBrightYellow),
		makeRow("Model", cfg.Model, colorCyan),
		makeRow("Haiku Model", cfg.HaikuModel, colorCyan),
		makeRow("Sonnet Model", cfg.SonnetModel, colorCyan),
		makeRow("Opus Model", cfg.OpusModel, colorCyan),
		makeRow("Disable Betas", fixedDisableExperimentalBetas, colorMagenta),
	}
	printBox("配置摘要", colorBrightWhite, lines)

	fmt.Println()
	switch runtime.GOOS {
	case "windows":
		printTip("配置已保存到用户环境变量")
		printTip("请重新打开终端窗口使配置生效")
	default:
		profile := detectShellProfile(runtime.GOOS)
		// 构建写入文件列表的显示文本
		displayFiles := make([]string, len(profile.configFiles))
		for i, f := range profile.configFiles {
			displayFiles[i] = "~/" + f
		}
		printTip(fmt.Sprintf("配置已写入 %s", strings.Join(displayFiles, " 和 ")))
		if profile.sourceCmd != "" {
			printTip(fmt.Sprintf("执行 %s 或重启终端使配置生效", profile.sourceCmd))
		} else {
			printTip("重启终端使配置生效")
		}
		if isWSL() {
			fmt.Println()
			printTip("注意：WSL 环境下，环境变量仅在当前 WSL 会话有效")
			printTip("若需要 Windows 侧程序读取，请在 Windows 侧单独配置")
		}
	}
}

// maskToken 遮盖 Token
func maskToken(token string) string {
	runes := []rune(token)
	if len(runes) <= 8 {
		return "********"
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
}

// checkClaudeCodeInstalled 检测 claude 命令是否已安装
func checkClaudeCodeInstalled() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// compareVersions 比较两个版本号字符串（major.minor.patch 格式）
// 返回 -1（a<b）、0（a==b）、1（a>b）
// 段数不足3段时补0；任何段解析失败返回0（视为相等，不触发更新提示）
func compareVersions(a, b string) int {
	parseSegments := func(v string) ([3]int, bool) {
		parts := strings.SplitN(v, ".", 3) // 最多取3段
		var segs [3]int
		for i := 0; i < 3; i++ {
			if i < len(parts) {
				n, err := strconv.Atoi(parts[i])
				if err != nil {
					return [3]int{}, false // 解析失败
				}
				segs[i] = n
			}
		}
		return segs, true
	}
	sa, okA := parseSegments(a)
	sb, okB := parseSegments(b)
	if !okA || !okB {
		return 0 // 任何段解析失败返回0
	}
	for i := 0; i < 3; i++ {
		if sa[i] < sb[i] {
			return -1
		}
		if sa[i] > sb[i] {
			return 1
		}
	}
	return 0
}

// fetchLatestVersion 从 CNB releases 页面获取最新版本号（不含 v 前缀）
// 失败时返回空字符串（静默跳过）
func fetchLatestVersion() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	// 读取前 64KB，足以覆盖 initialState 中的第一个 tagRef
	lr := io.LimitReader(resp.Body, 65536)
	data, err := io.ReadAll(lr)
	if err != nil {
		return ""
	}
	body := string(data)

	// CNB releases 页面为 SSR，tagRef 按发布时间倒序，第一条即最新版
	re := regexp.MustCompile(`"tagRef":"refs/tags/(v\d+\.\d+\.\d+)"`)
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	// 去掉 "v" 前缀，返回如 "1.4.5"
	return strings.TrimPrefix(match[1], "v")
}

// openBrowser 用系统命令打开浏览器，失败时打印链接
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		printInfo("请手动访问: " + url)
	}
}

// checkForUpdates 检查是否有新版本，有则提示用户
func checkForUpdates() {
	latest := fetchLatestVersion()
	if latest == "" {
		return // 网络失败或解析失败，静默跳过
	}
	if compareVersions(appVersion, latest) >= 0 {
		return // 当前版本已是最新（含版本号解析失败的情况，安全静默跳过）
	}
	fmt.Println()
	printInfo(fmt.Sprintf("发现新版本 v%s（当前 v%s）", latest, appVersion))
	fmt.Println()
	wantDownload := runConfirmMenu(fmt.Sprintf("发现新版本 v%s，是否立即前往下载页？", latest))
	if wantDownload {
		openBrowser("https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases")
		os.Exit(0)
	}
}

// ==================== 主程序 ====================

func main() {
	initWindowsConsole()
	// 显示 Logo
	printLogo()

	// 检测 Claude Code 是否已安装
	if !checkClaudeCodeInstalled() {
		fmt.Println()
		printError("未检测到 Claude Code，请先安装后再运行此工具")
		fmt.Println()
		if runtime.GOOS == "windows" {
			printInfo("安装命令（PowerShell）:")
			fmt.Println("  irm https://claude.ai/install.ps1 | iex")
			fmt.Println()
			printInfo("安装命令（CMD）:")
			fmt.Println("  curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd")
		} else {
			printInfo("安装命令（macOS / Linux / WSL）:")
			fmt.Println("  curl -fsSL https://claude.ai/install.sh | bash")
		}
		fmt.Println()
		styledInput("按回车键退出")
		os.Exit(1)
	}

	// 检查版本更新（失败则静默跳过）
	checkForUpdates()

	// 选择配置模式
	configMode := selectConfigMode()

	// 加载现有配置
	cfg := loadExistingConfig()

	// 根据配置模式执行不同流程
	if configMode == 1 {
		// 从头配置模式
		// 配置 Base URL
		cfg.BaseURL = getNewBaseURL(cfg.BaseURL)

		// 提取主机名用于提示
		hostname := extractHost(cfg.BaseURL)

		// 配置 Auth Token
		cfg.AuthToken = getNewAuthToken(cfg.AuthToken, hostname)

		// 验证 API 连接（循环直到成功）
		// 若用户首次配置尚未选择模型，使用默认模型进行验证
		if cfg.Model == "" {
			cfg.Model = defaultModel
		}
		fmt.Println()
		for {
			if err := validateAPIConnection(cfg.BaseURL, cfg.AuthToken, cfg.Model); err != nil {
				printError(fmt.Sprintf("API 连接验证失败: %v", err))

				// 显示当前的URL和Key
				fmt.Println()
				printInfo("当前配置:")
				fmt.Printf("  Base URL: %s\n", cfg.BaseURL)
				fmt.Printf("  API Key:  %s\n", cfg.AuthToken)
				fmt.Println()

				// 让用户选择要修改什么
				choice := selectFixOption()

				switch choice {
				case 1: // 修改URL
					cfg.BaseURL = inputNewBaseURL()
					hostname = extractHost(cfg.BaseURL)
				case 2: // 修改Key
					cfg.AuthToken = inputNewAuthToken(hostname)
				case 3: // 都修改
					cfg.BaseURL = inputNewBaseURL()
					hostname = extractHost(cfg.BaseURL)
					cfg.AuthToken = inputNewAuthToken(hostname)
				case 4: // 修改模型名
					cfg.Model = runL2Menu("默认模型", cfg.Model)
				}
				fmt.Println()
				continue
			}
			break
		}
		printSuccess("API 连接验证成功!")
	} else if configMode == 3 {
		// 解决 400 报错模式：无需任何输入，直接跳到保存
		printSectionHeader("修复 Claude Code 400 请求头错误")
		printInfo("禁用实验性请求头，解决 Claude Code 400 传入请求头错误问题")
		fmt.Println()
	} else if configMode == 4 {
		configureAgentTeams()
		return
	} else {
		// 仅配置模型模式
		if cfg.BaseURL == "" || cfg.AuthToken == "" {
			printWarning("未检测到现有的 URL 或 Token 配置")
			printInfo("将跳过 API 验证，仅配置模型")
		} else {
			printInfo("使用现有的 URL 和 Token 配置")
			fmt.Printf("  Base URL: %s\n", cfg.BaseURL)
			fmt.Printf("  Token: %s\n", maskToken(cfg.AuthToken))
		}
		fmt.Println()
	}

	// 配置模型（模式 3 直接跳过）
	if configMode != 3 {
		configureModels(&cfg)
	}

	// 保存配置（带动画）
	fmt.Println()
	err := runWithSpinner("正在保存配置...", func() error {
		return saveConfig(cfg)
	})
	if err != nil {
		printError(fmt.Sprintf("保存配置失败: %v", err))
		os.Exit(1)
	}
	printSuccess("保存成功!")

	// 打印摘要
	printSummary(cfg)

	// 等待用户退出
	fmt.Println()
	styledInput("按回车键退出")
}
