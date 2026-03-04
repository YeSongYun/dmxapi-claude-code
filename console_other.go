//go:build !windows

package main

// initWindowsConsole 在非 Windows 平台上是空操作
func initWindowsConsole() {}

// readConsoleKey 在非 Windows 平台上是编译占位，运行时永不被调用
func readConsoleKey() KeyType {
	return KeyOther
}
