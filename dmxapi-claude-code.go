// claude-cli-setup.go - Interactive setup for Anthropic Claude Code CLI
// 跨平台配置工具，支持 Windows/Linux/macOS

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
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
	envBaseURL                  = "ANTHROPIC_BASE_URL"
	envAuthToken                = "ANTHROPIC_AUTH_TOKEN"
	envModel                    = "ANTHROPIC_MODEL"
	envHaikuModel               = "ANTHROPIC_DEFAULT_HAIKU_MODEL"
	envSonnetModel              = "ANTHROPIC_DEFAULT_SONNET_MODEL"
	envOpusModel                = "ANTHROPIC_DEFAULT_OPUS_MODEL"
	envDisableExperimentalBetas = "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"
	envAgentTeams               = "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"
	envEffortLevel              = "CLAUDE_CODE_EFFORT_LEVEL"
	// defaultEffortLevel 启用 Effort Level 时写入的默认值。
	// 取 max：按用户明确要求，三处（系统环境变量 CLAUDE_CODE_EFFORT_LEVEL、settings.json env 块、
	// settings.json 顶层 effortLevel）统一强制写入 max。
	// 注意：max 在 Claude Code 中通常被视为 session 级——顶层 effortLevel 官方仅接受
	// low/medium/high/xhigh，故顶层写入的 max 可能被官方忽略；此为用户知悉风险后的明确选择。
	defaultEffortLevel = "max"

	// VSCode settings.json 中写入配置所用的键名（claudeCode 为扩展 package.json 中定义的配置前缀）
	vscodeEnvKey = "claudeCode.environmentVariables"
	// vscodeEnvKeyOld 为旧版工具使用的错误键名，仅用于向后兼容检测，不再写入
	vscodeEnvKeyOld = "claude-code.environmentVariables"

	// Claude Code settings.json 中写入环境变量所用的键名
	claudeSettingsEnvKey = "env"

	// Claude Code settings.json 顶层 effortLevel 字段名（持久化思考等级的镜像，
	// 与 env 块的 CLAUDE_CODE_EFFORT_LEVEL 保持同值）
	claudeSettingsEffortLevelKey = "effortLevel"

	// Claude Code settings.json 顶层 attribution 对象及其子键名（控制 git commit / PR 署名）
	claudeSettingsAttributionKey = "attribution"
	attributionCommitKey         = "commit"
	attributionPRKey             = "pr"

	// 默认模型值
	defaultModel       = "claude-sonnet-4-6-cc"
	defaultHaikuModel  = "claude-haiku-4-5-20251001-cc"
	defaultSonnetModel = "claude-sonnet-4-6-cc"
	defaultOpusModel   = "claude-opus-4-6-cc"

	// dmxapi 推荐配置（一键模式使用）
	recommendedBaseURL     = "https://www.dmxapi.cn"
	recommendedModel       = "claude-opus-4-8-cc"
	recommendedHaikuModel  = "claude-haiku-4-5-20251001-cc"
	recommendedSonnetModel = "claude-sonnet-4-6-cc"
	recommendedOpusModel   = "claude-opus-4-8-cc"

	// recommendedAttributionText 新手流程默认 git 署名（写入 settings.json 顶层 attribution）
	recommendedAttributionText = "Generated with dmxapi"

	fixedDisableExperimentalBetas = "1"
)

// presetModel 描述二级菜单中一个预设模型条目：ID 用于写入配置，Hint 仅用于显示。
type presetModel struct {
	ID   string
	Hint string
}

var presetModels = []presetModel{
	{"claude-opus-4-8-cc", "3.4 折"},
	{"claude-opus-4-8", "6.8 折"},
	{"claude-opus-4-8-ssvip", ""},
	{"claude-opus-4-7-cc", "3.4 折"},
	{"claude-opus-4-7", "6.8 折"},
	{"claude-opus-4-7-ssvip", ""},
	{"claude-haiku-4-5-20251001-cc", "3.4 折"},
	{"claude-haiku-4-5-20251001", "6.8 折"},
	{"claude-haiku-4-5-20251001-ssvip", ""},
	{"claude-sonnet-4-6-cc", "3.4 折"},
	{"claude-sonnet-4-6", "6.8 折"},
	{"claude-sonnet-4-6-ssvip", ""},
	{"glm-5.1-cc", ""},
	{"qwen3.6-plus-cc", ""},
	{"mimo-v2-pro-cc", ""},
	{"MiniMax-M2.7-cc", ""},
	{"DeepSeek-V3.2-cc", ""},
	{"hunyuan-2.0-thinking-20251109-cc", ""},
}

// allEnvVarKeys 本工具管理的所有环境变量名，清除时使用
var allEnvVarKeys = []string{
	envBaseURL,
	envAuthToken,
	envModel,
	envHaikuModel,
	envSonnetModel,
	envOpusModel,
	envDisableExperimentalBetas,
	envAgentTeams,
	envEffortLevel,
}

// attributionManagedKeys 本工具管理的 attribution 子键名，清除时使用（不动用户自填的其他子键）
var attributionManagedKeys = []string{
	attributionCommitKey,
	attributionPRKey,
}

// validTopEffortLevels 决定哪些 effort 值会被本工具镜像写入 settings.json 顶层 effortLevel 字段。
// 官方实际仅接受 low/medium/high/xhigh；max 是 session 级、顶层官方不接受、可能被 Claude Code 忽略，
// 但本工具按用户明确要求强制把 max 一并写入顶层（与 env 块、系统环境变量三处保持同值）。
// 用于 mergeClaudeEffortLevel 判定是否写顶层。
var validTopEffortLevels = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
	"max":    true, // 非官方：按用户要求强制把 max 也写入顶层 effortLevel
}

// 版本号 / 盒子宽度保持 const（运行时不会变）
const (
	appVersion = "1.8.1"
	boxWidth   = 60
)

// 颜色代码（改为 var 以便 applyLegacyTheme 在无 VT 的 Windows 控制台上置空）
var (
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
)

// 视觉元素（图标 / 盒形 / spinner），legacy 模式下替换为 ASCII 等价物
var (
	iconSuccess = "✔"
	iconError   = "✘"
	iconWarn    = "⚠"
	iconInfo    = "→"
	iconTip     = "◆"
	// 以下 3 个图标出现在对齐盒子内，必须用宽度恒为 1 列的 ASCII：
	// ❯ ✓ ✏ 等是 East Asian Ambiguous 字符，终端实际渲染宽度随字体而变，
	// 会与 visibleLength 的计算不一致导致右边框错位（见 isAmbiguousWidth 说明）。
	iconPrompt  = ">"
	iconCheck   = "*"
	iconEdit    = "+"
	iconNavUp   = "↑"
	iconNavDown = "↓"

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	// 双线盒（printBox 用）
	boxDTL, boxDTR, boxDBL, boxDBR = "╔", "╗", "╚", "╝"
	boxDH, boxDV                   = "═", "║"
	boxDML, boxDMR                 = "╠", "╣"

	// 单线圆角盒（菜单用）
	boxTL, boxTR, boxBL, boxBR = "╭", "╮", "╰", "╯"
	boxH, boxV                 = "─", "│"
	boxML, boxMR               = "├", "┤"

	// 节头前缀（printSectionHeader 用）
	sectionStart = "┌─"
)

// cjkAmbiguous 为 true 时，Unicode East Asian Ambiguous 宽度字符按 2 宽度渲染（CJK locale 习惯）
var cjkAmbiguous bool

// applyLegacyTheme 在无 VT 支持的控制台（如 Windows 7/8/老 Win10）下，
// 把所有颜色置空、盒形/图标/spinner 换为 ASCII 等价物，避免打印乱码或方框。
func applyLegacyTheme() {
	colorReset, colorRed, colorGreen, colorYellow, colorBlue, colorCyan = "", "", "", "", "", ""
	colorBrightRed, colorBrightGreen, colorBrightYellow = "", "", ""
	colorBrightBlue, colorBrightMagenta, colorBrightCyan, colorBrightWhite = "", "", "", ""
	colorMagenta, colorWhite = "", ""
	styleBold, styleDim = "", ""

	boxDTL, boxDTR, boxDBL, boxDBR = "+", "+", "+", "+"
	boxDH, boxDV = "=", "|"
	boxDML, boxDMR = "+", "+"
	boxTL, boxTR, boxBL, boxBR = "+", "+", "+", "+"
	boxH, boxV = "-", "|"
	boxML, boxMR = "+", "+"
	sectionStart = "+-"

	iconSuccess = "[OK]"
	iconError = "[X]"
	iconWarn = "[!]"
	iconInfo = "->"
	iconTip = "*"
	iconPrompt = ">"
	iconCheck = "*"
	iconEdit = ">"
	iconNavUp = "^"
	iconNavDown = "v"

	spinnerFrames = []string{"|", "/", "-", "\\"}
}

// detectCJKLocale 判断当前用户环境是否为 CJK locale（中日韩），用于 Ambiguous Width 渲染。
// 优先检查 LC_ALL / LC_CTYPE / LANG；Windows 补充检测系统活动代码页。
func detectCJKLocale() bool {
	for _, envName := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := strings.ToLower(os.Getenv(envName))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "zh") || strings.HasPrefix(v, "ja") || strings.HasPrefix(v, "ko") {
			return true
		}
		// 只要确认有具体 locale 就以此为准，不再尝试 Windows ACP
		return false
	}
	// 所有 env 变量都为空才 fallback 到 Windows ACP
	if runtime.GOOS == "windows" {
		switch getWindowsACP() {
		case 936, 950, 932, 949:
			return true
		}
	}
	return false
}

// isAmbiguousWidth 判断一个 rune 是否落在 Unicode East Asian Ambiguous 宽度区间
// （常用段覆盖 ◆ ❯ ✔ ↑↓ 等符号）。非详尽，但覆盖本工具所用字符。
func isAmbiguousWidth(r rune) bool {
	switch {
	case r >= 0x2190 && r <= 0x21FF: // 箭头 ←→↑↓
		return true
	case r >= 0x25A0 && r <= 0x25FF: // 几何形状 ◆◇○●
		return true
	case r >= 0x2600 && r <= 0x26FF: // 杂项符号 ☀★⚠
		return true
	case r >= 0x2700 && r <= 0x27BF: // Dingbats ✔✘❯✂
		return true
	}
	return false
}

// rawModeState 保存终端 raw 模式前的状态，用于 Ctrl+C 时恢复
var rawModeState *term.State

// Config 存储所有配置项
type Config struct {
	BaseURL     string `json:"baseUrl"`
	AuthToken   string `json:"authToken"`
	Model       string `json:"model"`
	HaikuModel  string `json:"haikuModel"`
	SonnetModel string `json:"sonnetModel"`
	OpusModel   string `json:"opusModel"`
}

// Attribution 表示 Claude Code settings.json 顶层 attribution 对象的三态配置，
// 用于自定义/关闭 git commit 与 PR 的署名。
// 字段为 *string：nil=不管理(保留 Claude 默认署名)；指向 ""=关闭署名；指向文本=自定义署名。
// omitempty 对指针仅在 nil 时省略；指向 "" 时仍序列化为 "commit": ""，正好表达“关闭”。
type Attribution struct {
	Commit *string `json:"commit,omitempty"`
	PR     *string `json:"pr,omitempty"`
}

// NamedConfig 命名配置的完整快照，持久化到 ~/.DMXAPI/claude_code/<name>.json。
// 内嵌 Config 使字段提升，且 nc.Config 可直接传给 saveConfig / applyNamedConfig。
type NamedConfig struct {
	Config
	Name        string       `json:"name"`
	AgentTeams  string       `json:"agentTeams,omitempty"`  // 快照时的开关值（"1" 或 ""）
	EffortLevel string       `json:"effortLevel,omitempty"` // 如 "max" 或 ""
	Attribution *Attribution `json:"attribution,omitempty"` // nil=该配置不管理 git 署名
	SavedAt     string       `json:"savedAt,omitempty"`     // RFC3339
	AppVersion  string       `json:"appVersion,omitempty"`  // 写入时的 appVersion
	FilePath    string       `json:"-"`                     // 运行期填充，不序列化
}

// clearResult 记录单个位置的清除结果
type clearResult struct {
	Location string // 位置描述，如 "~/.zshrc"
	Status   string // "success" | "skipped" | "failed"
	Message  string // 详细信息
	Err      error  // 错误信息（如有）
}

// ==================== 工具函数 ====================

// printColor 打印带颜色的文本
func printColor(color, text string) {
	fmt.Print(color + text + colorReset)
}

// printSuccess 打印成功信息
func printSuccess(text string) {
	fmt.Printf("%s%s%s%s %s\n", colorReset, colorBrightGreen, iconSuccess, colorReset, text)
}

// printError 打印错误信息
func printError(text string) {
	fmt.Printf("%s%s%s%s %s%s%s\n", colorReset, colorBrightRed, iconError, colorReset, colorBrightRed, text, colorReset)
}

// printWarning 打印警告信息
func printWarning(text string) {
	fmt.Printf("%s%s%s%s %s%s%s\n", colorReset, colorBrightYellow, iconWarn, colorReset, colorBrightYellow, text, colorReset)
}

// printInfo 打印信息
func printInfo(text string) {
	fmt.Printf("%s%s%s%s %s\n", colorReset, colorBrightCyan, iconInfo, colorReset, text)
}

