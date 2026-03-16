$VERSION = "v1.4.9"

# 设置控制台输出为 UTF-8，确保中文正常显示（在 Go exe 启动前生效）
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

# 检测架构
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -ne "AMD64" -and $arch -ne "ARM64") {
    Write-Host "不支持的架构: $arch" -ForegroundColor Red
    exit 1
}

# ARM64 Windows 通过 x64 模拟层运行 AMD64 二进制文件
$filename = "dmxapi-claude-code-$VERSION-windows-amd64.exe"
$url = "https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/$VERSION/$filename"
$tmpFile = Join-Path $env:TEMP $filename

Write-Host "正在下载 $filename..."
try {
    Invoke-WebRequest -Uri $url -OutFile $tmpFile -UseBasicParsing
} catch {
    Write-Host "下载失败：$_" -ForegroundColor Red
    exit 1
}

Write-Host "正在启动配置工具..."
# 使用 Start-Process -NoNewWindow 让 exe 直接继承控制台句柄，
# 避免通过 PowerShell 管道传输输出（否则 ANSI 颜色和交互式 TUI 无法正常显示）
Start-Process -FilePath $tmpFile -NoNewWindow -Wait

Remove-Item -Force $tmpFile -ErrorAction SilentlyContinue
