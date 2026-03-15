//go:build !windows

package main

import (
	"runtime"
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

// stdinBytesAvailable 使用 FIONREAD ioctl 查询 stdin 内核缓冲区中立即可读的字节数。
// 与 poll 超时方案相比，此方法无等待时间，且在 curl|bash（</dev/tty）场景下
// 不受文件描述符打开方式影响，更加可靠。
// FIONREAD 的 ioctl 编号在 macOS 和 Linux 上不同，通过 runtime.GOOS 区分。
func stdinBytesAvailable() int {
	var req uint
	if runtime.GOOS == "darwin" {
		req = 0x4004667F // macOS FIONREAD（<sys/filio.h>）
	} else {
		req = 0x541B // Linux FIONREAD/TIOCINQ（amd64/arm64）
	}
	n, err := unix.IoctlGetInt(int(syscall.Stdin), req)
	if err != nil {
		return 0
	}
	return n
}
