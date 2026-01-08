# 跨平台编译脚本
Write-Host "开始编译所有平台版本..." -ForegroundColor Cyan

# Windows AMD64
Write-Host "编译 Windows AMD64..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o dmxapi-claude-code.exe dmxapi-claude-code.go
Write-Host "Windows AMD64 完成" -ForegroundColor Green

# Linux AMD64
Write-Host "编译 Linux AMD64..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o dmxapi-claude-code-linux-amd64 dmxapi-claude-code.go
Write-Host "Linux AMD64 完成" -ForegroundColor Green

# Linux ARM64
Write-Host "编译 Linux ARM64..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "arm64"
go build -o dmxapi-claude-code-linux-arm64 dmxapi-claude-code.go
Write-Host "Linux ARM64 完成" -ForegroundColor Green

# macOS AMD64 (Intel)
Write-Host "编译 macOS AMD64 (Intel)..." -ForegroundColor Yellow
$env:GOOS = "darwin"
$env:GOARCH = "amd64"
go build -o dmxapi-claude-code-macos-amd64 dmxapi-claude-code.go
Write-Host "macOS AMD64 完成" -ForegroundColor Green

# macOS ARM64 (Apple Silicon)
Write-Host "编译 macOS ARM64 (Apple Silicon)..." -ForegroundColor Yellow
$env:GOOS = "darwin"
$env:GOARCH = "arm64"
go build -o dmxapi-claude-code-macos-arm64 dmxapi-claude-code.go
Write-Host "macOS ARM64 完成" -ForegroundColor Green

# 重置环境变量
$env:GOOS = ""
$env:GOARCH = ""

Write-Host "`n所有平台编译完成!" -ForegroundColor Cyan
Write-Host "编译产物:" -ForegroundColor Cyan
Get-ChildItem -Name dmxapi-claude-code*
