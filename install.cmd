@echo off
setlocal

set VERSION=v1.5.5

rem Detect architecture
set ARCH=%PROCESSOR_ARCHITECTURE%
if /i "%ARCH%"=="AMD64" goto :arch_ok
if /i "%ARCH%"=="ARM64" goto :arch_ok
echo Unsupported architecture: %ARCH%
exit /b 1

:arch_ok
rem ARM64 Windows runs AMD64 binaries via x64 emulation layer
set FILENAME=dmxapi-claude-code-%VERSION%-windows-amd64.exe
set URL=https://cnb.cool/dmxapi/dmxapi_claude_code/-/releases/download/%VERSION%/%FILENAME%
set TMP_FILE=%TEMP%\%FILENAME%

echo Downloading %FILENAME%...
curl -fsSL "%URL%" -o "%TMP_FILE%"
if errorlevel 1 goto :download_failed

echo Starting configuration tool...
"%TMP_FILE%"

del /f "%TMP_FILE%" 2>nul
endlocal
goto :eof

:download_failed
echo Download failed. Please check your network or download manually:
echo %URL%
exit /b 1