// runWithSpinner 带旋转动画执行任务
func runWithSpinner(message string, task func() error) error {
	done := make(chan bool, 1) // 带缓冲，防止 task panic 时 goroutine 阻塞
	var err error

	go func() {
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r  %s%s%s %s%s%s", styleBold+colorBrightCyan, spinnerFrames[i], colorReset, colorBrightWhite, message, colorReset)
				i = (i + 1) % len(spinnerFrames)
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
	// 在 CJK locale 下，East Asian Ambiguous 宽度字符按 2 宽度渲染（◆❯✔↑↓ 等）
	if cjkAmbiguous && isAmbiguousWidth(r) {
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
	fmt.Printf("\n%s%s%s %s%s%s\n", colorBrightBlue, sectionStart, colorReset, styleBold, title, colorReset)
}

// printTip 打印提示信息
func printTip(text string) {
	fmt.Printf("  %s%s%s %s\n", colorBrightBlue, iconTip, colorReset, text)
}

// printBox 打印双线边框盒子
func printBox(title, titleColor string, lines []string) {
	border := strings.Repeat(boxDH, boxWidth)
	fmt.Printf("%s%s%s\n", boxDTL, border, boxDTR)

	// 标题居中
	titleVisible := visibleLength(title)
	padding := boxWidth - titleVisible
	if padding < 0 {
		padding = 0 // title 超宽时不溢出 padding（避免 negative Repeat panic）
	}
	left := padding / 2
	right := padding - left
	fmt.Printf("%s%s%s%s%s%s%s\n",
		boxDV, strings.Repeat(" ", left), titleColor+styleBold, title, colorReset, strings.Repeat(" ", right), boxDV)

	fmt.Printf("%s%s%s\n", boxDML, border, boxDMR)

	for _, line := range lines {
		lineVisible := visibleLength(line)
		pad := boxWidth - lineVisible - 2 // 2 for leading spaces
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("%s  %s%s%s\n", boxDV, line, strings.Repeat(" ", pad), boxDV)
	}

	fmt.Printf("%s%s%s\n", boxDBL, border, boxDBR)
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
	KeyUp KeyType = iota
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
	border := strings.Repeat(boxH, boxWidth)
	fmt.Printf("%s%s%s\n", boxTL, border, boxTR)

	// 标题居中
	titleVisible := visibleLength(title)
	padding := boxWidth - titleVisible
	if padding < 0 {
		padding = 0 // title 超宽时不溢出 padding（避免 negative Repeat panic）
	}
	left := padding / 2
	right := padding - left
	fmt.Printf("%s%s%s%s%s%s%s\n",
		boxV, strings.Repeat(" ", left), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", right), boxV)

	fmt.Printf("%s%s%s\n", boxML, border, boxMR)

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
		fmt.Printf("%s  %s%s%s\n", boxV, content, strings.Repeat(" ", pad), boxV)
	}

	fmt.Printf("%s%s%s\n", boxBL, border, boxBR)
}

// ==================== 样式输入函数 ====================

// styledInput 带样式提示符的文本输入
func styledInput(label string) string {
	fmt.Printf("  %s%s%s %s%s:%s ", colorBrightCyan, iconPrompt, colorReset, styleBold, label, colorReset)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil && input == "" {
		// stdin 已到 EOF 且无任何输入（如非交互管道/输入流关闭）：
		// 继续交互无意义，优雅退出，避免上层循环空转刷屏。
		exitOnInputEOF()
	}
	return strings.TrimSpace(input)
}

// isBackToken 判断文本输入是否为"返回上一步"指令（大小写不敏感）。
// 仅用于菜单降级路径与模型名输入，绝不用于配置名称步骤。
func isBackToken(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	return t == "b" || t == "back"
}

// styledInputWithBack 带返回功能的文本输入。
// back=true 表示用户要返回上一步：优先识别 ESC 键（终端 raw 模式），并兼容输入 b/back 文字
// （Windows / 非终端降级路径）；否则返回 (已 TrimSpace 的输入, false)。
// 空串语义（保留现值）仍由调用方按 value=="" 判断。
func styledInputWithBack(label string) (value string, back bool) {
	in, esc := styledInputWithEsc(label) // raw 模式：ESC 即返回；已含退格/UTF-8/EOF 处理
	if esc {
		return "", true
	}
	if isBackToken(in) { // 兼容：Windows / 非终端降级时仍可输入 b/back 返回
		return "", true
	}
	return in, false
}

// styledInputWithEsc 带样式提示符的单行文本输入，支持按 ESC 返回上一步（esc=true）。
// 用于配置名称这类“自由文本但需可返回”的步骤：名称可能恰好是 b/back，故不能用
// isBackToken 文本指令判断，必须以 ESC 键作为返回信号。
//
// 仅在 raw 模式可用的非 Windows 终端下提供 ESC 返回；Windows / 非终端 /
// enterRawMode 失败时降级为普通 styledInput（不支持返回，回车正常前进）。
// 降级原因见 console_windows.go：readConsoleKey 丢弃 UnicodeChar，无法用于文本输入。
func styledInputWithEsc(label string) (value string, esc bool) {
	// Windows 无法复用现有 raw 抽象读取字符，直接降级（不支持返回）
	if runtime.GOOS == "windows" {
		return styledInput(label), false
	}
	// 先打印提示符（含 Esc 返回提示），再进入 raw 模式逐字节读取
	fmt.Printf("  %s%s%s %s%s:%s %sEsc 返回%s ",
		colorBrightCyan, iconPrompt, colorReset, styleBold, label, colorReset, styleDim, colorReset)

	restore, err := enterRawMode()
	if err != nil {
		// 降级：raw 模式不可用（非终端 / legacy 控制台）。提示符已打印，直接读整行。
		reader := bufio.NewReader(os.Stdin)
		in, rerr := reader.ReadString('\n')
		if rerr != nil && in == "" {
			exitOnInputEOF()
		}
		return strings.TrimSpace(in), false
	}

	var buf []byte
	one := make([]byte, 1)
	for {
		n, rerr := os.Stdin.Read(one)
		if n == 0 || rerr != nil {
			restore()
			fmt.Print("\r\n")
			// 无更多输入：与 styledInput 的 EOF 行为对齐
			if len(buf) == 0 {
				exitOnInputEOF()
			}
			return strings.TrimSpace(string(buf)), false
		}
		switch b := one[0]; b {
		case 0x1B: // ESC：单独按下视为返回；后随转义序列（方向键等）则忽略
			if stdinBytesAvailable() == 0 {
				restore()
				fmt.Print("\r\n")
				return "", true
			}
			rest := make([]byte, 2)
			os.Stdin.Read(rest) // 读掉并忽略后续转义字节
		case 0x0D, 0x0A: // 回车：完成输入
			restore()
			fmt.Print("\r\n")
			return strings.TrimSpace(string(buf)), false
		case 0x03: // Ctrl+C：与 readRawKey 一致，恢复终端后退出
			if rawModeState != nil {
				term.Restore(int(syscall.Stdin), rawModeState)
				fmt.Println()
			}
			restoreConsole()
			os.Exit(130)
		case 0x7F, 0x08: // 退格：按 rune 删除末尾（避免删半个多字节字符）
			if len(buf) > 0 {
				runes := []rune(string(buf))
				last := runes[len(runes)-1]
				runes = runes[:len(runes)-1]
				buf = []byte(string(runes))
				// 按被删字符的显示宽度回退对应列数（中文占 2 列）
				for i := 0; i < runeWidth(last); i++ {
					fmt.Print("\b \b")
				}
			}
		default:
			if b >= 0x20 { // 可打印字节：累积并回显。逐字节读取+原样写出天然兼容 UTF-8
				buf = append(buf, b)
				os.Stdout.Write(one) // 按字节回显，终端自行组装多字节字符
			}
		}
	}
}

// exitOnInputEOF 在标准输入到达 EOF 无法继续交互时，恢复终端并退出程序。
func exitOnInputEOF() {
	if rawModeState != nil {
		term.Restore(int(syscall.Stdin), rawModeState)
	}
	restoreConsole()
	fmt.Println()
	printError("输入已结束，已退出")
	os.Exit(1)
}

// styledPassword 带样式提示符的隐藏输入
func styledPassword(label string) string {
	fmt.Printf("  %s%s%s %s%s:%s ", colorBrightCyan, iconPrompt, colorReset, styleBold, label, colorReset)
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
	input, err := reader.ReadString('\n')
	if err != nil && input == "" {
		exitOnInputEOF()
	}
	return strings.TrimSpace(input)
}

// styledConfirm 带样式提示符的确认菜单
// allowBack=true 时支持 ESC / b 返回上一步（back=true）。
func styledConfirm(label string, allowBack bool) (confirmed bool, back bool) {
	return runConfirmMenu(label, allowBack)
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

// ==================== 文件写入 ====================

// writeFileAtomic 原子写入：先写临时文件，fsync 后 rename 替换目标。
// 防止写入中途被打断（Ctrl+C / OOM / 断电）导致目标文件被截断甚至清空。
// 失败时保证原目标文件内容不变。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// 任一失败都清理临时文件
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// describeWriteError 根据错误类型返回用户友好的写入失败描述。
// 权限问题附加可操作的排查建议。
func describeWriteError(err error) string {
	if err == nil {
		return "写入失败"
	}
	if errors.Is(err, fs.ErrPermission) {
		return "写入失败（权限不足；检查 ls -l 与 lsattr +i 锁定状态）"
	}
	return "写入失败"
}

// describeWindowsRegError 针对 Windows 注册表写入/删除失败给出可操作建议。
func describeWindowsRegError() string {
	return "（若企业域策略/组策略推送过该变量，登出重登后可能被恢复）"
}

// ==================== JSONC 处理 ====================

// trailingCommaRe 匹配 JSON 中 } 或 ] 前的尾随逗号
var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

// stripJSONC 将 JSONC（VS Code settings.json 格式）转换为合法 JSON。
// 支持剥离 // 单行注释、/* */ 块注释、以及尾随逗号。
// 使用字符级状态机正确处理字符串内容（不误删字符串中的 // 或 ,）。
func stripJSONC(data []byte) []byte {
	var buf bytes.Buffer
	inString := false
	i := 0
	for i < len(data) {
		c := data[i]
		if inString {
			buf.WriteByte(c)
			if c == '\\' && i+1 < len(data) {
				// 转义字符：原样写入下一个字节
				i++
				buf.WriteByte(data[i])
			} else if c == '"' {
				inString = false
			}
			i++
			continue
		}
		// 以下均为字符串外
		if c == '"' {
			inString = true
			buf.WriteByte(c)
			i++
			continue
		}
		// 单行注释 //
		if c == '/' && i+1 < len(data) && data[i+1] == '/' {
			i += 2
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}
		// 块注释 /* ... */
		if c == '/' && i+1 < len(data) && data[i+1] == '*' {
			i += 2
			for i+1 < len(data) {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		buf.WriteByte(c)
		i++
	}
	// 最后剥离尾随逗号
	return trailingCommaRe.ReplaceAll(buf.Bytes(), []byte("$1"))
}

// skipJSONCWhitespace 跳过空白和 JSONC 注释，返回新的位置
func skipJSONCWhitespace(data []byte, i int) int {
	for i < len(data) {
		c := data[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if c == '/' && i+1 < len(data) && data[i+1] == '/' {
			i += 2
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(data) && data[i+1] == '*' {
			i += 2
			for i+1 < len(data) {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		break
	}
	return i
}

// skipJSONCString 从 data[i]（必须是 "）开始跳过字符串，返回闭合引号后的位置
func skipJSONCString(data []byte, i int) int {
	if i >= len(data) || data[i] != '"' {
		return i
	}
	i++
	for i < len(data) {
		c := data[i]
		if c == '\\' && i+1 < len(data) {
			i += 2
			continue
		}
		if c == '"' {
			return i + 1
		}
		i++
	}
	return i
}

// skipJSONCValue 从 JSONC 值起点跳到值末尾（不含尾部空白），支持嵌套对象/数组/字符串/字面量
func skipJSONCValue(data []byte, i int) int {
	i = skipJSONCWhitespace(data, i)
	if i >= len(data) {
		return i
	}
	switch data[i] {
	case '"':
		return skipJSONCString(data, i)
	case '{':
		depth := 1
		i++
		for i < len(data) && depth > 0 {
			i = skipJSONCWhitespace(data, i)
			if i >= len(data) {
				break
			}
			c := data[i]
			switch c {
			case '"':
				i = skipJSONCString(data, i)
			case '{', '[':
				depth++
				i++
			case '}', ']':
				depth--
				i++
			default:
				i++
			}
		}
		return i
	case '[':
		depth := 1
		i++
		for i < len(data) && depth > 0 {
			i = skipJSONCWhitespace(data, i)
			if i >= len(data) {
				break
			}
			c := data[i]
			switch c {
			case '"':
				i = skipJSONCString(data, i)
			case '{', '[':
				depth++
				i++
			case '}', ']':
				depth--
				i++
			default:
				i++
			}
		}
		return i
	default:
		// 字面量（number / true / false / null），跳到下一个分隔符
		for i < len(data) {
			c := data[i]
			if c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' {
				break
			}
			i++
		}
		return i
	}
}

// removeJSONCTopKey 删除 JSONC 对象内某个顶层键（保留所有注释和非目标键的原始格式）。
// objStart 指向 '{' 的位置；返回修改后的字节流和是否删除了该键。
//
// 删除边界策略（保证 JSON 结构仍合法）：
//   - 若目标键**不是最后一个**（后面接 `,`）：删除 `[keyNameStart, 逗号之后]`，保留前面的逗号和格式
//   - 若目标键**是最后一个**（后面接 `}`）：向前找到最近的 `,` 或 `{`，若是 `,` 则连同前导逗号一起删；若直接就是 `{` 则只删键本身
//
// 不吞前后空白/换行，避免破坏用户原本的缩进和注释对齐。
func removeJSONCTopKey(data []byte, objStart int, keyName string) (output []byte, removed bool) {
	if objStart >= len(data) || data[objStart] != '{' {
		return data, false
	}
	i := objStart + 1
	for i < len(data) {
		i = skipJSONCWhitespace(data, i)
		if i >= len(data) || data[i] == '}' {
			return data, false
		}
		if data[i] != '"' {
			return data, false
		}
		keyNameStart := i
		keyEnd := skipJSONCString(data, i)
		rawKey := string(data[keyNameStart:keyEnd])
		quotedKey := fmt.Sprintf("%q", keyName)
		match := rawKey == quotedKey
		i = keyEnd
		i = skipJSONCWhitespace(data, i)
		if i >= len(data) || data[i] != ':' {
			return data, false
		}
		i++ // 跳过 ':'
		valEnd := skipJSONCValue(data, i)
		if match {
			// 看值后面是逗号还是 '}'
			after := skipJSONCWhitespace(data, valEnd)
			delStart := keyNameStart
			delEnd := valEnd
			if after < len(data) && data[after] == ',' {
				// 目标键不是最后：一并吞掉尾部逗号，后续键仍有前导格式
				delEnd = after + 1
			} else {
				// 目标键是最后一个：尝试吞前导逗号
				j := keyNameStart - 1
				for j > objStart {
					c := data[j]
					if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
						j--
						continue
					}
					break
				}
				if j >= objStart && data[j] == ',' {
					delStart = j
				}
				// 前面就是 '{' 的情况：保持 delStart = keyNameStart
			}
			var buf bytes.Buffer
			buf.Write(data[:delStart])
			buf.Write(data[delEnd:])
			return buf.Bytes(), true
		}
		// 未匹配：跳到下一个键
		i = valEnd
		i = skipJSONCWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
			continue
		}
		return data, false
	}
	return data, false
}

// findTopLevelObjectRange 返回 JSONC data 里顶层 JSON 对象的起止位置（包含 '{' 和 '}'）。
// 返回 (start, end+1, true)。若未找到对象返回 (_, _, false)。
func findTopLevelObjectRange(data []byte) (int, int, bool) {
	i := skipJSONCWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return 0, 0, false
	}
	start := i
	end := skipJSONCValue(data, i)
	return start, end, true
}

// findNestedObjectStart 在顶层对象里查找某个键对应的嵌套对象值，返回该对象的 '{' 位置。
// 未找到返回 (-1, false)。
func findNestedObjectStart(data []byte, parentStart int, keyName string) (int, bool) {
	if parentStart >= len(data) || data[parentStart] != '{' {
		return -1, false
	}
	i := parentStart + 1
	for i < len(data) {
		i = skipJSONCWhitespace(data, i)
		if i >= len(data) || data[i] == '}' {
			return -1, false
		}
		if data[i] != '"' {
			return -1, false
		}
		keyNameStart := i
		keyEnd := skipJSONCString(data, i)
		rawKey := string(data[keyNameStart:keyEnd])
		quotedKey := fmt.Sprintf("%q", keyName)
		match := rawKey == quotedKey
		i = keyEnd
		i = skipJSONCWhitespace(data, i)
		if i >= len(data) || data[i] != ':' {
			return -1, false
		}
		i++
		i = skipJSONCWhitespace(data, i)
		if match {
			if i < len(data) && data[i] == '{' {
				return i, true
			}
			return -1, false
		}
		// 跳过值
		i = skipJSONCValue(data, i)
		i = skipJSONCWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
			continue
		}
		return -1, false
	}
	return -1, false
}

// removeJSONCTopKeys 批量删除 JSONC 文档里多个顶层键（保留注释）。返回修改后的字节流和删除计数。
func removeJSONCTopKeys(data []byte, keys []string) ([]byte, int) {
	count := 0
	out := data
	for _, k := range keys {
		start, _, ok := findTopLevelObjectRange(out)
		if !ok {
			break
		}
		newOut, removed := removeJSONCTopKey(out, start, k)
		if removed {
			out = newOut
			count++
		}
	}
	return out, count
}

// removeJSONCNestedKeys 删除 JSONC 顶层对象中 parentKey 嵌套对象内的多个子键（保留注释）。
// 返回修改后的字节流、被删子键数量和是否整个 parent 对象变空（调用方可据此决定是否连带删 parent 键）。
func removeJSONCNestedKeys(data []byte, parentKey string, childKeys []string) (output []byte, removed int, parentEmpty bool) {
	topStart, _, ok := findTopLevelObjectRange(data)
	if !ok {
		return data, 0, false
	}
	nestedStart, nestedOk := findNestedObjectStart(data, topStart, parentKey)
	if !nestedOk {
		return data, 0, false
	}
	out := data
	for _, k := range childKeys {
		// 每次删除后 offset 会变，重新定位 parent
		topS, _, ok := findTopLevelObjectRange(out)
		if !ok {
			break
		}
		ns, nok := findNestedObjectStart(out, topS, parentKey)
		if !nok {
			break
		}
		newOut, ok := removeJSONCTopKey(out, ns, k)
		if ok {
			out = newOut
			removed++
		}
		_ = nestedStart // silence
	}
	// 检查 parent 是否空
	topS, _, ok := findTopLevelObjectRange(out)
	if ok {
		ns, nok := findNestedObjectStart(out, topS, parentKey)
		if nok {
			i := ns + 1
			i = skipJSONCWhitespace(out, i)
			if i < len(out) && out[i] == '}' {
				parentEmpty = true
			}
		}
	}
	return out, removed, parentEmpty
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
		configFiles := []string{".zshrc"}
		if goos == "darwin" {
			configFiles = append(configFiles, ".zprofile")
		}
		return shellProfile{
			configFiles: configFiles,
			sourceCmd:   "source ~/.zshrc",
		}
	case strings.Contains(shell, "fish"):
		return shellProfile{
			configFiles: []string{".config/fish/config.fish"},
			sourceCmd:   "", // fish universal 变量（set -Ux）立即在所有会话生效，无需 source
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
			return shellProfile{configFiles: []string{".zshrc", ".zprofile", ".bash_profile"}}
		}
		return shellProfile{configFiles: []string{".bashrc", ".profile"}}
	}
}

// allUnixCandidateConfigFiles 返回所有可能包含 ANTHROPIC_* 变量的 Unix shell 配置文件（相对 HOME）。
// 清除操作使用此集合，与 $SHELL 的登录 shell 无关——因为用户可能同时用多种 shell。
func allUnixCandidateConfigFiles() []string {
	return []string{
		".zshrc", ".zshenv", ".zprofile", ".zlogin",
		".bashrc", ".bash_profile", ".bash_login", ".profile",
		".config/fish/config.fish",
		".kshrc",
		".cshrc",
		".tcshrc",
	}
}

// setEnvVarsWindows 在 Windows 上批量设置用户环境变量（写入后立即校验）
func setEnvVarsWindows(vars map[string]string) error {
	for key, value := range vars {
		if value == "" {
			continue
		}
		if err := setAndVerifyUserEnv(key, value); err != nil {
			return err
		}
	}
	return nil
}

type userEnvGetter func(string) (string, bool, error)

type userEnvSetter func(string, string) error

type userEnvRemover func(string) error

func setAndVerifyUserEnv(key, value string) error {
	return setAndVerifyUserEnvWithOps(key, value, setUserEnv, getUserEnv)
}

func setAndVerifyUserEnvWithOps(key, value string, setFn userEnvSetter, getFn userEnvGetter) error {
	if err := setFn(key, value); err != nil {
		return fmt.Errorf("设置环境变量 %s 失败: %v", key, err)
	}
	actual, exists, err := getFn(key)
	if err != nil {
		return fmt.Errorf("校验环境变量 %s 失败: %v", key, err)
	}
	if !exists {
		return fmt.Errorf("校验环境变量 %s 失败: 变量未写入 Windows 用户环境变量", key)
	}
	if actual != value {
		return fmt.Errorf("校验环境变量 %s 失败: 期望 %q，实际 %q", key, value, actual)
	}
	return nil
}

func setUserEnv(key, value string) error {
	if len(value) > 900 {
		// REG ADD 不自动广播 WM_SETTINGCHANGE，需要手工通知其他进程环境变量已变
		if err := runCommand("REG", "ADD", `HKCU\Environment`, "/V", key, "/T", "REG_SZ", "/D", value, "/F"); err != nil {
			return err
		}
		broadcastEnvironmentChange()
		return nil
	}
	// setx 会自动广播，无需额外通知
	return runCommand("setx", key, value)
}

func getUserEnv(key string) (string, bool, error) {
	cmd := exec.Command("REG", "QUERY", `HKCU\Environment`, "/V", key)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// REG QUERY 退出码 1 = 变量不存在（不依赖文字，兼容所有系统语言和编码）
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", false, nil
		}
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return "", false, fmt.Errorf("%v: %s", err, trimmed)
		}
		return "", false, err
	}
	return parseRegQueryValue(key, output)
}

func parseRegQueryValue(key string, output []byte) (string, bool, error) {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 3 || !strings.EqualFold(fields[0], key) {
			continue
		}
		parts := strings.SplitN(trimmed, fields[1], 2)
		if len(parts) != 2 {
			return "", false, fmt.Errorf("未能从 REG QUERY 输出中解析 %s", key)
		}
		return strings.TrimLeft(parts[1], " \t"), true, nil
	}
	return "", false, fmt.Errorf("未能从 REG QUERY 输出中解析 %s", key)
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
						newLines = append(newLines, fmt.Sprintf("set -Ux %s '%s'", key, strings.ReplaceAll(value, "'", "'\\''")))
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
				newLines = append(newLines, fmt.Sprintf("set -Ux %s '%s'", key, strings.ReplaceAll(value, "'", "'\\''")))
			}
			newContent := strings.Join(newLines, "\n")
			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}
			if err := writeFileAtomic(configPath, []byte(newContent), 0644); err != nil {
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
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		if err := writeFileAtomic(configPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("写入 %s 失败: %v", configPath, err)
		}
	}

	return nil
}

// ==================== VSCode 插件配置 ====================

