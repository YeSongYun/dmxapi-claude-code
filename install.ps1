$VERSION = "v1.6.4"

# 设置控制台输出为 UTF-8，确保中文正常显示（在 Go exe 启动前生效）
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

# 检测架构
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -ne "AMD64" -and $arch -ne "ARM64") {
    Write-Error "Unsupported architecture: $arch"
    exit 1
}

# ARM64 Windows 通过 x64 模拟层运行 AMD64 二进制文件
$filename = "dmxapi-claude-code-$VERSION-windows-amd64.exe"
$url = "https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/$VERSION/$filename"
$tmpFile = Join-Path $env:TEMP $filename

Write-Host "Downloading $filename..."
try {
    Invoke-WebRequest -Uri $url -OutFile $tmpFile -UseBasicParsing
} catch {
    Write-Error "Download failed: $_"
    exit 1
}

Write-Host "Starting configuration tool..."
# 使用 Start-Process -NoNewWindow 让 exe 直接继承控制台句柄，
# 避免通过 PowerShell 管道传输输出（否则 ANSI 颜色和交互式 TUI 无法正常显示）
$process = Start-Process -FilePath $tmpFile -NoNewWindow -Wait -PassThru
$exitCode = $process.ExitCode

Remove-Item -Force $tmpFile -ErrorAction SilentlyContinue
if ($exitCode -ne 0) {
    throw "Configuration tool failed with exit code $exitCode"
}
