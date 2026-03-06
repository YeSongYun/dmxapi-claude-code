//go:build !windows

package main

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// initWindowsConsole 在非 Windows 平台上是空操作
func initWindowsConsole() {}

// readConsoleKey 在非 Windows 平台上是编译占位，运行时永不被调用
func readConsoleKey() KeyType {
	return KeyOther
}

// stdinDataReady 使用 poll(2) 检查 stdin 在指定超时内是否有数据可读。
// 用于区分单独按下 ESC 和以 ESC 开头的方向键序列。
func stdinDataReady(timeoutMs int) bool {
	fds := []unix.PollFd{{Fd: int32(syscall.Stdin), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, timeoutMs)
	return err == nil && n > 0
}