// vscodeSettingsPathFor 根据平台参数返回 VSCode settings.json 的绝对路径（纯函数，便于测试）。
// wslWindowsHome 非空时表示 WSL 环境，使用 Windows 侧路径。
func vscodeSettingsPathFor(goos, homeDir, appData, wslWindowsHome string) string {
	switch {
	case wslWindowsHome != "":
		// WSL：写入 Windows 侧 AppData（使用 filepath.Join 处理 Unix 风格路径）
		return filepath.Join(wslWindowsHome, "AppData", "Roaming", "Code", "User", "settings.json")
	case goos == "windows":
		// Windows：需要保持反斜杠，用 \\ 连接
		return strings.Join([]string{appData, "Code", "User", "settings.json"}, `\`)
	case goos == "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "settings.json")
	default:
		// linux 及其他
		return filepath.Join(homeDir, ".config", "Code", "User", "settings.json")
	}
}

// applyModelSuffix 对 claude-opus-4-8、claude-opus-4-7 和 claude-sonnet-4-6 系列模型 ID 追加 [1m] 后缀。
func applyModelSuffix(id string) string {
	if strings.HasSuffix(id, "[1m]") {
		return id
	}
	if strings.Contains(id, "claude-opus-4-8") || strings.Contains(id, "claude-opus-4-7") || strings.Contains(id, "claude-sonnet-4-6") {
		return id + "[1m]"
	}
	return id
}

// stripModelSuffix 去掉模型 ID 末尾的 [1m] 后缀，用于发起测试请求时还原真实模型名。
func stripModelSuffix(id string) string {
	return strings.TrimSuffix(id, "[1m]")
}

// buildManagedEnvMap 根据 Config 构建本工具管理的环境变量集合（纯函数）。
// agentTeamsVal 为空时不写入 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS。
func buildManagedEnvMap(cfg Config, agentTeamsVal string) map[string]string {
	vars := map[string]string{
		envBaseURL:                  cfg.BaseURL,
		envAuthToken:                cfg.AuthToken,
		envModel:                    applyModelSuffix(cfg.Model),
		envHaikuModel:               applyModelSuffix(cfg.HaikuModel),
		envSonnetModel:              applyModelSuffix(cfg.SonnetModel),
		envOpusModel:                applyModelSuffix(cfg.OpusModel),
		envDisableExperimentalBetas: fixedDisableExperimentalBetas,
	}
	if agentTeamsVal != "" {
		vars[envAgentTeams] = agentTeamsVal
	}
	if effortLevelVal := getManagedEffortLevelValue(); effortLevelVal != "" {
		vars[envEffortLevel] = effortLevelVal
	}
	return vars
}

// buildVSCodeEnvVars 根据 Config 构建 claudeCode.environmentVariables 数组（纯函数）。
// agentTeamsVal 为空时不写入 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS。
func buildVSCodeEnvVars(cfg Config, agentTeamsVal string) []map[string]string {
	managed := buildManagedEnvMap(cfg, agentTeamsVal)
	keys := []string{
		envBaseURL,
		envAuthToken,
		envModel,
		envHaikuModel,
		envSonnetModel,
		envOpusModel,
		envDisableExperimentalBetas,
	}
	if agentTeamsVal != "" {
		keys = append(keys, envAgentTeams)
	}
	if getManagedEffortLevelValue() != "" {
		keys = append(keys, envEffortLevel)
	}

	entries := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, map[string]string{
			"name":  key,
			"value": managed[key],
		})
	}
	return entries
}

// mergeVSCodeSettings 将 envVars 写入现有 JSON 的 claudeCode.environmentVariables 键，
// 保留所有其他键。existingJSON 必须是合法 JSON 对象。
// 返回格式化后的 JSON 字节（2空格缩进）。
func mergeVSCodeSettings(existingJSON []byte, envVars []map[string]string) ([]byte, error) {
	cleaned := stripJSONC(existingJSON)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return nil, fmt.Errorf("解析 settings.json 失败: %v", err)
	}
	settings[vscodeEnvKey] = envVars
	return json.MarshalIndent(settings, "", "  ")
}

// claudeSettingsPathFor 返回用户级 ~/.claude/settings.json 的绝对路径（纯函数，便于测试）。
func claudeSettingsPathFor(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "settings.json")
}

// getClaudeSettingsPath 返回当前用户 Claude Code settings.json 的绝对路径。
func getClaudeSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法确定 Claude settings.json 路径: %v", err)
	}
	return claudeSettingsPathFor(homeDir), nil
}

// dmxapiConfigDirFor 返回命名配置持久化目录 ~/.DMXAPI/claude_code（纯函数，便于测试）。
func dmxapiConfigDirFor(homeDir string) string {
	return filepath.Join(homeDir, ".DMXAPI", "claude_code")
}

// dmxapiConfigDir 返回当前用户的命名配置持久化目录。
func dmxapiConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法确定命名配置目录路径: %v", err)
	}
	return dmxapiConfigDirFor(homeDir), nil
}

// windowsReservedNames 是 Windows 下不允许作为文件名的保留设备名（不区分大小写）。
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// sanitizeConfigFileName 将用户输入的配置名转为安全的文件名主干（纯函数）。
// 防路径穿越、过滤跨平台非法字符、保留 CJK、规避 Windows 保留名、限制长度。
func sanitizeConfigFileName(name string) string {
	name = strings.TrimSpace(name)
	// 折叠路径穿越片段
	name = strings.ReplaceAll(name, "..", "")
	// 过滤控制字符与跨平台非法字符
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20:
			b.WriteRune('_')
		case strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	// 去掉首尾的点和空格（Windows 不允许结尾点/空格）
	cleaned = strings.Trim(cleaned, " .")
	if cleaned == "" {
		return "config"
	}
	// 规避 Windows 保留设备名
	if windowsReservedNames[strings.ToLower(cleaned)] {
		cleaned = "_" + cleaned
	}
	// 按 rune 截断到 80，规避文件名长度上限
	runes := []rune(cleaned)
	if len(runes) > 80 {
		cleaned = string(runes[:80])
	}
	return cleaned
}

// namedConfigFileName 返回命名配置对应的文件名（含扩展名，纯函数）。
func namedConfigFileName(name string) string {
	return sanitizeConfigFileName(name) + ".json"
}

// listNamedConfigsIn 列出指定目录下的所有命名配置，按 Name 升序。
// 目录不存在返回空切片；非 .json 文件与解析失败的文件跳过。
func listNamedConfigsIn(dir string) ([]NamedConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var configs []NamedConfig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		nc, err := readNamedConfig(path)
		if err != nil {
			continue // 跳过坏文件，保证健壮
		}
		configs = append(configs, nc)
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Name < configs[j].Name
	})
	return configs, nil
}

// saveNamedConfigIn 将命名配置写入指定目录，返回写入的文件路径。
func saveNamedConfigIn(dir string, nc NamedConfig) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(nc, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, namedConfigFileName(nc.Name))
	if err := writeFileAtomic(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

// deleteAllNamedConfigsIn 删除指定目录下所有 .json 命名配置，返回删除数量。
func deleteAllNamedConfigsIn(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// readNamedConfig 从文件读取单个命名配置，并填充运行期 FilePath 字段。
func readNamedConfig(path string) (NamedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return NamedConfig{}, err
	}
	var nc NamedConfig
	if err := json.Unmarshal(data, &nc); err != nil {
		return NamedConfig{}, err
	}
	nc.FilePath = path
	return nc, nil
}

// deleteNamedConfig 删除指定路径的命名配置文件。
func deleteNamedConfig(path string) error {
	return os.Remove(path)
}

// listNamedConfigs 列出当前用户保存的所有命名配置（目录不存在返回空切片）。
func listNamedConfigs() ([]NamedConfig, error) {
	dir, err := dmxapiConfigDir()
	if err != nil {
		return nil, err
	}
	return listNamedConfigsIn(dir)
}

// saveNamedConfig 将命名配置保存到当前用户的持久化目录，返回写入路径。
func saveNamedConfig(nc NamedConfig) (string, error) {
	dir, err := dmxapiConfigDir()
	if err != nil {
		return "", err
	}
	return saveNamedConfigIn(dir, nc)
}

// deleteAllNamedConfigs 删除当前用户的所有命名配置，返回删除数量。
func deleteAllNamedConfigs() (int, error) {
	dir, err := dmxapiConfigDir()
	if err != nil {
		return 0, err
	}
	return deleteAllNamedConfigsIn(dir)
}

// removeManagedEnvVar 同时从当前进程与系统持久层移除指定环境变量。
func removeManagedEnvVar(key string) error {
	os.Unsetenv(key)
	if runtime.GOOS == "windows" {
		return removeEnvVarWindows(key)
	}
	return removeEnvVarUnix(key)
}

// mergeClaudeSettings 将本工具管理的环境变量写入 Claude Code settings.json 的 env 键，保留其他设置和其他 env 键。
func mergeClaudeSettings(existingJSON []byte, managedEnv map[string]string) ([]byte, error) {
	cleaned := stripJSONC(existingJSON)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return nil, fmt.Errorf("解析 Claude settings.json 失败: %v", err)
	}

	envMap := map[string]interface{}{}
	if existingEnv, ok := settings[claudeSettingsEnvKey]; ok {
		existingMap, ok := existingEnv.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Claude settings.json 中 env 不是对象")
		}
		for key, value := range existingMap {
			envMap[key] = value
		}
	}

	for _, key := range allEnvVarKeys {
		delete(envMap, key)
	}
	for key, value := range managedEnv {
		if value == "" {
			continue
		}
		envMap[key] = value
	}

	if len(envMap) == 0 {
		delete(settings, claudeSettingsEnvKey)
	} else {
		settings[claudeSettingsEnvKey] = envMap
	}

	return json.MarshalIndent(settings, "", "  ")
}

// clearClaudeSettingsManagedKeys 从给定 JSON 中移除本工具管理的 env 键与顶层 attribution（commit/pr）。
// 返回清理后的 JSON、是否实际移除了配置以及错误。
// 通过基于字符串的定点删除保留用户 settings.json 中的 // 注释与尾随逗号。
func clearClaudeSettingsManagedKeys(existingJSON []byte) ([]byte, bool, error) {
	// 先解析确认是否存在受管键（没有就早退）
	cleaned := stripJSONC(existingJSON)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return nil, false, fmt.Errorf("解析 Claude settings.json 失败: %v", err)
	}

	// 检测 env 受管键命中
	envHit := false
	if existingEnv, ok := settings[claudeSettingsEnvKey]; ok {
		envMap, ok := existingEnv.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("Claude settings.json 中 env 不是对象")
		}
		for _, key := range allEnvVarKeys {
			if _, exists := envMap[key]; exists {
				envHit = true
				break
			}
		}
	}

	// 检测顶层 attribution 受管子键（commit/pr）命中
	attrHit := false
	if attrRaw, ok := settings[claudeSettingsAttributionKey]; ok {
		if attrMap, ok := attrRaw.(map[string]interface{}); ok {
			for _, key := range attributionManagedKeys {
				if _, exists := attrMap[key]; exists {
					attrHit = true
					break
				}
			}
		}
	}

	if !envHit && !attrHit {
		return nil, false, nil
	}

	output := existingJSON

	// 定点删除 env 对象里的受管键（保留注释）
	if envHit {
		var envBecameEmpty bool
		output, _, envBecameEmpty = removeJSONCNestedKeys(output, claudeSettingsEnvKey, allEnvVarKeys)
		if envBecameEmpty {
			output, _ = removeJSONCTopKeys(output, []string{claudeSettingsEnvKey})
		}
		// 顶层 effortLevel 是 env 块 CLAUDE_CODE_EFFORT_LEVEL 的持久化镜像，搭车在 envHit 下清除。
		// 不作独立触发器：仅当本工具确实写过 env（envHit）时才清，避免误删用户用 /effort 自设的顶层值。
		output, _ = removeJSONCTopKeys(output, []string{claudeSettingsEffortLevelKey})
	}

	// 定点删除 attribution 对象里的受管子键（保留注释）
	if attrHit {
		var attrBecameEmpty bool
		output, _, attrBecameEmpty = removeJSONCNestedKeys(output, claudeSettingsAttributionKey, attributionManagedKeys)
		if attrBecameEmpty {
			output, _ = removeJSONCTopKeys(output, []string{claudeSettingsAttributionKey})
		}
	}

	return output, true, nil
}

// mergeClaudeAttribution 在已是标准 JSON（非 JSONC）的 settings 字节流上写入顶层 attribution 三态。
// attr 中 nil 字段表示“不管理该子键”（从 settings 中删除），指向值（含 ""）表示写入该值。
// 两子键都不存在时删除整个 attribution 顶层键。本函数经 MarshalIndent 重排，不保留注释，
// 与 mergeClaudeSettings 的 env 写入路径行为一致。
func mergeClaudeAttribution(jsonBytes []byte, attr Attribution) ([]byte, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &settings); err != nil {
		return nil, fmt.Errorf("解析 Claude settings.json 失败: %v", err)
	}
	if settings == nil {
		settings = map[string]interface{}{}
	}

	attrMap := map[string]interface{}{}
	if existing, ok := settings[claudeSettingsAttributionKey]; ok {
		if existingMap, ok := existing.(map[string]interface{}); ok {
			for k, v := range existingMap {
				attrMap[k] = v
			}
		}
		// 非对象（容错）：视为空，下面按 attr 重建
	}

	if attr.Commit != nil {
		attrMap[attributionCommitKey] = *attr.Commit
	} else {
		delete(attrMap, attributionCommitKey)
	}
	if attr.PR != nil {
		attrMap[attributionPRKey] = *attr.PR
	} else {
		delete(attrMap, attributionPRKey)
	}

	if len(attrMap) == 0 {
		delete(settings, claudeSettingsAttributionKey)
	} else {
		settings[claudeSettingsAttributionKey] = attrMap
	}

	return json.MarshalIndent(settings, "", "  ")
}

func saveClaudeSettingsConfigWithAgentTeams(cfg Config, agentTeamsVal string) error {
	return saveClaudeSettingsConfigWithAttribution(cfg, agentTeamsVal, getManagedAttribution())
}

// mergeClaudeEffortLevel 在已是标准 JSON（非 JSONC）的 settings 字节流上同步顶层 effortLevel 字段，
// 使其与 env 块的 CLAUDE_CODE_EFFORT_LEVEL 保持同值。入参先经 normalizeEffortLevel 归一：
//   - 归一后 ∈ validTopEffortLevels（low/medium/high/xhigh/max）→ 写入顶层 effortLevel
//   - 归一后为空 → 删除顶层 effortLevel
//   - 其他（如 auto，顶层不接受但 env 合法）→ 保守不动，避免误删用户值
//
// 本函数经 MarshalIndent 重排，不保留注释，与 mergeClaudeSettings/mergeClaudeAttribution 路径一致。
func mergeClaudeEffortLevel(jsonBytes []byte, effort string) ([]byte, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &settings); err != nil {
		return nil, fmt.Errorf("解析 Claude settings.json 失败: %v", err)
	}
	if settings == nil {
		settings = map[string]interface{}{}
	}

	normalized := normalizeEffortLevel(effort)
	switch {
	case validTopEffortLevels[normalized]:
		settings[claudeSettingsEffortLevelKey] = normalized
	case normalized == "":
		delete(settings, claudeSettingsEffortLevelKey)
	default:
		// auto 等顶层不接受的值：不改动顶层字段（既不写也不删）
	}

	return json.MarshalIndent(settings, "", "  ")
}

// saveClaudeSettingsConfigWithAttribution 写入 env 受管键、顶层 attribution 三态与顶层 effortLevel。
// 现有 env 合并逻辑复用 mergeClaudeSettings，attribution 由 mergeClaudeAttribution 处理，
// 顶层 effortLevel 由 mergeClaudeEffortLevel 与 env 块同步。
func saveClaudeSettingsConfigWithAttribution(cfg Config, agentTeamsVal string, attr Attribution) error {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("创建 Claude 配置目录失败: %v", err)
	}

	existingJSON := []byte("{}")
	if data, err := os.ReadFile(settingsPath); err == nil {
		existingJSON = data
	}

	managed := buildManagedEnvMap(cfg, agentTeamsVal)
	merged, err := mergeClaudeSettings(existingJSON, managed)
	if err != nil {
		return err
	}
	merged, err = mergeClaudeAttribution(merged, attr)
	if err != nil {
		return err
	}
	// 顶层 effortLevel 与 env 块 CLAUDE_CODE_EFFORT_LEVEL 取同值（managed 已含归一后的值）。
	merged, err = mergeClaudeEffortLevel(merged, managed[envEffortLevel])
	if err != nil {
		return err
	}
	return writeFileAtomic(settingsPath, append(merged, '\n'), 0644)
}

// saveClaudeSettingsConfig 将 cfg 写入 Claude Code settings.json 的 env 键。
func saveClaudeSettingsConfig(cfg Config) error {
	return saveClaudeSettingsConfigWithAgentTeams(cfg, getManagedAgentTeamsValue())
}

// clearEffortFromClaudeSettings 从 ~/.claude/settings.json 删除 effort 受管痕迹：
// env 块的 CLAUDE_CODE_EFFORT_LEVEL 与顶层 effortLevel 字段，保留其他受管键、
// 用户其他 env 键及 JSONC 注释/格式。
// 用于禁用 Effort Level：避免走 saveClaudeSettings 的合并写回逻辑（会回读旧值再写回，导致删不干净）。
// 早退条件为 env 与顶层均未命中（保持幂等，避免累积尾随换行）；任一命中即写回。
func clearEffortFromClaudeSettings() error {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil // 文件不存在视为已无该键，幂等成功
	}

	output, removedEnv, envEmpty := removeJSONCNestedKeys(data, claudeSettingsEnvKey, []string{envEffortLevel})
	if envEmpty {
		output, _ = removeJSONCTopKeys(output, []string{claudeSettingsEnvKey})
	}
	output, removedTop := removeJSONCTopKeys(output, []string{claudeSettingsEffortLevelKey})

	if removedEnv == 0 && removedTop == 0 {
		return nil // env 与顶层都没有 effort 痕迹，无需改写文件
	}

	// 规范化为恰好一个尾随换行：removeJSONC* 已保留原文件结尾，这里不能无条件再追加。
	output = append(bytes.TrimRight(output, "\n"), '\n')
	return writeFileAtomic(settingsPath, output, 0644)
}

// clearClaudeSettingsConfig 从 Claude Code settings.json 中移除本工具写入的 env 键。
func clearClaudeSettingsConfig() clearResult {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return clearResult{Location: "Claude settings.json", Status: "skipped", Message: "无法确定路径"}
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return clearResult{Location: settingsPath, Status: "skipped", Message: "文件不存在或无法读取"}
	}

	output, removed, err := clearClaudeSettingsManagedKeys(data)
	if err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: "JSON 处理失败", Err: err}
	}
	if !removed {
		return clearResult{Location: settingsPath, Status: "skipped", Message: "未找到相关配置"}
	}
	output = append(output, '\n')
	if err := writeFileAtomic(settingsPath, output, 0644); err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: describeWriteError(err), Err: err}
	}
	return clearResult{Location: settingsPath, Status: "success", Message: "已移除受管 env 配置"}
}

// isClaudeSettingsConfigured 检测 JSON 内容是否含本工具管理的 Claude env 键。
// 注意：刻意不纳入顶层 effortLevel —— 该字段可能由用户 /effort 命令自设，
// 纳入会把"从未用本工具配过"误判为"已配置"。顶层 effortLevel 由 effort 专用
// 启用/禁用路径管理，不参与此通用检测（清除时则搭车在 env 受管键命中下处理）。
func isClaudeSettingsConfigured(data []byte) bool {
	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return false
	}
	existingEnv, ok := settings[claudeSettingsEnvKey]
	if !ok {
		return false
	}
	envMap, ok := existingEnv.(map[string]interface{})
	if !ok {
		return false
	}
	for _, key := range allEnvVarKeys {
		if _, exists := envMap[key]; exists {
			return true
		}
	}
	return false
}

type loadedClaudeSettings struct {
	Config
	AgentTeams string
	// EffortLevel 来自 env 块的 CLAUDE_CODE_EFFORT_LEVEL。
	EffortLevel string
	// TopLevelEffortLevel 来自 settings.json 顶层 effortLevel 字段（持久化镜像，可能由
	// /effort 命令或本工具写入）。作为 effort 真值源的最末回退。
	TopLevelEffortLevel string
}

// loadConfigFromClaudeSettings 从 Claude Code settings.json 中读取本工具管理的配置。
// 顶层 effortLevel 的读取独立于 env 块：即使没有 env 块，也需返回已读到的顶层值。
func loadConfigFromClaudeSettings() loadedClaudeSettings {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return loadedClaudeSettings{}
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return loadedClaudeSettings{}
	}

	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return loadedClaudeSettings{}
	}

	result := loadedClaudeSettings{}

	// 顶层 effortLevel 独立读取，不受 env 块存在与否影响。
	if topVal, ok := settings[claudeSettingsEffortLevelKey].(string); ok {
		result.TopLevelEffortLevel = topVal
	}

	existingEnv, ok := settings[claudeSettingsEnvKey]
	if !ok {
		return result // 无 env 块：仍保留已读到的顶层 effortLevel
	}
	envMap, ok := existingEnv.(map[string]interface{})
	if !ok {
		return result
	}

	getString := func(key string) string {
		value, ok := envMap[key]
		if !ok {
			return ""
		}
		if s, ok := value.(string); ok {
			return s
		}
		return ""
	}

	result.Config = Config{
		BaseURL:     getString(envBaseURL),
		AuthToken:   getString(envAuthToken),
		Model:       getString(envModel),
		HaikuModel:  getString(envHaikuModel),
		SonnetModel: getString(envSonnetModel),
		OpusModel:   getString(envOpusModel),
	}
	result.AgentTeams = getString(envAgentTeams)
	result.EffortLevel = getString(envEffortLevel)
	return result
}

// getManagedAgentTeamsValue 返回受管的 Agent Teams 配置值：优先系统环境变量，缺失时回退 Claude settings。
func getManagedAgentTeamsValue() string {
	if value := getEnvVar(envAgentTeams); value != "" {
		return value
	}
	return loadConfigFromClaudeSettings().AgentTeams
}

// normalizeEffortLevel 归一思考等级值，确保非法/历史值不向下游传播。
// 本工具早期把启用值写成 "ultracode"，但该值既不是 CLAUDE_CODE_EFFORT_LEVEL 接受的等级，
// 也无法持久化到 settings；这里折叠为其推理深度等价的 "xhigh"。其余值（含 max/auto/""）原样返回。
func normalizeEffortLevel(level string) string {
	if level == "ultracode" {
		return "xhigh"
	}
	return level
}

// getManagedEffortLevelValue 返回受管的 Effort Level 配置值：优先系统环境变量，
// 缺失时回退 Claude settings 的 env 块，再缺失时回退顶层 effortLevel 字段。
// 返回前经 normalizeEffortLevel 归一，确保历史残留的 "ultracode" 统一显示/写出为 "xhigh"。
func getManagedEffortLevelValue() string {
	if value := getEnvVar(envEffortLevel); value != "" {
		return normalizeEffortLevel(value)
	}
	loaded := loadConfigFromClaudeSettings()
	if loaded.EffortLevel != "" {
		return normalizeEffortLevel(loaded.EffortLevel)
	}
	return normalizeEffortLevel(loaded.TopLevelEffortLevel)
}

// loadAttributionFromClaudeSettings 从 Claude Code settings.json 顶层 attribution 读取 git 署名配置。
// 子键存在（含空串）→ 指向该值；子键不存在 / attribution 缺失或非对象 → nil。
func loadAttributionFromClaudeSettings() Attribution {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return Attribution{}
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return Attribution{}
	}
	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return Attribution{}
	}
	attrRaw, ok := settings[claudeSettingsAttributionKey]
	if !ok {
		return Attribution{}
	}
	attrMap, ok := attrRaw.(map[string]interface{})
	if !ok {
		return Attribution{}
	}
	getPtr := func(key string) *string {
		v, ok := attrMap[key]
		if !ok {
			return nil
		}
		if s, ok := v.(string); ok {
			return &s
		}
		return nil
	}
	return Attribution{
		Commit: getPtr(attributionCommitKey),
		PR:     getPtr(attributionPRKey),
	}
}

// getManagedAttribution 返回受管的 git 署名配置。attribution 仅存于 settings.json（不进环境变量）。
func getManagedAttribution() Attribution {
	return loadAttributionFromClaudeSettings()
}

// clearAttributionFromClaudeSettings 仅从 ~/.claude/settings.json 顶层 attribution 删除指定子键，
// 保留其他子键、其他顶层键及 JSONC 注释/格式。子键全删后移除整个 attribution 顶层键。
// fields 取 ["commit"] / ["pr"] / ["commit","pr"]。用于“恢复 Claude 默认署名”。
func clearAttributionFromClaudeSettings(fields []string) error {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil // 文件不存在视为已无该键，幂等成功
	}
	output, removed, parentEmpty := removeJSONCNestedKeys(data, claudeSettingsAttributionKey, fields)
	if removed == 0 {
		return nil // 本就没有相关子键，无需改写（保持幂等）
	}
	if parentEmpty {
		output, _ = removeJSONCTopKeys(output, []string{claudeSettingsAttributionKey})
	}
	output = append(bytes.TrimRight(output, "\n"), '\n')
	return writeFileAtomic(settingsPath, output, 0644)
}

// winPathToWSL 将 Windows 路径（如 C:\Users\alice）转换为 WSL 挂载路径（/mnt/c/Users/alice）。
// 注意：不使用 filepath.ToSlash，因为 filepath.ToSlash 在 Linux/WSL 宿主上不转换反斜杠。
func winPathToWSL(winPath string) string {
	winPath = strings.TrimSpace(winPath)
	if len(winPath) < 3 || winPath[1] != ':' {
		return ""
	}
	drive := strings.ToLower(string(winPath[0]))
	rest := strings.ReplaceAll(winPath[2:], "\\", "/")
	return "/mnt/" + drive + rest
}

// getWindowsHomeFromWSL 在 WSL 环境中获取 Windows 用户目录对应的 WSL 路径。
// 优先调用 cmd.exe；失败时回退到扫描 /mnt/c/Users/ 中的第一个非系统用户目录。
func getWindowsHomeFromWSL() string {
	// 方法1：调用 cmd.exe /c echo %USERPROFILE%
	cmd := exec.Command("cmd.exe", "/c", "echo %USERPROFILE%")
	if out, err := cmd.Output(); err == nil {
		if p := winPathToWSL(string(out)); p != "" {
			return p
		}
	}

	// 方法2：扫描 /mnt/c/Users/ 取第一个非系统目录
	usersDir := "/mnt/c/Users"
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return ""
	}
	systemDirs := map[string]bool{
		"Default": true, "Public": true, "All Users": true,
		"Default User": true, "desktop.ini": true,
	}
	for _, e := range entries {
		if e.IsDir() && !systemDirs[e.Name()] {
			return filepath.Join(usersDir, e.Name())
		}
	}
	return ""
}

// getVSCodeSettingsPath 返回当前系统 VSCode settings.json 的绝对路径。
func getVSCodeSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	appData := os.Getenv("APPDATA")
	if appData == "" && runtime.GOOS == "windows" {
		// 回退：用 homeDir 拼出 AppData\Roaming
		appData = filepath.Join(homeDir, "AppData", "Roaming")
	}

	wslWindowsHome := ""
	if isWSL() {
		wslWindowsHome = getWindowsHomeFromWSL()
	}

	path := vscodeSettingsPathFor(runtime.GOOS, homeDir, appData, wslWindowsHome)
	if path == "" {
		return "", fmt.Errorf("无法确定 VSCode settings.json 路径")
	}
	return path, nil
}

// saveVSCodeConfig 将 cfg 写入 VSCode settings.json 的 claudeCode.environmentVariables。
// 若文件不存在则自动创建；若 JSON 解析失败则询问用户是否备份重建。
func saveVSCodeConfig(cfg Config) error {
	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 读取现有内容（不存在则用空对象）
	existingJSON := []byte("{}")
	if data, err := os.ReadFile(settingsPath); err == nil {
		existingJSON = data
	}

	agentTeamsVal := getManagedAgentTeamsValue()
	envVars := buildVSCodeEnvVars(cfg, agentTeamsVal)

	merged, err := mergeVSCodeSettings(existingJSON, envVars)
	if err != nil {
		// JSON 解析失败：询问是否备份重建
		printError(fmt.Sprintf("settings.json 解析失败: %v", err))
		if ok, _ := styledConfirm("是否备份原文件并重新创建", false); !ok {
			return fmt.Errorf("用户取消：保留原文件，跳过写入")
		}
		backupPath := settingsPath + ".bak"
		if berr := os.Rename(settingsPath, backupPath); berr != nil {
			return fmt.Errorf("备份失败: %v", berr)
		}
		printInfo(fmt.Sprintf("原文件已备份至: %s", backupPath))
		merged, err = mergeVSCodeSettings([]byte("{}"), envVars)
		if err != nil {
			return err
		}
	}

	return writeFileAtomic(settingsPath, merged, 0644)
}

// clearVSCodeConfig 从 VSCode settings.json 中移除本工具写入的配置键。
// 仅删除 claudeCode.environmentVariables 和旧版 claude-code.environmentVariables 键，
// 保留用户的所有其他配置不变。
func clearVSCodeConfig() clearResult {
	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		return clearResult{Location: "VSCode settings.json", Status: "skipped", Message: "无法确定路径"}
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return clearResult{Location: settingsPath, Status: "skipped", Message: "文件不存在或无法读取"}
	}

	// 先用合法 JSON 快速检测是否需要处理
	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: "JSON 解析失败", Err: err}
	}
	_, hasNew := settings[vscodeEnvKey]
	_, hasOld := settings[vscodeEnvKeyOld]
	if !hasNew && !hasOld {
		return clearResult{Location: settingsPath, Status: "skipped", Message: "未找到相关配置"}
	}

	// 用基于字符串的定点删除保留用户的注释和尾随逗号
	output, _ := removeJSONCTopKeys(data, []string{vscodeEnvKey, vscodeEnvKeyOld})

	if err := writeFileAtomic(settingsPath, output, 0644); err != nil {
		return clearResult{Location: settingsPath, Status: "failed", Message: describeWriteError(err), Err: err}
	}

	return clearResult{Location: settingsPath, Status: "success", Message: "已移除配置键（保留其他内容与注释）"}
}

// shellConfigLocation 返回 shell 配置文件的展示名称。
func shellConfigLocation(configFile string) string {
	return "~/" + configFile
}

// fishSetLinePattern 匹配 fish 的 set 语句设置 key：
// set [-选项集]* KEY [值...]  后可跟可选行注释 `# ...`
// 选项集覆盖 U/g/x/e/l（universal/global/export/erase/local 各种组合）
// 示例：set -Ux KEY / set -gx KEY / set -x KEY / set -U KEY / set -Ue KEY / set KEY
func fishSetLineRe(key string) *regexp.Regexp {
	// 正则字面量：`^set(\s+-[UugxelL]+)*\s+{key}(\s|$|#)`
	escKey := regexp.QuoteMeta(key)
	return regexp.MustCompile(`^set(\s+-[UugxelL]+)*\s+` + escKey + `(\s|$|#)`)
}

// shellLineManagesEnvVar 判断一行 shell 配置是否在管理指定环境变量。
// isFish=true 时按 fish 语法匹配；此外还识别 csh/tcsh 的 setenv/unsetenv 语法。
func shellLineManagesEnvVar(line, key string, isFish bool) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}

	if isFish {
		return fishSetLineRe(key).MatchString(trimmed)
	}

	// POSIX 风格（bash/zsh/ksh/sh）
	patterns := []string{
		fmt.Sprintf("export %s=", key),
		fmt.Sprintf("declare -x %s=", key),
		fmt.Sprintf("typeset -x %s=", key),
		fmt.Sprintf("readonly %s=", key),
	}
	for _, pattern := range patterns {
		if strings.HasPrefix(trimmed, pattern) {
			return true
		}
	}

	// csh/tcsh: setenv KEY VALUE / unsetenv KEY
	if strings.HasPrefix(trimmed, fmt.Sprintf("setenv %s ", key)) ||
		strings.HasPrefix(trimmed, fmt.Sprintf("setenv %s\t", key)) ||
		trimmed == fmt.Sprintf("setenv %s", key) ||
		strings.HasPrefix(trimmed, fmt.Sprintf("unsetenv %s", key)) {
		return true
	}

	assignPrefix := key + "="
	if !strings.HasPrefix(trimmed, assignPrefix) {
		return false
	}

	remainder := strings.TrimSpace(trimmed[len(assignPrefix):])
	if remainder == "" {
		return false
	}
	return strings.Contains(remainder, "; export "+key) || strings.HasSuffix(remainder, " export "+key)
}

// removeEnvVarsUnixFromFile 从单个 Unix shell 配置文件中删除受管环境变量。
// 未命中任何受管行时不修改文件（保持 mtime 不变），避免干扰 git 追踪 dotfiles。
// 不再压缩用户原本的空行格式。
func removeEnvVarsUnixFromFile(configPath string, keys []string, isFish bool) (removedCount int, err error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return 0, nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}
	info, statErr := os.Stat(configPath)
	perm := os.FileMode(0644)
	if statErr == nil {
		perm = info.Mode()
	}

	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	newLines := make([]string, 0, len(lines))

	for _, line := range lines {
		managed := false
		for _, key := range keys {
			if shellLineManagesEnvVar(line, key, isFish) {
				removedCount++
				managed = true
				break
			}
		}
		if managed {
			continue
		}
		newLines = append(newLines, line)
	}

	// 未命中任何受管行：不重写文件，保持原 mtime
	if removedCount == 0 {
		return 0, nil
	}

	newContent := strings.Join(newLines, "\n")
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	if err := writeFileAtomic(configPath, []byte(newContent), perm); err != nil {
		return removedCount, fmt.Errorf("写入 %s 失败: %v", configPath, err)
	}
	return removedCount, nil
}

