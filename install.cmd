@echo off
setlocal

set VERSION=v1.4.7

rem 检测架构
if /i not "%PROCESSOR_ARCHITECTURE%"=="AMD64" (
    echo 当前仅支持 x64 架构，检测到：%PROCESSOR_ARCHITECTURE%
    exit /b 1
)

set FILENAME=dmxapi-claude-code-%VERSION%-windows-amd64.exe
set URL=https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/%VERSION%/%FILENAME%
set TMP_FILE=%TEMP%\%FILENAME%

echo 正在下载 %FILENAME%...
curl -fsSL "%URL%" -o "%TMP_FILE%"
if errorlevel 1 (
    echo 下载失败，请检查网络连接或手动下载：%URL%
    exit /b 1
)

echo 正在启动配置工具...
"%TMP_FILE%"

del /f "%TMP_FILE%" 2>nul
endlocal
