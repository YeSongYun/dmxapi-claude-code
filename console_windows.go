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
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle = kernel32.NewProc("GetStdHandle")
	procReadConsole  = kernel32.NewProc("ReadConsoleInputW")
	procSetConsoleCP = kernel32.NewProc("SetConsoleCP")
	procSetOutputCP  = kernel32.NewProc("SetConsoleOutputCP")
	procGetConsMode  = kernel32.NewProc("GetConsoleMode")
	procSetConsMode  = kernel32.NewProc("SetConsoleMode")
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
			os.Exit(0)
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

// initWindowsConsole 初始化 Windows 控制台：设置 UTF-8 代码页并启用 ANSI/VT 颜色处理
func initWindowsConsole() {
	// 1. 设置输入/输出代码页为 UTF-8 (65001)，解决中文乱码
	procSetConsoleCP.Call(65001)
	procSetOutputCP.Call(65001)

	// 2. 启用 stdout 的 ANSI/VT 处理（ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004）
	//    让颜色转义码（\033[31m 等）和框线字符正常渲染
	// STD_OUTPUT_HANDLE = (DWORD)(-11) = 0xFFFFFFF5
	// 注意：必须用 uintptr(^uint32(10)) 而非 ^uintptr(10)
	// 后者在 64 位系统为 0xFFFFFFFFFFFFFFF5，与 Windows DWORD 不符
	const stdOutputHandle = uintptr(^uint32(10))
	h, _, _ := procGetStdHandle.Call(stdOutputHandle)
	var mode uint32
	procGetConsMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	procSetConsMode.Call(h, uintptr(mode|0x0004))
}