// clearWindowsRegistryFromWSL 通过 WSL interop 调用 reg.exe 清理 Windows 用户环境变量。
// 当在 WSL 内执行清除时，只清 Linux 侧 shell 配置会留下注册表里残留，导致 Windows 侧 cmd/VSCode 仍用旧值。
func clearWindowsRegistryFromWSL() clearResult {
	regExe, err := exec.LookPath("reg.exe")
	if err != nil {
		return clearResult{Location: "WSL → Windows 注册表", Status: "skipped", Message: "reg.exe 不可用（WSL interop 未启用或非 WSL 环境）"}
	}
	var failures []string
	removed := 0
	for _, key := range allEnvVarKeys {
		cmd := exec.Command(regExe, "DELETE", `HKCU\Environment`, "/V", key, "/F")
		output, err := cmd.CombinedOutput()
		if err == nil {
			removed++
			continue
		}
		// reg.exe 在变量不存在时返回退出码 1，不应视为失败
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %s", key, strings.TrimSpace(string(output))))
	}
	if len(failures) > 0 {
		return clearResult{
			Location: "WSL → Windows 注册表",
			Status:   "failed",
			Message:  fmt.Sprintf("%d 个变量清除失败\n      %s", len(failures), strings.Join(failures, "\n      ")),
		}
	}
	if removed == 0 {
		return clearResult{Location: "WSL → Windows 注册表", Status: "skipped", Message: "注册表未包含受管变量"}
	}
	return clearResult{Location: "WSL → Windows 注册表", Status: "success", Message: fmt.Sprintf("已移除 %d 个环境变量", removed)}
}

// clearFishUniversalVariables 通过 `fish -c "set -Ue KEY"` 真正清除 fish 的 universal 变量。
// fish 的 universal 变量存储在二进制 fish_variables 里，只删 config.fish 文本的 set -Ux 行不会影响它。
func clearFishUniversalVariables() clearResult {
	fishExe, err := exec.LookPath("fish")
	if err != nil {
		return clearResult{Location: "Fish universal 变量", Status: "skipped", Message: "未检测到 fish 可执行文件"}
	}
	var failures []string
	removed := 0
	for _, key := range allEnvVarKeys {
		// set -Ue KEY：erase universal variable
		cmd := exec.Command(fishExe, "-c", fmt.Sprintf("set -Ue %s", key))
		if output, err := cmd.CombinedOutput(); err == nil {
			removed++
		} else {
			trimmed := strings.TrimSpace(string(output))
			// fish 在变量不存在时也会返回非 0，但 stderr 为空或为 set: universal variable... 一类提示；跳过
			if trimmed == "" {
				continue
			}
			failures = append(failures, fmt.Sprintf("%s: %s", key, trimmed))
		}
	}
	if len(failures) > 0 {
		return clearResult{Location: "Fish universal 变量", Status: "failed", Message: strings.Join(failures, "；")}
	}
	if removed == 0 {
		return clearResult{Location: "Fish universal 变量", Status: "skipped", Message: "无受管 universal 变量"}
	}
	return clearResult{Location: "Fish universal 变量", Status: "success", Message: fmt.Sprintf("已 erase %d 个 universal 变量", removed)}
}

// clearAllConfig 清除所有配置。
// 显示摘要 → 二次确认 → 逐位置清除 → 显示报告。
// 返回 true 表示用户确认并执行了清除；false 表示用户取消。
func clearAllConfig() bool {
	printSectionHeader("清除所有配置")
	fmt.Println()

	// 显示清除摘要
	printInfo("将从以下位置清除所有 dmxapi 相关配置：")
	fmt.Println()
	switch runtime.GOOS {
	case "windows":
		fmt.Println("    • Windows 用户环境变量（注册表）")
	default:
		profile := detectShellProfile(runtime.GOOS)
		for _, f := range profile.configFiles {
			fmt.Printf("    • %s\n", shellConfigLocation(f))
		}
	}
	if path, err := getVSCodeSettingsPath(); err == nil {
		fmt.Printf("    • VSCode settings.json (%s)\n", path)
	}
	if path, err := getClaudeSettingsPath(); err == nil {
		fmt.Printf("    • Claude Code settings.json (%s)\n", path)
	}
	if dir, err := dmxapiConfigDir(); err == nil {
		fmt.Printf("    • 已保存的命名配置 (%s)\n", dir)
	}
	fmt.Println("    • 当前进程环境变量")
	fmt.Println()
	printInfo("涉及的环境变量：")
	for _, key := range allEnvVarKeys {
		fmt.Printf("    • %s\n", key)
	}
	fmt.Println()

	printWarning("此操作不可撤销，Auth Token 清除后需要重新获取")
	fmt.Println()

	if ok, _ := styledConfirm("确定要清除所有配置吗", false); !ok {
		fmt.Println()
		printInfo("已取消，未做任何更改")
		return false
	}

	fmt.Println()
	var results []clearResult

	switch runtime.GOOS {
	case "windows":
		var regFailures []string
		for _, key := range allEnvVarKeys {
			if err := removeEnvVarWindows(key); err != nil {
				regFailures = append(regFailures, fmt.Sprintf("%s: %v", key, err))
			}
		}
		if len(regFailures) > 0 {
			results = append(results, clearResult{
				Location: "Windows 注册表",
				Status:   "failed",
				Message:  fmt.Sprintf("%d 个变量清除失败 %s\n      %s", len(regFailures), describeWindowsRegError(), strings.Join(regFailures, "\n      ")),
			})
		} else {
			results = append(results, clearResult{
				Location: "Windows 注册表",
				Status:   "success",
				Message:  fmt.Sprintf("已移除 %d 个环境变量", len(allEnvVarKeys)),
			})
		}
	default:
		homeDir, err := os.UserHomeDir()
		if err != nil {
			results = append(results, clearResult{Location: "Shell 配置文件", Status: "failed", Message: "无法确定用户目录", Err: err})
			break
		}
		// 扫全集配置文件：用户可能同时用多个 shell，保存时只写 $SHELL 对应文件，
		// 但清除要覆盖所有可能的位置（包括 .zshenv、.bash_login、.config/fish/config.fish、.cshrc 等）。
		for _, configFile := range allUnixCandidateConfigFiles() {
			configPath := filepath.Join(homeDir, configFile)
			location := shellConfigLocation(configFile)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				continue // 不存在就不报告，避免长列表里都是 skipped
			} else if err != nil {
				results = append(results, clearResult{Location: location, Status: "failed", Message: "无法读取文件状态", Err: err})
				continue
			}
			isFish := strings.HasSuffix(configFile, "config.fish")
			removedCount, err := removeEnvVarsUnixFromFile(configPath, allEnvVarKeys, isFish)
			switch {
			case err != nil:
				results = append(results, clearResult{Location: location, Status: "failed", Message: "清理失败", Err: err})
			case removedCount > 0:
				results = append(results, clearResult{Location: location, Status: "success", Message: fmt.Sprintf("已移除 %d 个环境变量配置", removedCount)})
			default:
				// 文件存在但无受管配置：不记录，保持输出简洁
			}
		}
		// WSL 环境下尝试清 Windows 侧用户注册表（通过 interop 调用 reg.exe）
		if isWSL() {
			results = append(results, clearWindowsRegistryFromWSL())
		}
		// Fish universal 变量在 fish_variables 二进制里，仅清 config.fish 不够——直接用 fish 命令 erase
		results = append(results, clearFishUniversalVariables())
	}

	results = append(results, clearClaudeSettingsConfig())
	results = append(results, clearVSCodeConfig())

	persistentFailure := false
	for _, r := range results {
		if r.Status == "failed" {
			persistentFailure = true
			break
		}
	}

	for _, key := range allEnvVarKeys {
		os.Unsetenv(key)
	}
	processMsg := "已清除当前会话中的所有环境变量"
	if persistentFailure {
		processMsg = "已清除当前会话中的所有环境变量，但持久化配置仍有失败项"
	}
	results = append(results, clearResult{
		Location: "当前进程",
		Status:   "success",
		Message:  processMsg,
	})

	fmt.Println()
	printSectionHeader("清除结果")
	fmt.Println()
	for _, r := range results {
		switch r.Status {
		case "success":
			printSuccess(fmt.Sprintf("%s — %s", r.Location, r.Message))
		case "skipped":
			printInfo(fmt.Sprintf("%s — %s（已跳过）", r.Location, r.Message))
		case "failed":
			printError(fmt.Sprintf("%s — %s", r.Location, r.Message))
			if r.Err != nil {
				fmt.Printf("    %s%s%s\n", colorRed, r.Err.Error(), colorReset)
			}
		}
	}
	fmt.Println()
	if persistentFailure {
		printTip("当前会话环境变量已清除，但仍有持久化配置未清理成功，请按上方失败项继续检查")
	} else {
		printTip("重新打开终端后配置清除完全生效")
	}
	return true
}

