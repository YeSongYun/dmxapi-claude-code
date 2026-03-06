//go:build !windows

package main

import (
	"syscall"
	"unsafe"
)

// initWindowsConsole 在非 Windows 平台上是空操作
func initWindowsConsole() {}

// readConsoleKey 在非 Windows 平台上是编译占位，运行时永不被调用
func readConsoleKey() KeyType {
	return KeyOther
}

// pollFd 对应 struct pollfd（poll(2) 使用）
type pollFd struct {
	Fd      int32
	Events  int16
	Revents int16
}

// stdinDataReady 使用 poll(2) 检查 stdin 在指定超时内是否有数据可读。
// 用于区分单独按下 ESC 和以 ESC 开头的方向键序列。
func stdinDataReady(timeoutMs int) bool {
	fds := [1]pollFd{{Fd: 0, Events: 1}} // POLLIN = 1
	r, _, errno := syscall.Syscall(syscall.SYS_POLL, uintptr(unsafe.Pointer(&fds[0])), 1, uintptr(timeoutMs))
	return errno == 0 && r > 0
}
