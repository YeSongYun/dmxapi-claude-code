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
	envOpusModel   = "ANTHROPIC_DEFAULT_OPUS_MODEL"

	// 默认模型值
	defaultModel       = "claude-opus-4-5-20251101-cc"
	defaultHaikuModel  = "claude-haiku-4-5-20251001-cc"
	defaultSonnetModel = "claude-sonnet-4-5-20250929-cc"
	defaultOpusModel   = "claude-opus-4-5-20251101-cc"
)

// 颜色代码
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
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
	printColor(colorGreen, "✓ "+text+"\n")
}

// printError 打印错误信息
func printError(text string) {
	printColor(colorRed, "✗ "+text+"\n")
}

// printWarning 打印警告信息
func printWarning(text string) {
	printColor(colorYellow, "⚠ "+text+"\n")
}

// printInfo 打印信息
func printInfo(text string) {
	printColor(colorCyan, "→ "+text+"\n")
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

// setEnvVar 设置环境变量（跨平台）
func setEnvVar(key, value string) error {
	// 设置当前进程的环境变量
	if err := os.Setenv(key, value); err != nil {
		return err
	}

	// 持久化到系统
	switch runtime.GOOS {
	case "windows":
		return setEnvVarWindows(key, value)
	default:
		return setEnvVarUnix(key, value)
	}
}

// setEnvVarWindows 在 Windows 上设置用户环境变量
func setEnvVarWindows(key, value string) error {
	// 使用 PowerShell 执行，避免 cmd 的转义问题
	psCmd := fmt.Sprintf(`[Environment]::SetEnvironmentVariable('%s', '%s', 'User')`,
		strings.ReplaceAll(key, "'", "''"),
		strings.ReplaceAll(value, "'", "''"))

	// 优先使用 PowerShell
	return runCommand("powershell", "-NoProfile", "-Command", psCmd)
}

// setEnvVarUnix 在 Unix 系统上设置环境变量
func setEnvVarUnix(key, value string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// 构建 export 语句
	exportLine := fmt.Sprintf("export %s='%s'\n", key, strings.ReplaceAll(value, "'", "'\\''"))

	// 确定要写入的配置文件
	var configFiles []string

	switch runtime.GOOS {
	case "darwin":
		// macOS: 优先 zsh，兼容 bash
		configFiles = []string{".zshrc", ".bash_profile"}
	default:
		// Linux: 优先 bashrc，兼容 profile
		configFiles = []string{".bashrc", ".profile"}
	}

	// 写入配置文件
	for _, configFile := range configFiles {
		configPath := filepath.Join(homeDir, configFile)

		// 检查文件是否存在
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		// 读取现有内容
		content, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		// 检查是否已存在该环境变量的设置
		marker := fmt.Sprintf("export %s=", key)
		lines := strings.Split(string(content), "\n")
		found := false
		newLines := make([]string, 0, len(lines))

		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), marker) {
				// 替换现有设置
				newLines = append(newLines, strings.TrimSuffix(exportLine, "\n"))
				found = true
			} else {
				newLines = append(newLines, line)
			}
		}

		if !found {
			// 添加到文件末尾
			newLines = append(newLines, strings.TrimSuffix(exportLine, "\n"))
		}

		// 写回文件
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
	fmt.Println()
	printInfo("配置 API 服务器地址")
	fmt.Println("  示例: https://www.dmxapi.cn")

	if existing != "" {
		fmt.Printf("  当前值: %s\n", existing)
		if !confirm("是否修改 Base URL?") {
			return existing
		}
	}

	for {
		input := readInput("请输入 Base URL: ")
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
	fmt.Println()
	printInfo("配置 API 认证令牌")

	if hostname != "" {
		fmt.Printf("  获取地址: https://%s/token\n", hostname)
	}

	if existing != "" {
		fmt.Println("  当前已配置 Token")
		if !confirm("是否更新 Token?") {
			return existing
		}
	}

	for {
		input := readInput("请输入 Auth Token: ")
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
// 返回值: 1 = 完整配置, 2 = 仅配置模型
func selectConfigMode() int {
	fmt.Println("请选择配置模式:")
	fmt.Println("  1. 完整配置 (配置 URL、Token 和模型)")
	fmt.Println("  2. 仅配置模型 (跳过 URL 和 Token 配置)")
	fmt.Println()

	for {
		input := readInput("请输入选项 (1/2): ")
		switch input {
		case "1":
			return 1
		case "2":
			return 2
		default:
			printError("无效选项，请输入 1 或 2")
		}
	}
}

// selectFixOption 让用户选择要修改的内容
func selectFixOption() int {
	fmt.Println("请选择要修改的内容:")
	fmt.Println("  1. URL 有问题")
	fmt.Println("  2. Key 有问题")
	fmt.Println("  3. 都有问题")

	for {
		input := readInput("请输入选项 (1/2/3): ")
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
		input := readInput("请输入新的 Base URL: ")
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
		input := readInput("请输入新的 Auth Token: ")
		if input == "" {
			printError("Token 不能为空")
			continue
		}
		return strings.TrimSpace(input)
	}
}

// configureModels 配置模型
func configureModels(cfg *Config) {
	fmt.Println()
	printInfo("配置模型设置")

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

	if !confirm("\n是否修改模型配置?") {
		return
	}

	// 逐个配置模型
	fmt.Println()
	input := readInput(fmt.Sprintf("默认模型 [%s]: ", cfg.Model))
	if input != "" {
		cfg.Model = input
	}

	input = readInput(fmt.Sprintf("Haiku 模型 [%s]: ", cfg.HaikuModel))
	if input != "" {
		cfg.HaikuModel = input
	}

	input = readInput(fmt.Sprintf("Sonnet 模型 [%s]: ", cfg.SonnetModel))
	if input != "" {
		cfg.SonnetModel = input
	}

	input = readInput(fmt.Sprintf("Opus 模型 [%s]: ", cfg.OpusModel))
	if input != "" {
		cfg.OpusModel = input
	}
}

// saveConfig 保存配置
func saveConfig(cfg Config) error {
	fmt.Println()
	printInfo("正在保存配置...")

	// 保存所有环境变量
	vars := map[string]string{
		envBaseURL:     cfg.BaseURL,
		envAuthToken:   cfg.AuthToken,
		envModel:       cfg.Model,
		envHaikuModel:  cfg.HaikuModel,
		envSonnetModel: cfg.SonnetModel,
		envOpusModel:   cfg.OpusModel,
	}

	for key, value := range vars {
		if value == "" {
			continue
		}
		if err := setEnvVar(key, value); err != nil {
			return fmt.Errorf("设置 %s 失败: %v", key, err)
		}
	}

	return nil
}

// printSummary 打印配置摘要
func printSummary(cfg Config) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	printSuccess("配置完成!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()
	fmt.Printf("  %-35s = %s\n", envBaseURL, cfg.BaseURL)
	fmt.Printf("  %-35s = %s\n", envAuthToken, maskToken(cfg.AuthToken))
	fmt.Printf("  %-35s = %s\n", envModel, cfg.Model)
	fmt.Printf("  %-35s = %s\n", envHaikuModel, cfg.HaikuModel)
	fmt.Printf("  %-35s = %s\n", envSonnetModel, cfg.SonnetModel)
	fmt.Printf("  %-35s = %s\n", envOpusModel, cfg.OpusModel)
	fmt.Println()

	switch runtime.GOOS {
	case "windows":
		printInfo("配置已保存到用户环境变量")
		printInfo("请重新打开终端窗口使配置生效")
	default:
		printInfo("配置已保存到 shell 配置文件")
		printInfo("请运行 'source ~/.bashrc' 或重新打开终端使配置生效")
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
	// 显示欢迎信息
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	printColor(colorCyan, fmt.Sprintf("  %s 配置工具\n", appName))
	fmt.Println(strings.Repeat("=", 50))

	// 检测系统信息
	fmt.Printf("  系统: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

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

	// 配置模型
	configureModels(&cfg)

	// 保存配置
	if err := saveConfig(cfg); err != nil {
		printError(fmt.Sprintf("保存配置失败: %v", err))
		os.Exit(1)
	}

	// 打印摘要
	printSummary(cfg)
}