// configureVSCode 模式5交互流程：展示将写入的配置，用户确认后写入 VSCode settings.json。
// exitOnDone=true 时末尾显示"按回车键退出"（独立运行模式5时使用）；
// 嵌入模式1后置步骤时传 false，由 main 统一处理退出。
// allowBack=true 时，写入确认按 ESC 返回上一步（back=true，静默返回不等待回车）。
func configureVSCode(cfg Config, exitOnDone, allowBack bool) (back bool) {
	printSectionHeader("配置 VSCode 插件")
	fmt.Println()

	settingsPath, err := getVSCodeSettingsPath()
	if err != nil {
		printError(fmt.Sprintf("无法确定 settings.json 路径: %v", err))
		if exitOnDone {
			fmt.Println()
			styledInput("按回车键退出")
		}
		return false
	}
	printInfo(fmt.Sprintf("目标文件: %s", settingsPath))
	fmt.Println()

	agentTeamsVal := getManagedAgentTeamsValue()
	envVars := buildVSCodeEnvVars(cfg, agentTeamsVal)

	if cfg.BaseURL == "" || cfg.AuthToken == "" {
		printWarning("未检测到 BaseURL 或 Token 配置，建议先运行「从头配置」")
		fmt.Println()
	}

	printInfo("将写入以下环境变量:")
	for _, v := range envVars {
		val := v["value"]
		if v["name"] == envAuthToken && len(val) > 8 {
			val = maskToken(val)
		}
		fmt.Printf("  %-45s = %s\n", v["name"], val)
	}
	fmt.Println()

	ok, back := styledConfirm("确认写入 VSCode settings.json", allowBack)
	if allowBack && back {
		return true // 静默返回上层，不走 exitOnDone 等回车
	}
	if !ok {
		printInfo("已取消")
		if exitOnDone {
			fmt.Println()
			styledInput("按回车键退出")
		}
		return false
	}

	fmt.Println()
	err = runWithSpinner("正在写入 VSCode 配置...", func() error {
		return saveVSCodeConfig(cfg)
	})
	if err != nil {
		printError(fmt.Sprintf("写入失败: %v", err))
	} else {
		printSuccess("VSCode 配置写入成功!")
		printInfo(fmt.Sprintf("文件路径: %s", settingsPath))
		if isWSL() {
			printTip("注意：已写入 Windows 侧 VSCode 配置，重启 VSCode 后生效")
		} else {
			printTip("重启 VSCode 后配置生效")
		}
	}

	if exitOnDone {
		fmt.Println()
		styledInput("按回车键退出")
	}
	return false
}

// removeEnvVarUnix 从 Unix shell 配置文件中删除指定环境变量（幂等）。
// 扫描全集候选文件（而不只是 $SHELL 的登录 shell），以清掉用户在其他 shell 配置里残留的值。
func removeEnvVarUnix(key string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	for _, configFile := range allUnixCandidateConfigFiles() {
		configPath := filepath.Join(homeDir, configFile)
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		isFish := strings.HasSuffix(configFile, "config.fish")
		if _, err := removeEnvVarsUnixFromFile(configPath, []string{key}, isFish); err != nil {
			return err
		}
	}
	return nil
}

// removeEnvVarWindows 从 Windows 用户环境变量中删除指定变量
func removeEnvVarWindows(key string) error {
	return removeAndVerifyUserEnvWithOps(key, removeUserEnv, getUserEnv)
}

func removeAndVerifyUserEnvWithOps(key string, removeFn userEnvRemover, getFn userEnvGetter) error {
	removeErr := removeFn(key)
	// 无论 removeFn 是否报错，都通过 getFn 验证最终状态
	_, exists, verifyErr := getFn(key)
	if verifyErr != nil {
		if removeErr != nil {
			return fmt.Errorf("删除 %s 失败: %v", key, removeErr)
		}
		return fmt.Errorf("校验删除 %s 失败: %v", key, verifyErr)
	}
	if exists {
		// 变量仍在：以 removeFn 的错误为准（更直接）
		if removeErr != nil {
			return fmt.Errorf("删除 %s 失败: %v", key, removeErr)
		}
		return fmt.Errorf("删除 %s 失败: 变量仍存在于 Windows 用户环境变量中", key)
	}
	// 变量已不存在：无论 removeFn 是否报错，目标达成
	return nil
}

func removeUserEnv(key string) error {
	// 优先用 PowerShell .NET API：变量不存在时也不报错，兼容所有 Windows 语言版本和编码
	// 先 LookPath 确认 powershell.exe 在 PATH 中，避免 Server Core 等精简环境下的无意义错误输出
	// PowerShell .NET API 会自动广播 WM_SETTINGCHANGE，无需手工通知
	if _, err := exec.LookPath("powershell"); err == nil {
		psKey := strings.ReplaceAll(key, "'", "''") // 转义单引号（防御性处理）
		script := fmt.Sprintf(`[Environment]::SetEnvironmentVariable('%s', $null, 'User')`, psKey)
		if err := runCommand("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script); err == nil {
			return nil
		}
	}
	// PowerShell 不可用时回退到 REG DELETE（不会自动广播，需要手工通知）
	cmd := exec.Command("REG", "DELETE", `HKCU\Environment`, "/V", key, "/F")
	output, err := cmd.CombinedOutput()
	if err == nil {
		broadcastEnvironmentChange()
		return nil
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed != "" {
		return fmt.Errorf("%v: %s", err, trimmed)
	}
	return err
}

// wslContentMatches 判断 /proc/version 文件内容是否表明运行在 WSL 环境。
// 独立为纯函数以便单元测试，I/O 部分由 isWSL() 负责。
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
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return err
	}
	return fmt.Errorf("%v: %s", err, trimmed)
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
	// 测试时去掉模型名末尾的 [1m] 后缀，用真实模型名发起请求（避免上游不识别带后缀的 id）
	model = stripModelSuffix(model)
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
	cfg := Config{
		BaseURL:     getEnvVar(envBaseURL),
		AuthToken:   getEnvVar(envAuthToken),
		Model:       getEnvVar(envModel),
		HaikuModel:  getEnvVar(envHaikuModel),
		SonnetModel: getEnvVar(envSonnetModel),
		OpusModel:   getEnvVar(envOpusModel),
	}

	fallback := loadConfigFromClaudeSettings()
	if cfg.BaseURL == "" {
		cfg.BaseURL = fallback.BaseURL
	}
	if cfg.AuthToken == "" {
		cfg.AuthToken = fallback.AuthToken
	}
	if cfg.Model == "" {
		cfg.Model = fallback.Model
	}
	if cfg.HaikuModel == "" {
		cfg.HaikuModel = fallback.HaikuModel
	}
	if cfg.SonnetModel == "" {
		cfg.SonnetModel = fallback.SonnetModel
	}
	if cfg.OpusModel == "" {
		cfg.OpusModel = fallback.OpusModel
	}
	return cfg
}

// getNewBaseURL 获取新的 Base URL。allowBack=true 时支持返回上一步（back=true）。
func getNewBaseURL(existing string, allowBack bool) (string, bool) {
	printSectionHeader("配置 API 服务器地址")
	fmt.Println("  示例: https://www.dmxapi.cn")

	if existing != "" {
		fmt.Printf("  当前值: %s\n", existing)
		ok, back := styledConfirm("是否修改 Base URL", allowBack)
		if allowBack && back {
			return existing, true
		}
		if !ok {
			return existing, false
		}
	}

	for {
		input, back := styledInputWithBack("Base URL")
		if allowBack && back {
			return existing, true
		}
		if input == "" && existing != "" {
			return existing, false
		}

		input = ensureScheme(input)
		if err := validateURL(input); err != nil {
			printError(err.Error())
			continue
		}

		return input, false
	}
}

// getNewAuthToken 获取新的 Auth Token。allowBack=true 时支持返回上一步（back=true）。
func getNewAuthToken(existing, hostname string, allowBack bool) (string, bool) {
	printSectionHeader("配置 API 认证令牌")

	if hostname != "" {
		fmt.Printf("  获取地址: https://%s/token\n", hostname)
	}

	if existing != "" {
		fmt.Printf("  当前已配置 Token: %s\n", maskToken(existing))
		ok, back := styledConfirm("是否更新 Token", allowBack)
		if allowBack && back {
			return existing, true
		}
		if !ok {
			return existing, false
		}
	}

	for {
		input, back := styledInputWithBack("Auth Token")
		if allowBack && back {
			return existing, true
		}
		if input == "" {
			if existing != "" {
				return existing, false
			}
			printError("Token 不能为空")
			continue
		}

		return strings.TrimSpace(input), false
	}
}

// topMenuKind 表示动态主菜单选中项的语义类别。
type topMenuKind int

const (
	topRecommended topMenuKind = iota // dmxapi 推荐配置
	topNamed                          // 某个已保存的命名配置
	topAdd                            // 新增配置
	topClear                          // 清除配置
)

// topMenuChoice 是动态主菜单的分发结果。
type topMenuChoice struct {
	kind  topMenuKind
	named NamedConfig // 仅当 kind == topNamed 时有效
}

// mapTopMenuIndex 将 1-based 菜单索引映射为语义类别（纯函数，便于测试）。
// 布局：1=推荐；2..1+n=命名配置；2+n=新增；3+n=清除。
func mapTopMenuIndex(idx, namedCount int) topMenuKind {
	switch {
	case idx == 1:
		return topRecommended
	case idx >= 2 && idx <= 1+namedCount:
		return topNamed
	case idx == 2+namedCount:
		return topAdd
	default:
		return topClear
	}
}

// selectTopModeDynamic 渲染动态主菜单（含已保存的命名配置），返回分发结果。
func selectTopModeDynamic() topMenuChoice {
	configs, _ := listNamedConfigs() // 出错按空处理，不阻断主流程
	n := len(configs)

	items := []MenuItem{{"1", "dmxapi 推荐配置", "Claude Opus 4.8 一键配置"}}
	for i, c := range configs {
		items = append(items, MenuItem{strconv.Itoa(i + 2), c.Name, namedConfigDesc(c)})
	}
	items = append(items,
		MenuItem{strconv.Itoa(n + 2), "新增配置", "手动配置 URL / Token / 模型等"},
		MenuItem{strconv.Itoa(n + 3), "清除配置", "清除全部或单个已保存配置"},
	)

	idx, _ := runItemMenu("请选择配置方式", items, false)
	kind := mapTopMenuIndex(idx, n)
	choice := topMenuChoice{kind: kind}
	if kind == topNamed {
		choice.named = configs[idx-2]
	}
	return choice
}

// namedConfigDesc 为命名配置生成菜单项副描述。
func namedConfigDesc(c NamedConfig) string {
	host := extractHost(c.BaseURL)
	if c.Model != "" && host != "" {
		return fmt.Sprintf("%s | %s", c.Model, host)
	}
	if c.Model != "" {
		return c.Model
	}
	return host
}

// selectFixOption 让用户选择要修改的内容。allowBack=true 时支持返回（back=true）。
func selectFixOption(allowBack bool) (int, bool) {
	return runItemMenu("选择要修改的内容", []MenuItem{
		{"1", "修改 URL", "Base URL 有问题"},
		{"2", "修改 Key", "API Key 有问题"},
		{"3", "都修改", "URL 和 Key 都有问题"},
		{"4", "修改模型名", "模型名称可能不正确"},
		{"5", "强制配置", "跳过验证，直接保存当前配置"},
	}, allowBack)
}

// inputNewBaseURL 输入新的 Base URL（无需确认是否修改）。
// allowBack=true 时输入 b/back 返回（back=true）。
func inputNewBaseURL(allowBack bool) (string, bool) {
	for {
		input, back := styledInputWithBack("新 Base URL")
		if allowBack && back {
			return "", true
		}
		if input == "" {
			printError("URL 不能为空")
			continue
		}
		input = ensureScheme(input)
		if err := validateURL(input); err != nil {
			printError(err.Error())
			continue
		}
		return input, false
	}
}

// inputNewAuthToken 输入新的 Auth Token（无需确认是否修改）。
// allowBack=true 时输入 b/back 返回（back=true）。
func inputNewAuthToken(hostname string, allowBack bool) (string, bool) {
	if hostname != "" {
		fmt.Printf("  获取地址: https://%s/token\n", hostname)
	}
	for {
		input, back := styledInputWithBack("新 Auth Token")
		if allowBack && back {
			return "", true
		}
		if input == "" {
			printError("Token 不能为空")
			continue
		}
		return strings.TrimSpace(input), false
	}
}

// renderItemMenu 渲染通用条目菜单，返回渲染行数（len(items)+6）
func renderItemMenu(title string, items []MenuItem, selectedIdx int, linesPrinted int, allowBack bool) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	// 行内真实固定开销（实际打印的非内容列）：
	//   非选中行 = 前导(1)+占位(2)+label后分隔(2)=5；
	//   选中行   = 前导(1)+提示符(iconW)+空格(1)+label后分隔(2)。iconPrompt 已为 ASCII，宽度恒 1。
	const rightMargin = 2 // 内容与右边框之间的固定空白列数（只用于撑大 inner，不进 pad 计算）
	iconW := visibleLength(iconPrompt)
	selOverhead := 2 + iconW + 2
	unselOverhead := 5
	overhead := unselOverhead // 取两者较大值作为 inner 的统一基准，保证选中切换时盒子宽度不变
	if selOverhead > overhead {
		overhead = selOverhead
	}
	// 盒子内宽（两条 │ 之间的列数）按最长行动态自适应，下限保持 boxWidth。
	// title 也可能含用户输入（如「管理配置「name」」），需一并纳入，否则长名称会撑破盒子。
	inner := boxWidth
	if w := visibleLength(title) + 2; w > inner { // 标题至少左右各留 1 列
		inner = w
	}
	for _, item := range items {
		// +rightMargin 使最长行右侧也留出固定空白，内容不紧贴右边框。
		if w := overhead + visibleLength(item.Label) + visibleLength(item.Desc) + rightMargin; w > inner {
			inner = w
		}
	}
	// 封顶到终端宽度（盒子总宽 = inner + 2 条边框），避免超出终端导致换行错位。
	if cols, _, err := term.GetSize(int(syscall.Stdin)); err == nil && cols-2 >= 40 && inner > cols-2 {
		inner = cols - 2
	}
	border := strings.Repeat(boxH, inner)
	fmt.Printf("%s%s%s\033[K\r\n", boxTL, border, boxTR)
	title = fitWidth(title, inner) // 极窄终端封顶后 title 仍可能超宽，截断兜底
	titleW := visibleLength(title)
	lPad := (inner - titleW) / 2
	rPad := inner - titleW - lPad
	fmt.Printf("%s%s%s%s%s%s%s\033[K\r\n",
		boxV, strings.Repeat(" ", lPad), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", rPad), boxV)
	fmt.Printf("%s%s%s\033[K\r\n", boxML, border, boxMR)
	for i, item := range items {
		selected := i == selectedIdx
		lineOverhead := unselOverhead
		if selected {
			lineOverhead = selOverhead
		}
		// 内容超出可用宽度时，优先截断自定义名称（label）保留描述；
		// 若描述本身已超宽（极窄终端），再兜底截断描述，确保右边框始终对齐。
		label := fitWidth(item.Label, inner-lineOverhead-visibleLength(item.Desc))
		desc := fitWidth(item.Desc, inner-lineOverhead-visibleLength(label))
		pad := inner - lineOverhead - visibleLength(label) - visibleLength(desc)
		if pad < 0 {
			pad = 0
		}
		if selected {
			fmt.Printf("%s %s%s %s%s  %s%s%s%s%s\033[K\r\n",
				boxV, colorBrightCyan+styleBold, iconPrompt,
				label, colorReset,
				colorBrightCyan, desc, colorReset,
				strings.Repeat(" ", pad), boxV)
		} else {
			fmt.Printf("%s %s  %s%s  %s%s%s%s%s\033[K\r\n",
				boxV, styleDim,
				label, colorReset,
				styleDim, desc, colorReset,
				strings.Repeat(" ", pad), boxV)
		}
	}
	fmt.Printf("%s%s%s\033[K\r\n", boxBL, border, boxBR)
	fmt.Printf("\033[K\r\n")
	if allowBack {
		fmt.Printf("  %s%s%s 导航%s  %sEnter 确认%s  %sEsc 返回%s\033[K\r\n",
			styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset, styleDim, colorReset)
	} else {
		fmt.Printf("  %s%s%s 导航%s  %sEnter 确认%s\033[K\r\n",
			styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset)
	}
	return len(items) + 6
}

