//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// initWindowsConsole 初始化 Windows 控制台：设置 UTF-8 代码页并启用 ANSI/VT 颜色处理
func initWindowsConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	// 1. 设置输入/输出代码页为 UTF-8 (65001)，解决中文乱码
	kernel32.NewProc("SetConsoleCP").Call(65001)
	kernel32.NewProc("SetConsoleOutputCP").Call(65001)

	// 2. 启用 stdout 的 ANSI/VT 处理（ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004）
	//    让颜色转义码（\033[31m 等）和框线字符正常渲染
	getStdHandle := kernel32.NewProc("GetStdHandle")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	// STD_OUTPUT_HANDLE = (DWORD)(-11) = 0xFFFFFFF5
	// 注意：必须用 uintptr(^uint32(10)) 而非 ^uintptr(10)
	// 后者在 64 位系统为 0xFFFFFFFFFFFFFFF5，与 Windows DWORD 不符
	const stdOutputHandle = uintptr(^uint32(10))
	h, _, _ := getStdHandle.Call(stdOutputHandle)
	var mode uint32
	getConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	setConsoleMode.Call(h, uintptr(mode|0x0004))
}
