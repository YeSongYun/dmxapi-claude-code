//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/term"
)

// keyEvent 是 INPUT_RECORD.EventType 的键盘事件标识
const keyEvent = uint16(0x0001)

// 方向键/功能键虚拟键码
const (
	vkUp     = uint16(0x26)
	vkDown   = uint16(0x28)
	vkReturn = uint16(0x0D)
	vkEscape = uint16(0x1B)
)

// inputRecord 对应 Windows INPUT_RECORD 结构体（20 字节）
// 仅展开 KEY_EVENT_RECORD 字段（最大的 union 成员）
type inputRecord struct {
	EventType         uint16  // offset 0, 2 bytes
	_                 [2]byte // offset 2, 2 bytes padding
	BKeyDown          int32   // offset 4, 4 bytes (BOOL)
	WRepeatCount      uint16  // offset 8, 2 bytes
	WVirtualKeyCode   uint16  // offset 10, 2 bytes
	WVirtualScanCode  uint16  // offset 12, 2 bytes
	UnicodeChar       uint16  // offset 14, 2 bytes (WCHAR)
	DwControlKeyState uint32  // offset 16, 4 bytes
} // total: 20 bytes ✓

// kernel32 及其导出函数，整个包共用一次加载
var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle       = kernel32.NewProc("GetStdHandle")
	procReadConsole        = kernel32.NewProc("ReadConsoleInputW")
	procSetConsoleCP       = kernel32.NewProc("SetConsoleCP")
	procSetOutputCP        = kernel32.NewProc("SetConsoleOutputCP")
	procGetConsMode        = kernel32.NewProc("GetConsoleMode")
	procSetConsMode        = kernel32.NewProc("SetConsoleMode")
	procGetConsoleCP       = kernel32.NewProc("GetConsoleCP")
	procGetConsoleOutputCP = kernel32.NewProc("GetConsoleOutputCP")
	procGetACP             = kernel32.NewProc("GetACP")
)

// Windows 控制台兼容性运行时状态
var (
	// legacyConsoleMode 为 true 表示控制台不支持 ANSI/VT，需要走 ASCII 降级
	legacyConsoleMode bool
	// origInputCP / origOutputCP 保存 initWindowsConsole 调整前的代码页，用于退出时恢复
	origInputCP  uint32
	origOutputCP uint32
)

// readConsoleKey 使用 ReadConsoleInputW 直接读取原始键盘事件，
// 不依赖控制台模式标志，兼容 CMD、PowerShell、Windows Terminal
func readConsoleKey() KeyType {
	// STD_INPUT_HANDLE = (DWORD)(-10) = ^uint32(9)
	const stdInputHandle = uintptr(^uint32(9))
	hIn, _, _ := procGetStdHandle.Call(stdInputHandle)

	for {
		var rec inputRecord
		var n uint32
		ret, _, _ := procReadConsole.Call(
			hIn,
			uintptr(unsafe.Pointer(&rec)),
			1,
			uintptr(unsafe.Pointer(&n)),
		)
		if ret == 0 || n == 0 {
			continue
		}
		if rec.EventType != keyEvent {
			continue // 跳过鼠标/窗口调整等事件
		}
		if rec.BKeyDown == 0 {
			continue // 跳过键抬起事件
		}
		// Ctrl+C: UnicodeChar == 3
		if rec.UnicodeChar == 3 {
			if rawModeState != nil {
				term.Restore(int(syscall.Stdin), rawModeState)
				fmt.Println()
			}
			restoreConsole()
			os.Exit(130)
		}
		switch rec.WVirtualKeyCode {
		case vkUp:
			return KeyUp
		case vkDown:
			return KeyDown
		case vkReturn:
			return KeyEnter
		case vkEscape:
			return KeyEsc
		default:
			if rec.UnicodeChar == 'q' || rec.UnicodeChar == 'Q' {
				return KeyEsc
			}
			return KeyOther
		}
	}
}

// stdinDataReady 在 Windows 上是编译占位，Windows 使用 readConsoleKey 处理键盘输入
func stdinDataReady(timeoutMs int) bool {
	return true
}

// stdinBytesAvailable 在 Windows 上是编译占位，Windows 使用 readConsoleKey 处理键盘输入
func stdinBytesAvailable() int {
	return 0
}

// initWindowsConsole 初始化 Windows 控制台：保存原代码页、切 UTF-8、尝试启用 ANSI/VT。
// 返回的 restore 函数应在程序退出时调用以恢复原代码页。
// 若 VT 启用失败，设置 legacyConsoleMode=true 并调用 applyLegacyTheme 降级为 ASCII。
func initWindowsConsole() (restore func()) {
	// 1. 保存当前输入/输出代码页，退出时恢复
	inCP, _, _ := procGetConsoleCP.Call()
	outCP, _, _ := procGetConsoleOutputCP.Call()
	origInputCP = uint32(inCP)
	origOutputCP = uint32(outCP)

	// 2. 设置输入/输出代码页为 UTF-8 (65001)，解决中文乱码
	procSetConsoleCP.Call(65001)
	procSetOutputCP.Call(65001)

	// 3. 尝试启用 stdout 的 ANSI/VT 处理（ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004）
	// STD_OUTPUT_HANDLE = (DWORD)(-11) = 0xFFFFFFF5
	const stdOutputHandle = uintptr(^uint32(10))
	h, _, _ := procGetStdHandle.Call(stdOutputHandle)
	var mode uint32
	procGetConsMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	procSetConsMode.Call(h, uintptr(mode|0x0004))

	// 4. 再读一次 mode，确认 0x0004 位是否真正生效；未生效 = 老版控制台，进入兼容模式
	var verifyMode uint32
	procGetConsMode.Call(h, uintptr(unsafe.Pointer(&verifyMode)))
	if verifyMode&0x0004 == 0 {
		legacyConsoleMode = true
		applyLegacyTheme()
		fmt.Println("[compat] 检测到较旧的 Windows 控制台，已启用兼容模式；推荐使用 Windows Terminal 或升级至 Windows 10 1809+ 获取更好体验")
	}

	return restoreConsole
}

// restoreConsole 将控制台代码页恢复为 initWindowsConsole 调整前的值。
// 可被多次调用（幂等）；若 origInputCP/origOutputCP 为 0（未初始化），则跳过恢复。
func restoreConsole() {
	if origInputCP != 0 {
		procSetConsoleCP.Call(uintptr(origInputCP))
	}
	if origOutputCP != 0 {
		procSetOutputCP.Call(uintptr(origOutputCP))
	}
}

// getWindowsACP 返回当前系统活动代码页（ANSI Code Page），用于 CJK locale 检测。
// 常见值：936=GBK 简中、950=Big5 繁中、932=日文、949=朝文。
func getWindowsACP() uint32 {
	cp, _, _ := procGetACP.Call()
	return uint32(cp)
}