// runItemMenu 运行通用条目菜单，返回1-based选中索引。
// allowBack=true 时支持 ESC / b 返回上一步（back=true，此时 idx 无意义返回 0）。
func runItemMenu(title string, items []MenuItem, allowBack bool) (int, bool) {
	n := len(items)
	restore, err := enterRawMode()
	if err != nil {
		// 降级：数字输入
		printMenu(title, items)
		fmt.Println()
		if allowBack {
			printInfo("输入对应数字选择，或输入 b 返回上一步")
		}
		validKeys := make([]string, n)
		for i := range items {
			validKeys[i] = items[i].Key
		}
		for {
			input := styledInput("选项")
			if allowBack && isBackToken(input) {
				return 0, true
			}
			for i, k := range validKeys {
				if input == k {
					return i + 1, false
				}
			}
			printError(fmt.Sprintf("无效选项，请输入 %s", strings.Join(validKeys, "、")))
		}
	}
	defer restore()

	selectedIdx := 0
	linesPrinted := 0
	for {
		linesPrinted = renderItemMenu(title, items, selectedIdx, linesPrinted, allowBack)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + n) % n
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % n
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			return selectedIdx + 1, false
		case KeyEsc:
			if allowBack {
				restore()
				clearMenuLines(linesPrinted)
				return 0, true
			}
			// allowBack=false：忽略 ESC（顶层主菜单必须做出选择）
		}
	}
}

// ==================== 交互式模型选择 ====================

// enterRawMode 进入终端原始模式，返回恢复函数。
// 在 legacy 控制台（如无 VT 支持的老版 Windows cmd）下主动返回错误，
// 让菜单走数字输入降级分支，避免使用光标控制序列（\033[...）带来乱码。
func enterRawMode() (restoreFn func(), err error) {
	if legacyConsoleMode {
		return nil, fmt.Errorf("legacy console: raw mode disabled")
	}
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
		restoreConsole()
		os.Exit(130)
	case 0x1B: // ESC 序列（Linux/macOS/Windows Terminal）
		if stdinBytesAvailable() == 0 {
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

// fitWidth 将字符串裁剪到不超过 max 显示宽度。
// max>=3 时超长部分以 "..." 结尾；max<3 时按字符宽度硬截断（不留省略号）；max<=0 返回空串。
func fitWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if visibleLength(s) <= max {
		return s
	}
	if max >= 3 {
		return truncateStr(s, max)
	}
	width := 0
	var result []rune
	for _, r := range s {
		rw := runeWidth(r)
		if width+rw > max {
			break
		}
		result = append(result, r)
		width += rw
	}
	return string(result)
}

// findPresetIndex 在 presetModels 中查找，找不到返回 -1
func findPresetIndex(value string) int {
	for i, m := range presetModels {
		if m.ID == value {
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
func renderConfirmMenuCore(question string, labels [2]string, descs [2]string, selectedIdx int, linesPrinted int, allowBack bool) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	const innerW = 44
	border := strings.Repeat(boxH, innerW)
	fmt.Printf("%s%s%s\033[K\r\n", boxTL, border, boxTR)

	questionW := visibleLength(question)
	lPad := (innerW - questionW) / 2
	if lPad < 0 {
		lPad = 0
	}
	rPad := innerW - questionW - lPad
	if rPad < 0 {
		rPad = 0
	}
	fmt.Printf("%s%s%s%s%s%s%s\033[K\r\n",
		boxV, strings.Repeat(" ", lPad), styleBold+colorBrightWhite, question, colorReset, strings.Repeat(" ", rPad), boxV)
	fmt.Printf("%s%s%s\033[K\r\n", boxML, border, boxMR)

	for i := 0; i < 2; i++ {
		label := labels[i]
		desc := descs[i]
		pad := innerW - 5 - visibleLength(label) - visibleLength(desc)
		if pad < 0 {
			pad = 0
		}
		if i == selectedIdx {
			fmt.Printf("%s %s%s %s%s  %s%s%s%s%s\033[K\r\n",
				boxV, colorBrightCyan+styleBold, iconPrompt,
				label, colorReset,
				colorBrightCyan, desc, colorReset,
				strings.Repeat(" ", pad), boxV)
		} else {
			fmt.Printf("%s %s  %s%s  %s%s%s%s%s\033[K\r\n",
				boxV, styleDim,
				label, colorReset,
				styleDim, desc, colorReset,
				strings.Repeat(" ", pad), boxV)
		}
	}

	fmt.Printf("%s%s%s\033[K\r\n", boxBL, border, boxBR)
	fmt.Printf("\033[K\r\n")
	if allowBack {
		fmt.Printf("  %s%s%s 导航%s  %sEnter 确认%s  %sEsc 返回%s\033[K\r\n",
			styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset, styleDim, colorReset)
	} else {
		fmt.Printf("  %s%s%s 导航%s  %sEnter 确认%s\033[K\r\n",
			styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset)
	}
	return 8
}

// renderConfirmMenu 渲染默认确认菜单（是/否），返回渲染行数（固定8行）
// selectedIdx: 0=是, 1=否
func renderConfirmMenu(question string, selectedIdx int, linesPrinted int, allowBack bool) int {
	return renderConfirmMenuCore(
		question,
		[2]string{"是", "否"},
		[2]string{"确认修改", "保持当前值不变"},
		selectedIdx,
		linesPrinted,
		allowBack,
	)
}

// runConfirmMenu 运行确认菜单，返回是否确认（true=是，false=否）
// 默认选中"否"（与原来 y/N 默认 No 行为一致）。
// allowBack=true 时 ESC / b 返回上一步（back=true）；
// allowBack=false 时 ESC 维持原语义（返回 false，等价于"否"）。
func runConfirmMenu(question string, allowBack bool) (confirmed bool, back bool) {
	restore, err := enterRawMode()
	if err != nil {
		// 降级：非终端时用数字选项
		printMenu(question, []MenuItem{
			{"1", "是", "确认修改"},
			{"2", "否", "保持当前值不变"},
		})
		fmt.Println()
		if allowBack {
			printInfo("输入 1 或 2，或输入 b 返回上一步")
		}
		for {
			input := styledInput("选项")
			if allowBack && isBackToken(input) {
				return false, true
			}
			switch input {
			case "1":
				return true, false
			case "2":
				return false, false
			default:
				printError("请输入 1 或 2")
			}
		}
	}
	defer restore()

	selectedIdx := 1 // 默认"否"
	linesPrinted := 0

	for {
		linesPrinted = renderConfirmMenu(question, selectedIdx, linesPrinted, allowBack)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + 2) % 2
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % 2
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			return selectedIdx == 0, false
		case KeyEsc:
			restore()
			clearMenuLines(linesPrinted)
			if allowBack {
				return false, true
			}
			return false, false
		}
	}
}

// runEnableDisableMenu 运行启用/禁用确认菜单，返回是否启用（true=启用，false=禁用）
// 默认选中"禁用"（索引1）。
// allowBack=true 时 ESC / b 返回上一步（back=true）；
// allowBack=false 时 ESC 维持原语义（返回 false，等价于"禁用"）。
func runEnableDisableMenu(question string, allowBack bool) (enabled bool, back bool) {
	restore, err := enterRawMode()
	if err != nil {
		// 降级：非终端时用数字选项
		printMenu(question, []MenuItem{
			{"1", "启用", "开启此功能"},
			{"2", "禁用", "关闭此功能"},
		})
		fmt.Println()
		if allowBack {
			printInfo("输入 1 或 2，或输入 b 返回上一步")
		}
		for {
			input := styledInput("选项")
			if allowBack && isBackToken(input) {
				return false, true
			}
			switch input {
			case "1":
				return true, false
			case "2":
				return false, false
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
			[2]string{"开启此功能", "关闭此功能"},
			selectedIdx,
			linesPrinted,
			allowBack,
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
			return selectedIdx == 0, false
		case KeyEsc:
			restore()
			clearMenuLines(linesPrinted)
			if allowBack {
				return false, true
			}
			return false, false
		}
	}
}

// renderL1Menu 渲染一级菜单（含模型条目 + 1 个「完成模型配置」出口项），返回渲染行数（len(entries)+7）
func renderL1Menu(entries []modelTypeEntry, selectedIdx int, linesPrinted int, allowBack bool) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	border := strings.Repeat(boxH, boxWidth)
	fmt.Printf("%s%s%s\033[K\r\n", boxTL, border, boxTR)
	title := "选择要配置的模型"
	titleW := visibleLength(title)
	lPad := (boxWidth - titleW) / 2
	rPad := boxWidth - titleW - lPad
	fmt.Printf("%s%s%s%s%s%s%s\033[K\r\n",
		boxV, strings.Repeat(" ", lPad), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", rPad), boxV)
	fmt.Printf("%s%s%s\033[K\r\n", boxML, border, boxMR)

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
			fmt.Printf("%s %s%s %s%s%s  %s%s%s%s%s\033[K\r\n",
				boxV, colorBrightCyan+styleBold, iconPrompt,
				label, labelFill, colorReset,
				colorBrightCyan, val, colorReset,
				strings.Repeat(" ", pad), boxV)
		} else {
			fmt.Printf("%s %s  %s%s%s  %s%s%s%s%s\033[K\r\n",
				boxV, styleDim,
				label, labelFill, colorReset,
				styleDim, val, colorReset,
				strings.Repeat(" ", pad), boxV)
		}
	}

	// 完成项（索引 len(entries)）：作为本页的「前进/完成」出口
	doneText := fmt.Sprintf("%s 完成模型配置", iconCheck)
	donePad := boxWidth - 3 - visibleLength(doneText)
	if donePad < 0 {
		donePad = 0
	}
	if selectedIdx == len(entries) {
		fmt.Printf("%s %s%s%s %s%s%s%s%s\033[K\r\n",
			boxV, colorBrightCyan+styleBold, iconPrompt, colorReset,
			colorBrightGreen, doneText, colorReset,
			strings.Repeat(" ", donePad), boxV)
	} else {
		fmt.Printf("%s   %s%s%s%s%s\033[K\r\n",
			boxV, styleDim, doneText, colorReset,
			strings.Repeat(" ", donePad), boxV)
	}

	fmt.Printf("%s%s%s\033[K\r\n", boxBL, border, boxBR)
	fmt.Printf("\033[K\r\n")
	if allowBack {
		fmt.Printf("  %s%s%s 导航%s  %sEnter 选择/完成%s  %sq/Esc 返回上一步%s\033[K\r\n",
			styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset, styleDim, colorReset)
	} else {
		fmt.Printf("  %s%s%s 导航%s  %sEnter 选择/完成%s  %sq/Esc 完成%s\033[K\r\n",
			styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset, styleDim, colorReset)
	}
	return len(entries) + 7
}

// renderL2Menu 渲染二级菜单，返回渲染行数（len(presetModels)+7）
func renderL2Menu(typeName string, currentValue string, selectedIdx int, linesPrinted int) int {
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	border := strings.Repeat(boxH, boxWidth)
	fmt.Printf("%s%s%s\033[K\r\n", boxTL, border, boxTR)
	title := fmt.Sprintf("选择 %s", typeName)
	titleW := visibleLength(title)
	lPad := (boxWidth - titleW) / 2
	rPad := boxWidth - titleW - lPad
	fmt.Printf("%s%s%s%s%s%s%s\033[K\r\n",
		boxV, strings.Repeat(" ", lPad), styleBold+colorBrightWhite, title, colorReset, strings.Repeat(" ", rPad), boxV)
	fmt.Printf("%s%s%s\033[K\r\n", boxML, border, boxMR)

	for i, m := range presetModels {
		isCurrent := (m.ID == currentValue)
		isSelected := (i == selectedIdx)
		display := m.ID
		if m.Hint != "" {
			display = fmt.Sprintf("%s （%s）", m.ID, m.Hint)
		}
		name := truncateStr(display, boxWidth-6)
		nameW := visibleLength(name)
		var check string
		var checkW int
		if isCurrent {
			check = fmt.Sprintf("%s%s%s", colorBrightGreen, iconCheck, colorReset)
			checkW = visibleLength(iconCheck)
		} else {
			check = strings.Repeat(" ", visibleLength(iconCheck))
			checkW = visibleLength(iconCheck)
		}
		pad := boxWidth - 3 - nameW - checkW - 1
		if pad < 0 {
			pad = 0
		}
		if isSelected {
			fmt.Printf("%s %s%s%s %s%s%s%s %s%s\033[K\r\n",
				boxV, colorBrightCyan+styleBold, iconPrompt, colorReset,
				colorBrightCyan, name, colorReset,
				strings.Repeat(" ", pad),
				check, boxV)
		} else {
			fmt.Printf("%s   %s%s%s%s %s%s\033[K\r\n",
				boxV, styleDim, name, colorReset,
				strings.Repeat(" ", pad),
				check, boxV)
		}
	}

	// 自定义选项（索引 len(presetModels)）
	customText := fmt.Sprintf("%s 自定义输入...", iconEdit)
	customPad := boxWidth - 3 - visibleLength(customText)
	if selectedIdx == len(presetModels) {
		fmt.Printf("%s %s%s%s %s%s%s%s%s\033[K\r\n",
			boxV, colorBrightCyan+styleBold, iconPrompt, colorReset,
			colorBrightYellow, customText, colorReset,
			strings.Repeat(" ", customPad), boxV)
	} else {
		fmt.Printf("%s   %s%s%s%s%s\033[K\r\n",
			boxV, styleDim, customText, colorReset,
			strings.Repeat(" ", customPad), boxV)
	}

	fmt.Printf("%s%s%s\033[K\r\n", boxBL, border, boxBR)
	fmt.Printf("\033[K\r\n")
	fmt.Printf("  %s%s%s 导航%s  %sEnter 确认%s  %sq/Esc 返回%s\033[K\r\n",
		styleDim, iconNavUp, iconNavDown, colorReset, styleDim, colorReset, styleDim, colorReset)
	return len(presetModels) + 7
}

// runL2Menu 运行二级菜单，返回选中的模型名。
// 预设列表上的 ESC 恒为"取消修改、停留上层"（返回 currentValue, back=false）。
// allowBack=true 时，进入文本输入（降级/自定义）后按 ESC 或输入 b/back 触发返回（back=true）。
func runL2Menu(typeName, currentValue string, allowBack bool) (string, bool) {
	restore, err := enterRawMode()
	if err != nil {
		// 降级：直接文本输入
		hint := "(输入模型名，留空不改)"
		val, back := styledInputWithBack(typeName + " " + hint)
		if allowBack && back {
			return currentValue, true
		}
		if val == "" {
			return currentValue, false
		}
		return val, false
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
			idx = (idx - 1 + len(presetModels) + 1) % (len(presetModels) + 1)
		case KeyDown:
			idx = (idx + 1) % (len(presetModels) + 1)
		case KeyEnter:
			restore()
			clearMenuLines(linesPrinted)
			if idx == len(presetModels) {
				// 自定义输入
				hint := "(自定义)"
				val, back := styledInputWithBack(typeName + " " + hint)
				if allowBack && back {
					return currentValue, true
				}
				if val == "" {
					return currentValue, false
				}
				return val, false
			}
			return presetModels[idx].ID, false
		case KeyEsc:
			restore()
			clearMenuLines(linesPrinted)
			return currentValue, false // 取消修改，停留上层（back 恒 false）
		}
	}
}

// runL1Menu 运行一级菜单。allowBack=true 时 q/Esc 返回上一步（back=true）；
// allowBack=false 时 q/Esc 表示"完成"（back=false）。
func runL1Menu(cfg *Config, allowBack bool) bool {
	entries := []modelTypeEntry{
		{"默认模型", &cfg.Model},
		{"Haiku 模型", &cfg.HaikuModel},
		{"Sonnet 模型", &cfg.SonnetModel},
		{"Opus 模型", &cfg.OpusModel},
	}

	restore, err := enterRawMode()
	if err != nil {
		return configureModelsFallback(cfg, allowBack)
	}
	defer restore()

	selectedIdx := 0
	linesPrinted := 0
	// 菜单项数 = 模型条目 + 1 个「完成模型配置」出口项
	itemCount := len(entries) + 1

	for {
		linesPrinted = renderL1Menu(entries, selectedIdx, linesPrinted, allowBack)
		key := readRawKey()
		switch key {
		case KeyUp:
			selectedIdx = (selectedIdx - 1 + itemCount) % itemCount
		case KeyDown:
			selectedIdx = (selectedIdx + 1) % itemCount
		case KeyEnter:
			if selectedIdx == len(entries) {
				// 焦点在「完成模型配置」项：前进到下一步（非返回）
				restore()
				clearMenuLines(linesPrinted)
				return false
			}
			restore()
			clearMenuLines(linesPrinted)
			// L2 为层内子菜单，不需 b/back 返回（其 ESC 已表示取消修改）
			newVal, _ := runL2Menu(entries[selectedIdx].Label, *entries[selectedIdx].ValuePtr, false)
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
			clearMenuLines(linesPrinted)
			return allowBack // allowBack=true → 返回上一步；否则完成
		}
	}
}

