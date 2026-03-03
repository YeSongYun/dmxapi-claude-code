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
	"runtime"
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

	// 默认模型值
	defaultModel       = "claude-sonnet-4-6-cc"
	defaultHaikuModel  = "claude-haiku-4-5-20251001-cc"
	defaultSonnetModel = "claude-sonnet-4-6-cc"
	defaultOpusModel   = "claude-opus-4-6-cc"

	fixedDisableExperimentalBetas = "1"
)

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
	appVersion = "1.0.0"
	// 统一盒子内容宽度（不含左右边框字符）
	boxWidth = 60
)

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
	if runtime.GOOS == "windows" {
		// Windows 下尝试启用 ANSI 颜色支持
		fmt.Print(color + text + colorReset)
	} else {
		fmt.Print(color + text + colorReset)
	}
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
	done := make(chan bool)
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

	fmt.Print("\r" + strings.Repeat(" ", 70) + "\r")
	return err
}

// ==================== 终端 UI 组件 ====================

// visibleLength 计算字符串在终端中的可见宽度（ANSI 感知 + CJK 双宽度）
func visibleLength(s string) int {
	inEscape := false
	count := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		// CJK 统一汉字区间，占 2 格
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0xFF00 && r <= 0xFFEF) ||
			(r >= 0x3000 && r <= 0x303F) {
			count += 2
		} else {
			count++
		}
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

// styledConfirm 带样式提示符的确认（y/N）
func styledConfirm(label string) bool {
	fmt.Printf("  %s?%s %s%s%s %s(y/N):%s ",
		colorBrightYellow, colorReset, styleBold, label, colorReset, styleDim, colorReset)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
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
			// setx KEY "VALUE" 设置用户环境变量
			if err := runCommand("setx", k, v); err != nil {
				errChan <- fmt.Errorf("设置环境变量 %s 失败: %v", k, err)
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

	// 确定要写入的配置文件
	var configFiles []string
	switch runtime.GOOS {
	case "darwin":
		configFiles = []string{".zshrc", ".bash_profile"}
	default:
		configFiles = []string{".bashrc", ".profile"}
	}

	// 写入配置文件
	for _, configFile := range configFiles {
		configPath := filepath.Join(homeDir, configFile)

		// macOS 特殊处理：如果是 .zshrc 且不存在，则创建
		if runtime.GOOS == "darwin" && configFile == ".zshrc" {
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				// 创建空的 .zshrc 文件
				if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
					return fmt.Errorf("创建 %s 失败: %v", configPath, err)
				}
			}
		} else {
			// 其他文件：不存在则跳过
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				continue
			}
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		newLines := make([]string, 0, len(lines))
		foundKeys := make(map[string]bool)

		// 遍历现有行，替换已存在的变量
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

		// 添加未找到的变量
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
func validateAPIConnection(baseURL, authToken string) error {
	// 构建测试请求 URL
	testURL := strings.TrimSuffix(baseURL, "/") + "/v1/messages"

	// 创建一个简单的测试请求体
	requestBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
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
		return fmt.Errorf("API 端点不存在: 请检查 Base URL 是否正确")
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
		fmt.Println("  当前已配置 Token")
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
// 返回值: 1 = 完整配置, 2 = 仅配置模型, 3 = 仅禁用 Betas
func selectConfigMode() int {
	printMenu("配置模式选择", []MenuItem{
		{"1", "完整配置", "配置 URL、Token 和模型"},
		{"2", "仅配置模型", "跳过 URL 和 Token 配置"},
		{"3", "仅禁用 Betas", "设置 DISABLE_EXPERIMENTAL_BETAS=1"},
	})
	fmt.Println()

	for {
		input := styledInput("选项")
		switch input {
		case "1":
			return 1
		case "2":
			return 2
		case "3":
			return 3
		default:
			printError("无效选项，请输入 1、2 或 3")
		}
	}
}

// selectFixOption 让用户选择要修改的内容
func selectFixOption() int {
	printMenu("选择要修改的内容", []MenuItem{
		{"1", "修改 URL", "Base URL 有问题"},
		{"2", "修改 Key", "API Key 有问题"},
		{"3", "都修改", "URL 和 Key 都有问题"},
	})
	fmt.Println()

	for {
		input := styledInput("选项")
		switch input {
		case "1":
			return 1
		case "2":
			return 2
		case "3":
			return 3
		default:
			printError("无效选项，请输入 1、2 或 3")
		}
	}
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

// configureModels 配置模型
func configureModels(cfg *Config) {
	printSectionHeader("配置模型设置")

	// 设置默认值
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

	fmt.Println()
	fmt.Println("当前模型配置:")
	fmt.Printf("  %-35s = %s\n", envModel, cfg.Model)
	fmt.Printf("  %-35s = %s\n", envHaikuModel, cfg.HaikuModel)
	fmt.Printf("  %-35s = %s\n", envSonnetModel, cfg.SonnetModel)
	fmt.Printf("  %-35s = %s\n", envOpusModel, cfg.OpusModel)

	if !styledConfirm("是否修改模型配置") {
		return
	}

	// 逐个配置模型
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
	case "darwin":
		printTip("配置已写入 ~/.zshrc 和 ~/.bash_profile")
		printTip("执行 source ~/.zshrc 或重启终端使配置生效")
	default:
		printTip("配置已写入 ~/.bashrc 和 ~/.profile")
		printTip("执行 source ~/.bashrc 或重启终端使配置生效")
	}
}

// maskToken 遮盖 Token
func maskToken(token string) string {
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// ==================== 主程序 ====================

func main() {
	// 显示 Logo
	printLogo()

	// 选择配置模式
	configMode := selectConfigMode()

	// 加载现有配置
	cfg := loadExistingConfig()

	// 根据配置模式执行不同流程
	if configMode == 1 {
		// 完整配置模式
		// 配置 Base URL
		cfg.BaseURL = getNewBaseURL(cfg.BaseURL)

		// 提取主机名用于提示
		hostname := extractHost(cfg.BaseURL)

		// 配置 Auth Token
		cfg.AuthToken = getNewAuthToken(cfg.AuthToken, hostname)

		// 验证 API 连接（循环直到成功）
		fmt.Println()
		for {
			if err := validateAPIConnection(cfg.BaseURL, cfg.AuthToken); err != nil {
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
				}
				fmt.Println()
				continue
			}
			break
		}
		printSuccess("API 连接验证成功!")
	} else if configMode == 3 {
		// 仅禁用 Betas 模式：无需任何输入，直接跳到保存
		printSectionHeader("快速配置 Disable Betas")
		printInfo("将写入 CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1，其余配置保持不变")
		fmt.Println()
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
