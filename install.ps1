$VERSION = "v1.4.6"

# 检测架构
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -ne "AMD64") {
    Write-Host "当前仅支持 x64 架构，检测到：$arch" -ForegroundColor Red
    exit 1
}

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
& $tmpFile

Remove-Item -Force $tmpFile -ErrorAction SilentlyContinue