// configureModelsFallback 降级模型配置（文本输入模式）。
// allowBack=true 时仅"是否修改模型配置"确认支持返回；各模型名输入不支持回退。
func configureModelsFallback(cfg *Config, allowBack bool) bool {
	fmt.Println()
	fmt.Println("当前模型配置:")
	fmt.Printf("  %-35s = %s\n", envModel, cfg.Model)
	fmt.Printf("  %-35s = %s\n", envHaikuModel, cfg.HaikuModel)
	fmt.Printf("  %-35s = %s\n", envSonnetModel, cfg.SonnetModel)
	fmt.Printf("  %-35s = %s\n", envOpusModel, cfg.OpusModel)

	ok, back := styledConfirm("是否修改模型配置", allowBack)
	if allowBack && back {
		return true
	}
	if !ok {
		return false
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
	return false
}

// configureModels 配置模型。allowBack 透传给 L1 菜单（true 时 q/Esc 返回上一步，back=true）。
func configureModels(cfg *Config, allowBack bool) bool {
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

	if runL1Menu(cfg, allowBack) {
		return true // 返回上一步
	}

	fmt.Println()
	printSuccess("模型配置已完成")
	return false
}

// saveConfig 保存配置（同时写入系统环境变量与 Claude settings）。
func saveConfig(cfg Config) error {
	vars := buildManagedEnvMap(cfg, getManagedAgentTeamsValue())

	// 设置当前进程环境变量，保证当前会话立即可用
	for key, value := range vars {
		if value != "" {
			os.Setenv(key, value)
		}
	}

	var errs []string

	// 持久化到系统环境变量
	switch runtime.GOOS {
	case "windows":
		if err := setEnvVarsWindows(vars); err != nil {
			errs = append(errs, fmt.Sprintf("系统环境变量写入失败: %v", err))
		}
	default:
		if err := setEnvVarsUnix(vars); err != nil {
			errs = append(errs, fmt.Sprintf("系统环境变量写入失败: %v", err))
		}
	}

	// 同步写入 Claude settings.json
	if err := saveClaudeSettingsConfig(cfg); err != nil {
		errs = append(errs, fmt.Sprintf("Claude settings.json 写入失败: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "；"))
	}
	return nil
}

// runRecommendedConfig 推荐配置一键流程：只输入 key，其他参数使用 dmxapi 推荐默认值，
// 并自动写入 Claude settings、系统环境变量与 VSCode settings.json。
// 返回 back=true 表示用户在 Token 输入或验证失败菜单按 ESC 返回主菜单。
func runRecommendedConfig() (back bool) {
	printSectionHeader("dmxapi 推荐配置 (Claude Opus 4.8)")
	fmt.Println()
	printInfo(fmt.Sprintf("Base URL:         %s", recommendedBaseURL))
	printInfo(fmt.Sprintf("默认模型:         %s", recommendedModel))
	printInfo(fmt.Sprintf("Haiku 模型:       %s", recommendedHaikuModel))
	printInfo(fmt.Sprintf("Sonnet 模型:      %s", recommendedSonnetModel))
	printInfo(fmt.Sprintf("Opus 模型:        %s", recommendedOpusModel))
	printInfo(fmt.Sprintf("Effort Level:     %s (%s=%s)", defaultEffortLevel, envEffortLevel, defaultEffortLevel))
	printInfo("将自动禁用实验性请求头、设置 max 最高推理深度，并配置 VSCode 插件")
	fmt.Println()

	existing := loadExistingConfig()
	hostname := extractHost(recommendedBaseURL)

	authToken, b := getNewAuthToken(existing.AuthToken, hostname, true)
	if b {
		return true // Token 输入返回 → 回主菜单
	}

	cfg := Config{
		BaseURL:     recommendedBaseURL,
		AuthToken:   authToken,
		Model:       recommendedModel,
		HaikuModel:  recommendedHaikuModel,
		SonnetModel: recommendedSonnetModel,
		OpusModel:   recommendedOpusModel,
	}

	fmt.Println()
	for {
		if err := validateAPIConnection(cfg.BaseURL, cfg.AuthToken, cfg.Model); err != nil {
			printError(fmt.Sprintf("API 连接验证失败: %v", err))
			fmt.Println()
			printInfo("当前配置:")
			fmt.Printf("  Base URL: %s\n", cfg.BaseURL)
			fmt.Printf("  API Key:  %s\n", cfg.AuthToken)
			fmt.Println()

			choice, cb := runItemMenu("API 验证失败，如何处理", []MenuItem{
				{"1", "修改 Key", "重新输入 API Key"},
				{"2", "强制保存", "跳过验证直接保存当前配置"},
			}, true)
			if cb {
				return true // 验证失败菜单返回 → 放弃推荐配置，回主菜单
			}
			if choice == 1 {
				tok, tb := inputNewAuthToken(hostname, true)
				if tb {
					fmt.Println()
					continue // 放弃改 Key → 重新验证
				}
				cfg.AuthToken = tok
				fmt.Println()
				continue
			}
			printWarning("已跳过 API 验证，将直接保存当前配置")
			break
		}
		printSuccess("API 连接验证成功!")
		break
	}

	// 提前注入 effort level，saveConfig 内的 buildManagedEnvMap 会读取并写入所有目标位置
	os.Setenv(envEffortLevel, defaultEffortLevel)

	fmt.Println()
	err := runWithSpinner("正在保存配置...", func() error {
		return saveConfig(cfg)
	})
	if err != nil {
		printError(fmt.Sprintf("保存配置失败: %v", err))
		os.Exit(1)
	}
	printSuccess("保存成功!")

	// 写入 dmxapi 默认 git 署名（顶层 attribution）。此前已注入 effort env，
	// 本调用内部 buildManagedEnvMap 会幂等重写 env，仅额外叠加 attribution。
	if err := saveClaudeSettingsConfigWithAttribution(cfg, getManagedAgentTeamsValue(), dmxapiDefaultAttribution()); err != nil {
		printWarning(fmt.Sprintf("Git 署名写入失败: %v", err))
	}

	fmt.Println()
	err = runWithSpinner("正在配置 VSCode 插件...", func() error {
		return saveVSCodeConfig(cfg)
	})
	if err != nil {
		printWarning(fmt.Sprintf("VSCode 配置写入失败: %v", err))
	} else {
		printSuccess("VSCode 插件配置成功!")
	}

	printSummary(cfg)
	return false
}

// configureAgentTeams 配置实验性 Agent Teams 功能环境变量。
// exitOnDone=true 时末尾显示"按回车键退出"（独立运行模式4时使用）；
// 嵌入模式1后置步骤时传 false，由 main 统一处理退出。
// allowBack=true 时，启用/禁用选择按 ESC 返回上一步（back=true，在任何写入副作用之前）。
func configureAgentTeams(exitOnDone, allowBack bool) (back bool) {
	printSectionHeader("配置实验性 Agent Teams 功能")
	fmt.Println()

	currentVal := getManagedAgentTeamsValue()
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

	enable, b := runEnableDisableMenu("是否启用 Agent Teams 功能", allowBack)
	if allowBack && b {
		return true // 返回上一步（尚未产生任何写入副作用）
	}

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
			if err := saveClaudeSettingsConfigWithAgentTeams(loadExistingConfig(), "1"); err != nil {
				printError(fmt.Sprintf("Claude settings 同步失败: %v", err))
				os.Unsetenv(envAgentTeams)
			} else {
				printSuccess(fmt.Sprintf("已启用 %s=1", envAgentTeams))
			}
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
				if err := saveClaudeSettingsConfigWithAgentTeams(loadExistingConfig(), ""); err != nil {
					printError(fmt.Sprintf("Claude settings 同步失败: %v", err))
					os.Setenv(envAgentTeams, currentVal)
				} else {
					printSuccess(fmt.Sprintf("已禁用并删除 %s", envAgentTeams))
				}
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
	if exitOnDone {
		fmt.Println()
		styledInput("按回车键退出")
	}
	return false
}

// configureEffortLevel 配置 CLAUDE_CODE_EFFORT_LEVEL 环境变量。
// exitOnDone=true 时末尾显示"按回车键退出"（独立运行模式时使用）；
// 嵌入后置步骤时传 false，由 main 统一处理退出。
// allowBack=true 时，启用/禁用选择按 ESC 返回上一步（back=true，在任何写入副作用之前）。
func configureEffortLevel(exitOnDone, allowBack bool) (back bool) {
	printSectionHeader("配置 Effort Level（max 最高推理深度）")
	fmt.Println()

	currentVal := getManagedEffortLevelValue()
	if currentVal != "" {
		printInfo(fmt.Sprintf("当前状态: %s已设置 (%s)%s", colorBrightGreen, currentVal, colorReset))
	} else {
		printInfo(fmt.Sprintf("当前状态: %s未设置%s", colorRed, colorReset))
	}
	fmt.Println()
	fmt.Printf("  CLAUDE_CODE_EFFORT_LEVEL=max 可让 Claude Code 使用最高推理深度，\n")
	fmt.Printf("  获得更深入的分析和更高质量的代码输出。\n")
	fmt.Println()
	fmt.Printf("  关闭后将移除 CLAUDE_CODE_EFFORT_LEVEL 环境变量。\n")
	fmt.Println()

	enable, b := runEnableDisableMenu("是否启用 Effort Level=max", allowBack)
	if allowBack && b {
		return true // 返回上一步（尚未产生任何写入副作用）
	}

	fmt.Println()
	var err error
	if enable {
		vars := map[string]string{envEffortLevel: defaultEffortLevel}
		switch runtime.GOOS {
		case "windows":
			err = setEnvVarsWindows(vars)
		default:
			err = setEnvVarsUnix(vars)
		}
		if err != nil {
			printError(fmt.Sprintf("设置失败: %v", err))
		} else {
			os.Setenv(envEffortLevel, defaultEffortLevel)
			if err := saveClaudeSettingsConfigWithAgentTeams(loadExistingConfig(), getManagedAgentTeamsValue()); err != nil {
				printError(fmt.Sprintf("Claude settings 同步失败: %v", err))
				os.Unsetenv(envEffortLevel)
			} else {
				printSuccess(fmt.Sprintf("已启用 %s=%s", envEffortLevel, defaultEffortLevel))
			}
		}
	} else {
		if currentVal == "" {
			printInfo("当前未设置该变量，无需操作")
		} else {
			switch runtime.GOOS {
			case "windows":
				err = removeEnvVarWindows(envEffortLevel)
			default:
				err = removeEnvVarUnix(envEffortLevel)
			}
			if err != nil {
				printError(fmt.Sprintf("删除失败: %v", err))
			} else {
				os.Unsetenv(envEffortLevel)
				if err := clearEffortFromClaudeSettings(); err != nil {
					printError(fmt.Sprintf("Claude settings 同步失败: %v", err))
					os.Setenv(envEffortLevel, currentVal)
				} else {
					printSuccess(fmt.Sprintf("已禁用并删除 %s", envEffortLevel))
				}
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
	if exitOnDone {
		fmt.Println()
		styledInput("按回车键退出")
	}
	return false
}

// attributionStateDesc 返回某个署名字段的当前态可读描述（仅 ASCII/中文，避免 ambiguous 宽度字符）。
func attributionStateDesc(p *string) string {
	if p == nil {
		return "Claude 默认署名"
	}
	if *p == "" {
		return "已关闭（不署名）"
	}
	return fmt.Sprintf("自定义: %s", *p)
}

// attributionSummaryDesc 返回 git 署名在配置摘要中的简短状态描述与颜色。
// 合并 commit / pr 两态：一致时给出统一描述，不一致时分别标注（避免自定义长文本撑破盒子）。
func attributionSummaryDesc(attr Attribution) (string, string) {
	// 0=Claude 默认；1=已关闭；2=自定义
	state := func(p *string) int {
		if p == nil {
			return 0
		}
		if *p == "" {
			return 1
		}
		return 2
	}
	label := func(s int) string {
		switch s {
		case 1:
			return "关闭"
		case 2:
			return "自定义"
		default:
			return "默认"
		}
	}
	c, pr := state(attr.Commit), state(attr.PR)
	if c == pr {
		switch c {
		case 1:
			return "已关闭", colorBrightYellow
		case 2:
			return "自定义", colorBrightGreen
		default:
			return "Claude 默认", colorWhite
		}
	}
	return fmt.Sprintf("commit:%s pr:%s", label(c), label(pr)), colorCyan
}

// configureAttributionField 对单个署名字段（commit 或 pr）做三态选择，直接修改 *target 指针。
// 返回 back=true 表示用户在本子菜单按 ESC 返回上一层（不改动 target）。
func configureAttributionField(fieldLabel string, target **string, allowBack bool) (back bool) {
	choice, b := runItemMenu(fmt.Sprintf("配置 %s 署名", fieldLabel), []MenuItem{
		{"1", "自定义署名文本", "输入要写入的署名内容"},
		{"2", "关闭署名", "写入空串，Claude Code 不再添加署名"},
		{"3", "恢复 Claude 默认", "移除该项，使用 Claude Code 默认署名"},
	}, allowBack)
	if allowBack && b {
		return true
	}
	switch choice {
	case 1:
		fmt.Println()
		printInfo(fmt.Sprintf("请输入 %s 署名文本（支持多行请用 \\n 表示换行）", fieldLabel))
		val, back := styledInputWithBack(fmt.Sprintf("%s 署名", fieldLabel))
		if back {
			return false // 输入步骤返回：本字段保持原值，回到署名主菜单
		}
		v := val
		*target = &v
	case 2:
		empty := ""
		*target = &empty
	case 3:
		*target = nil
	}
	return false
}

// configureAttribution 配置 Claude Code git 署名（顶层 attribution.commit / attribution.pr）。
// 直接操作传入的内存 *Attribution 指针，不立即写文件，由调用方统一持久化。
// 主菜单按 ESC（或降级模式输入 b）表示“配置完毕返回”，始终可退出本循环。
func configureAttribution(attr *Attribution) {
	for {
		printSectionHeader("配置 Git 署名 (attribution)")
		fmt.Println()
		printInfo(fmt.Sprintf("当前 Commit 署名: %s", attributionStateDesc(attr.Commit)))
		printInfo(fmt.Sprintf("当前 PR 署名:     %s", attributionStateDesc(attr.PR)))
		fmt.Println()
		printInfo("控制 git commit 与 Pull Request 中由 Claude Code 添加的署名文本。")
		printInfo("改完后选择「完成配置」或按 ESC 返回，修改将自动保存。")
		fmt.Println()

		choice, b := runItemMenu("选择要配置的项", []MenuItem{
			{"1", "配置 Commit 署名", attributionStateDesc(attr.Commit)},
			{"2", "配置 PR 署名", attributionStateDesc(attr.PR)},
			{"3", "完成配置", "保存当前署名设置并返回"},
		}, true)
		if b {
			return // ESC：等同完成，配置态已在内存，由调用方统一持久化
		}
		fmt.Println()
		switch choice {
		case 1:
			configureAttributionField("Commit", &attr.Commit, true)
		case 2:
			configureAttributionField("PR", &attr.PR, true)
		case 3:
			return // 完成配置：带着已修改的 attr 返回，由调用方统一持久化
		}
		fmt.Println()
	}
}

// normalizeAttribution 把 *Attribution 拷贝为可安全编辑的值；nil 入参返回零值 Attribution{}。
func normalizeAttribution(p *Attribution) Attribution {
	if p == nil {
		return Attribution{}
	}
	return *p
}

// attributionForSnapshot 将编辑态 Attribution 规范化为快照存储用的 *Attribution：
// 两子键都未管理（均 nil）时返回 nil，避免持久化出空对象 "attribution":{}。
func attributionForSnapshot(attr Attribution) *Attribution {
	if attr.Commit == nil && attr.PR == nil {
		return nil
	}
	return &attr
}

// dmxapiDefaultAttribution 返回新手流程默认 git 署名：commit 与 pr 同为 recommendedAttributionText。
// 两字段使用各自独立的指针，避免后续修改其一影响另一。
func dmxapiDefaultAttribution() Attribution {
	c := recommendedAttributionText
	p := recommendedAttributionText
	return Attribution{Commit: &c, PR: &p}
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
		return fmt.Sprintf("%s%s%s%s%s %s%s%s",
			styleBold+colorBrightWhite, label, colorReset,
			strings.Repeat(" ", pad), boxV,
			valueColor, value, colorReset)
	}

	// Agent Teams：优先读取系统环境变量，缺失时回退 Claude settings
	agentTeamsDisplay, agentTeamsColor := "未启用", colorWhite
	if getManagedAgentTeamsValue() == "1" {
		agentTeamsDisplay, agentTeamsColor = "已启用", colorBrightGreen
	}

	// Effort Level：优先读取系统环境变量，缺失时回退 Claude settings
	effortLevelDisplay, effortLevelColor := "未设置", colorWhite
	if val := getManagedEffortLevelValue(); val != "" {
		effortLevelDisplay, effortLevelColor = val, colorBrightGreen
	}

	// Claude Settings：解析 ~/.claude/settings.json，检测受管 env 是否存在
	claudeSettingsDisplay, claudeSettingsColor := "未配置", colorWhite
	if path, err := getClaudeSettingsPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if isClaudeSettingsConfigured(data) {
				claudeSettingsDisplay, claudeSettingsColor = "已配置", colorBrightGreen
			}
		}
	}

	// VSCode Plugin：解析 settings.json，检测目标键是否存在
	vscodeDisplay, vscodeColor := "未配置", colorWhite
	if path, err := getVSCodeSettingsPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if isVSCodeConfigured(data) {
				vscodeDisplay, vscodeColor = "已配置", colorBrightGreen
			}
		}
	}

	// Git 署名：读取 settings.json 顶层 attribution，合并 commit/pr 两态展示
	gitSignDisplay, gitSignColor := attributionSummaryDesc(getManagedAttribution())

	lines := []string{
		makeRow("Base URL", cfg.BaseURL, colorBrightGreen),
		makeRow("Auth Token", maskToken(cfg.AuthToken), colorBrightYellow),
		makeRow("Model", cfg.Model, colorCyan),
		makeRow("Haiku Model", cfg.HaikuModel, colorCyan),
		makeRow("Sonnet Model", cfg.SonnetModel, colorCyan),
		makeRow("Opus Model", cfg.OpusModel, colorCyan),
		makeRow("Disable Betas", fixedDisableExperimentalBetas, colorMagenta),
		makeRow("Agent Teams", agentTeamsDisplay, agentTeamsColor),
		makeRow("Effort Level", effortLevelDisplay, effortLevelColor),
		makeRow("Git 署名", gitSignDisplay, gitSignColor),
		makeRow("settings.json", claudeSettingsDisplay, claudeSettingsColor),
		makeRow("VSCode Plugin", vscodeDisplay, vscodeColor),
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

// isVSCodeConfigured 检测 JSON 内容是否含 claudeCode.environmentVariables 键（新键）
// 或旧版工具写入的 claude-code.environmentVariables 键（向后兼容）。
// 用于判断 VSCode settings.json 是否已由本工具写入配置。
func isVSCodeConfigured(data []byte) bool {
	cleaned := stripJSONC(data)
	var settings map[string]interface{}
	if err := json.Unmarshal(cleaned, &settings); err != nil {
		return false
	}
	_, hasNew := settings[vscodeEnvKey]
	_, hasOld := settings[vscodeEnvKeyOld]
	return hasNew || hasOld
}

// maskToken 遮盖 Token
func maskToken(token string) string {
	runes := []rune(token)
	if len(runes) <= 8 {
		return "********"
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
}

// checkClaudeCodeInstalled 检测 claude 命令是否已安装。
// Windows 下依次尝试 claude / claude.cmd / claude.exe，全部失败后
// 再用子进程运行 claude --version 作为最后兜底（兼容未注册 PATH 的安装）。
func checkClaudeCodeInstalled() bool {
	if _, err := exec.LookPath("claude"); err == nil {
		return true
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("claude.cmd"); err == nil {
			return true
		}
		if _, err := exec.LookPath("claude.exe"); err == nil {
			return true
		}
	}
	// 兜底：直接运行 claude --version，命令存在但未在 PATH 中时仍可检测到
	cmd := exec.Command("claude", "--version")
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
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
	// 读取前 256KB，tagRef 数据位于约 142KB 处，需留足余量
	lr := io.LimitReader(resp.Body, 262144)
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
	wantDownload, _ := runConfirmMenu(fmt.Sprintf("发现新版本 v%s，是否立即前往下载页？", latest), false)
	if wantDownload {
		openBrowser("https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases")
		os.Exit(0)
	}
}

// ==================== 主程序 ====================

func main() {
	// Windows 下保存旧代码页并启用 UTF-8 / VT；其他平台为 no-op
	restore := initWindowsConsole()
	defer restore()

	// 检测 CJK locale，用于 ambiguous width 渲染
	cjkAmbiguous = detectCJKLocale()

	// Ctrl+C / SIGTERM：清理 raw mode + 恢复代码页后退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		if rawModeState != nil {
			term.Restore(int(syscall.Stdin), rawModeState)
			fmt.Println()
		}
		restore()
		os.Exit(130)
	}()

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
		choice, _ := runItemMenu("检测未通过，如何处理", []MenuItem{
			{"1", "我已安装，跳过检测", "忽略检测结果，继续使用配置工具"},
			{"2", "确认退出", "退出程序"},
		}, false)
		if choice == 2 {
			os.Exit(1)
		}
		fmt.Println()
	}

	// 检查版本更新（失败则静默跳过）
	checkForUpdates()

	// 顶层配置方式选择（动态：含已保存的命名配置）。
	// 循环展示主菜单：新增流程在第一步（URL）回退时会回到此处重新选择。
	for {
		choice := selectTopModeDynamic()
		switch choice.kind {
		case topRecommended:
			if back := runRecommendedConfig(); back {
				fmt.Println()
				continue
			}
			fmt.Println()
			styledInput("按回车键退出")
			return
		case topNamed:
			if back := manageNamedConfig(choice.named); back {
				fmt.Println()
				continue
			}
			fmt.Println()
			styledInput("按回车键退出")
			return
		case topClear:
			if back := runClearConfigMenu(); back {
				fmt.Println()
				continue
			}
			fmt.Println()
			styledInput("按回车键退出")
			return
		case topAdd:
			// 新增配置流程；URL 第一步回退则回到主菜单重新选择
			if back := runAddConfigFlow(); back {
				fmt.Println()
				continue
			}
			// 等待用户退出
			fmt.Println()
			styledInput("按回车键退出")
			return
		}
	}
}

// runAddConfigFlow 新增配置流程：先填配置名称，再直接走"从头配置"。
// 返回 back=true 表示用户在 URL 第一步回退，应回到上层主菜单。
func runAddConfigFlow() (back bool) {
	fmt.Println()
	name, b := promptConfigName()
	if b {
		return true // 名称步骤 ESC 返回 → 上抛给 main 回主菜单
	}
	return runFromScratchConfig(name)
}

// configureURLTokenWithValidation 配置 Base URL / Token / 模型并循环验证 API 连接，
// 直到验证通过或用户选择"强制配置"跳过验证。就地修改 *cfg。
// 以状态机实现逐步回退：每步可退回上一步并保留已填值；第一步（URL）回退则
// 返回 back=true 上抛给调用方。forced 表示是否走了"强制跳过验证"分支（可忽略）。
func configureURLTokenWithValidation(cfg *Config) (forced bool, back bool) {
	const (
		sURL = iota
		sToken
		sModel
		sValidate
	)
	step := sURL
	for {
		switch step {
		case sURL:
			url, b := getNewBaseURL(cfg.BaseURL, true)
			if b {
				return false, true // 第一步回退 → 上抛
			}
			cfg.BaseURL = url
			step = sToken
		case sToken:
			hostname := extractHost(cfg.BaseURL)
			tok, b := getNewAuthToken(cfg.AuthToken, hostname, true)
			if b {
				step = sURL // 回 URL（cfg.BaseURL 已保留）
				continue
			}
			cfg.AuthToken = tok
			step = sModel
		case sModel:
			// 配置模型（在 API 验证前，使验证所用模型与用户选择一致）
			fmt.Println()
			if configureModels(cfg, true) {
				step = sToken // L1 的 ESC=返回 → 回 Token
				continue
			}
			step = sValidate
		case sValidate:
			// 验证 API 连接
			fmt.Println()
			if err := validateAPIConnection(cfg.BaseURL, cfg.AuthToken, cfg.Model); err != nil {
				printError(fmt.Sprintf("API 连接验证失败: %v", err))

				// 显示当前的URL和Key
				fmt.Println()
				printInfo("当前配置:")
				fmt.Printf("  Base URL: %s\n", cfg.BaseURL)
				fmt.Printf("  API Key:  %s\n", cfg.AuthToken)
				fmt.Println()

				// 让用户选择要修改什么
				choice, b := selectFixOption(true)
				if b {
					step = sModel // 修复菜单返回 → 回模型步重看并重验
					continue
				}

				switch choice {
				case 1: // 修改URL
					url, bb := inputNewBaseURL(true)
					if bb {
						fmt.Println()
						continue // 取消修改 → 重新验证
					}
					cfg.BaseURL = url
				case 2: // 修改Key
					tok, bb := inputNewAuthToken(extractHost(cfg.BaseURL), true)
					if bb {
						fmt.Println()
						continue
					}
					cfg.AuthToken = tok
				case 3: // 都修改
					url, bb := inputNewBaseURL(true)
					if bb {
						fmt.Println()
						continue
					}
					cfg.BaseURL = url
					tok, bb2 := inputNewAuthToken(extractHost(cfg.BaseURL), true)
					if bb2 {
						fmt.Println()
						continue
					}
					cfg.AuthToken = tok
				case 4: // 修改模型名（独立入口，b 返回表示放弃改模型）
					m, bb := runL2Menu("默认模型", cfg.Model, true)
					if !bb {
						cfg.Model = m
					}
				case 5: // 强制配置，跳过验证
					printWarning("已跳过 API 验证，将直接保存当前配置")
					fmt.Println()
					return true, false
				}
				fmt.Println()
				continue
			}
			printSuccess("API 连接验证成功!")
			return false, false
		}
	}
}

// runFromScratchConfig 从头配置流程：URL/Token/模型/验证 → 询问 Teams/VSCode →
// 保存生效 → 以 name 持久化命名配置 → 打印摘要。
// 以状态机实现逐步回退；返回 back=true 表示在第一步（URL）回退，需上抛给调用方。
func runFromScratchConfig(name string) (back bool) {
	// 加载现有配置作为默认值（循环外声明，回退时保留已填值）
	cfg := loadExistingConfig()

	const (
		sURLTok = iota
		sTeams
		sVSCode
		sAttribution
		sSave
	)
	step := sURLTok
	wantTeams, wantVSCode := false, false
	attr := dmxapiDefaultAttribution() // 默认 dmxapi 署名，sAttribution 步骤可覆盖

	for {
		switch step {
		case sURLTok:
			// 配置 URL / Token / 模型并验证
			if _, b := configureURLTokenWithValidation(&cfg); b {
				return true // URL 第一步回退 → 上抛
			}
			step = sTeams
		case sTeams:
			// 询问附加配置意向
			fmt.Println()
			v, b := styledConfirm("是否同时配置 Agent Teams 功能", true)
			if b {
				step = sURLTok
				continue
			}
			wantTeams = v
			step = sVSCode
		case sVSCode:
			fmt.Println()
			v, b := styledConfirm("是否同时配置 VSCode 插件", true)
			if b {
				step = sTeams
				continue
			}
			wantVSCode = v
			step = sAttribution
		case sAttribution:
			fmt.Println()
			printInfo(fmt.Sprintf("Git 署名默认: %s（直接回车采用默认，或输入自定义文本）", recommendedAttributionText))
			val, b := styledInputWithBack("Git 署名")
			if b {
				step = sVSCode
				continue
			}
			text := recommendedAttributionText
			if strings.TrimSpace(val) != "" {
				text = val
			}
			c, p := text, text
			attr = Attribution{Commit: &c, PR: &p}
			step = sSave
		case sSave:
			// 自动为新增配置启用思考等级（max），与推荐配置流程保持一致。
			// 提前注入进程环境变量，saveConfig 内 buildManagedEnvMap 会读取并写入所有目标位置，
			// 后续命名配置快照 EffortLevel 也会读到该值。
			os.Setenv(envEffortLevel, defaultEffortLevel)

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

			// 写入 git 署名（顶层 attribution）。saveConfig 已落定 env，此处叠加 attribution。
			if err := saveClaudeSettingsConfigWithAttribution(cfg, getManagedAgentTeamsValue(), attr); err != nil {
				printWarning(fmt.Sprintf("Git 署名写入失败: %v", err))
			}

			// 执行附加配置
			if wantTeams {
				fmt.Println()
				configureAgentTeams(false, false)
			}
			if wantVSCode {
				fmt.Println()
				configureVSCode(cfg, false, false)
			}

			// 持久化命名配置快照（生效已由 saveConfig 完成；此处失败仅告警，不影响生效）
			nc := NamedConfig{
				Config:      cfg,
				Name:        name,
				AgentTeams:  getManagedAgentTeamsValue(),
				EffortLevel: getManagedEffortLevelValue(),
				Attribution: attributionForSnapshot(attr),
				SavedAt:     time.Now().Format(time.RFC3339),
				AppVersion:  appVersion,
			}
			if path, err := saveNamedConfig(nc); err != nil {
				printWarning(fmt.Sprintf("命名配置持久化失败: %v", err))
			} else {
				printSuccess(fmt.Sprintf("已保存命名配置: %s", path))
			}

			// 打印摘要
			printSummary(cfg)
			return false
		}
	}
}

// promptConfigName 循环读取配置名称，已存在同名文件时确认是否覆盖。
// 按 ESC 返回上一步（back=true，此时 name 无意义返回空串）。
func promptConfigName() (name string, back bool) {
	for {
		input, esc := styledInputWithEsc("配置名称")
		if esc {
			return "", true
		}
		if input == "" {
			printError("名称不能为空")
			continue
		}
		dir, err := dmxapiConfigDir()
		if err == nil {
			path := filepath.Join(dir, namedConfigFileName(input))
			if _, statErr := os.Stat(path); statErr == nil {
				if ok, _ := styledConfirm(fmt.Sprintf("已存在同名配置「%s」，是否覆盖", input), false); !ok {
					fmt.Println()
					continue
				}
			}
		}
		return input, false
	}
}

// manageNamedConfig 命名配置管理界面：应用、编辑或删除。
// 返回 back=true 表示应回到主菜单：用户按 ESC 返回，或删除分支处理完毕
// （删除后主菜单会重新 listNamedConfigs 刷新列表）。
func manageNamedConfig(nc NamedConfig) (back bool) {
	printSectionHeader(fmt.Sprintf("配置: %s", nc.Name))
	fmt.Println()
	printInfo(fmt.Sprintf("Base URL:     %s", nc.BaseURL))
	printInfo(fmt.Sprintf("Token:        %s", maskToken(nc.AuthToken)))
	printInfo(fmt.Sprintf("默认模型:     %s", nc.Model))
	if nc.EffortLevel != "" {
		printInfo(fmt.Sprintf("Effort Level: %s", nc.EffortLevel))
	}
	if nc.AgentTeams != "" {
		printInfo("Agent Teams:  已启用")
	}
	if nc.Attribution != nil {
		if nc.Attribution.Commit != nil {
			printInfo(fmt.Sprintf("Commit 署名:   %s", attributionStateDesc(nc.Attribution.Commit)))
		}
		if nc.Attribution.PR != nil {
			printInfo(fmt.Sprintf("PR 署名:       %s", attributionStateDesc(nc.Attribution.PR)))
		}
	}
	fmt.Println()

	choice, back := runItemMenu(fmt.Sprintf("管理配置「%s」", nc.Name), []MenuItem{
		{"1", "应用此配置", "写入 settings.json 与系统环境变量"},
		{"2", "编辑此配置", "修改 URL/Token/模型/开关等"},
		{"3", "删除此配置", "从 ~/.DMXAPI/claude_code 移除"},
	}, true)
	if back {
		return true // 返回主菜单
	}
	switch choice {
	case 1:
		fmt.Println()
		if err := runWithSpinner("正在应用配置...", func() error {
			return applyNamedConfig(nc)
		}); err != nil {
			printError(fmt.Sprintf("应用配置失败: %v", err))
			return false
		}
		printSuccess("应用成功!")
		printSummary(nc.Config)
	case 2:
		fmt.Println()
		editNamedConfig(nc)
	case 3:
		fmt.Println()
		if ok, _ := styledConfirm(fmt.Sprintf("确定删除配置「%s」", nc.Name), false); !ok {
			printInfo("已取消，未做任何更改")
			return true // 取消删除：回主菜单（配置仍在列表）
		}
		if err := deleteNamedConfig(nc.FilePath); err != nil {
			printError(fmt.Sprintf("删除失败: %v", err))
			return true // 删除失败：回主菜单（配置仍在列表）
		}
		printSuccess(fmt.Sprintf("已删除配置「%s」", nc.Name))
		return true // 删除成功：回主菜单（列表已刷新，被删项随之消失）
	}
	return false
}

// editNamedConfig 编辑已保存的命名配置：选择一项细分配置修改后，
// 沿用原配置名称 nc.Name 覆盖保存并重新应用使其生效。
func editNamedConfig(nc NamedConfig) {
	// 先以快照播种 live env，使 getManaged*/各 configure* 子工具把 nc 的值视为"当前态"
	if nc.EffortLevel != "" {
		os.Setenv(envEffortLevel, normalizeEffortLevel(nc.EffortLevel))
	} else {
		os.Unsetenv(envEffortLevel)
	}
	if nc.AgentTeams != "" {
		os.Setenv(envAgentTeams, nc.AgentTeams)
	} else {
		os.Unsetenv(envAgentTeams)
	}

	// attribution 不进环境变量，以编辑态局部值贯穿本次编辑（跨 case 累积修改）
	attr := normalizeAttribution(nc.Attribution)

	// 循环展示编辑菜单：case 1 在 URL 第一步回退时不保存，回到此菜单重新选择。
	for {
		// 每轮从快照重新值拷贝，避免上一轮中途修改残留
		cfg := nc.Config

		choice, back := runItemMenu(fmt.Sprintf("编辑配置「%s」", nc.Name), []MenuItem{
			{"1", "修改 URL/Token", "重新配置 URL、Token 和模型"},
			{"2", "仅配置模型", "只修改默认/各档位模型"},
			{"3", "解决 400 报错", "禁用实验性请求头"},
			{"4", "配置 Effort Level", "设置 max 最高思考等级"},
			{"5", "配置实验性功能", "启用/禁用 Agent Teams"},
			{"6", "配置 VSCode 插件", "写入 VSCode settings.json"},
			{"7", "配置 Git 署名", "自定义/关闭 commit 与 PR 署名"},
		}, true)
		if back {
			return // 编辑菜单 ESC → 不保存，返回上一界面
		}
		fmt.Println()
		switch choice {
		case 1:
			// forced 可忽略：验证成功与强制跳过均应落到下方保存；
			// 仅 URL 第一步 ESC 返回（back）才放弃本次编辑，回到编辑菜单。
			if _, b := configureURLTokenWithValidation(&cfg); b {
				fmt.Println()
				continue
			}
		case 2:
			if configureModels(&cfg, true) {
				fmt.Println()
				continue
			}
		case 3:
			printSectionHeader("修复 Claude Code 400 请求头错误")
			printInfo("禁用实验性请求头，解决 Claude Code 400 传入请求头错误问题")
			fmt.Println()
		case 4:
			if configureEffortLevel(false, true) {
				fmt.Println()
				continue
			}
		case 5:
			if configureAgentTeams(false, true) {
				fmt.Println()
				continue
			}
		case 6:
			if configureVSCode(cfg, false, true) {
				fmt.Println()
				continue
			}
		case 7:
			configureAttribution(&attr)
			fmt.Println()
			// 不 continue：configureAttribution 返回即表示署名配置完毕，
			// 落到下方保存逻辑统一持久化（attr 已是最新编辑态）。
		}

		// 构造更新后的快照：沿用原名 → 覆盖同一文件；Teams/Effort 回读已播种的当前态
		newNC := NamedConfig{
			Config:      cfg,
			Name:        nc.Name,
			AgentTeams:  getManagedAgentTeamsValue(),
			EffortLevel: getManagedEffortLevelValue(),
			Attribution: attributionForSnapshot(attr),
			SavedAt:     time.Now().Format(time.RFC3339),
			AppVersion:  appVersion,
		}

		// 应用使其生效（内部含 saveConfig + 关闭语义清理）
		fmt.Println()
		if err := runWithSpinner("正在保存配置...", func() error {
			return applyNamedConfig(newNC)
		}); err != nil {
			printError(fmt.Sprintf("保存配置失败: %v", err))
			return
		}
		printSuccess("保存成功!")

		// 覆盖命名配置文件（沿用原名）
		if path, err := saveNamedConfig(newNC); err != nil {
			printWarning(fmt.Sprintf("命名配置持久化失败: %v", err))
		} else {
			printSuccess(fmt.Sprintf("已更新命名配置: %s", path))
		}

		// 打印摘要
		printSummary(newNC.Config)
		return
	}
}

// applyNamedConfig 忠实还原命名配置快照到 Claude settings 与系统环境变量。
// 处理 buildManagedEnvMap"只增不删"约束：快照未启用的开关需显式从持久层移除。
func applyNamedConfig(nc NamedConfig) error {
	// 1) 先把开关注入当前进程 env，供 saveConfig→buildManagedEnvMap 读取
	if nc.EffortLevel != "" {
		os.Setenv(envEffortLevel, normalizeEffortLevel(nc.EffortLevel))
	} else {
		os.Unsetenv(envEffortLevel)
	}
	if nc.AgentTeams != "" {
		os.Setenv(envAgentTeams, nc.AgentTeams)
	} else {
		os.Unsetenv(envAgentTeams)
	}

	// 2) 复用 saveConfig 写入 base/token/models +（因 env 已设）effort/agentteams
	if err := saveConfig(nc.Config); err != nil {
		return err
	}

	// 3) 关闭语义：快照未启用但系统/settings 可能残留旧值，显式移除
	if nc.EffortLevel == "" {
		_ = removeManagedEnvVar(envEffortLevel)
		_ = clearEffortFromClaudeSettings()
	}
	if nc.AgentTeams == "" {
		_ = removeManagedEnvVar(envAgentTeams)
		_ = saveClaudeSettingsConfigWithAgentTeams(nc.Config, "")
	}

	// 4) attribution 三态落定：必须放在上述 saveClaudeSettings* 调用之后，
	// 否则会被那些步骤回读的 getManagedAttribution()（文件旧值）覆盖。
	// nil 字段由 mergeClaudeAttribution 从 settings 删除，实现“快照未管理则清除残留”。
	attr := normalizeAttribution(nc.Attribution)
	if err := saveClaudeSettingsConfigWithAttribution(nc.Config, getManagedAgentTeamsValue(), attr); err != nil {
		return err
	}
	return nil
}

// runClearConfigMenu 清除配置入口菜单：清除所有 / 清除单个命名配置。
// 返回 back=true 表示用户按 ESC 返回主菜单。
func runClearConfigMenu() (back bool) {
	choice, b := runItemMenu("清除配置", []MenuItem{
		{"1", "清除所有配置", "删除全部命名配置 + 清除 Claude Code 配置"},
		{"2", "清除用户新增配置", "选择并删除某个已保存的命名配置"},
	}, true)
	if b {
		return true // 返回主菜单
	}
	switch choice {
	case 1:
		if !clearAllConfig() {
			return false // 用户在二次确认时取消，命名配置也不删除
		}
		if n, err := deleteAllNamedConfigs(); err != nil {
			printWarning(fmt.Sprintf("删除命名配置文件失败: %v", err))
		} else if n > 0 {
			fmt.Println()
			printSuccess(fmt.Sprintf("已删除 %d 个命名配置文件", n))
		}
	case 2:
		runDeleteNamedConfigMenu()
	}
	return false
}

// runDeleteNamedConfigMenu 平铺所有命名配置，让用户选择删除其中一个。
func runDeleteNamedConfigMenu() {
	configs, err := listNamedConfigs()
	if err != nil {
		printError(fmt.Sprintf("读取命名配置失败: %v", err))
		return
	}
	if len(configs) == 0 {
		printInfo("暂无已保存的命名配置")
		return
	}
	items := make([]MenuItem, len(configs))
	for i, c := range configs {
		items[i] = MenuItem{strconv.Itoa(i + 1), fmt.Sprintf("删除 %s 配置", c.Name), maskToken(c.AuthToken)}
	}
	idx, back := runItemMenu("选择要删除的配置", items, true)
	if back {
		return // 返回上一界面，不删除任何配置
	}
	target := configs[idx-1]
	fmt.Println()
	if ok, _ := styledConfirm(fmt.Sprintf("确定删除配置「%s」", target.Name), false); !ok {
		printInfo("已取消，未做任何更改")
		return
	}
	if err := deleteNamedConfig(target.FilePath); err != nil {
		printError(fmt.Sprintf("删除失败: %v", err))
		return
	}
	printSuccess(fmt.Sprintf("已删除配置「%s」", target.Name))
}
