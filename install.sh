#!/bin/sh
set -e

VERSION="v1.4.6"

# 检测操作系统
OS=$(uname -s)
case "$OS" in
  Darwin) OS_NAME="macos" ;;
  Linux)  OS_NAME="linux" ;;
  *)
    echo "不支持的操作系统: $OS"
    exit 1
    ;;
esac

# 检测架构
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH_NAME="amd64" ;;
  arm64|aarch64) ARCH_NAME="arm64" ;;
  *)
    echo "不支持的架构: $ARCH"
    exit 1
    ;;
esac

FILENAME="dmxapi-claude-code-${VERSION}-${OS_NAME}-${ARCH_NAME}"
URL="https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/${VERSION}/${FILENAME}"
TMP_FILE="/tmp/${FILENAME}"

echo "正在下载 ${FILENAME}..."
if ! curl -fsSL "$URL" -o "$TMP_FILE"; then
  echo "下载失败，请检查网络连接或手动下载：$URL"
  exit 1
fi

chmod +x "$TMP_FILE"

# macOS：移除 Gatekeeper 隔离标记
if [ "$OS_NAME" = "macos" ]; then
  xattr -cr "$TMP_FILE" 2>/dev/null || true
fi

echo "正在启动配置工具..."
"$TMP_FILE" </dev/tty

rm -f "$TMP_FILE"
