@echo off
chcp 65001 > nul
echo ================================
echo    Go 跨平台编译脚本
echo ================================
echo.

set APP_NAME=instrument-server
set VERSION=v1.0.0
set BUILD_DIR=build

:: 清理旧文件
if exist %BUILD_DIR% (
    echo 🧹 清理旧文件...
    rd /s /q %BUILD_DIR%
)
mkdir %BUILD_DIR%

echo.
echo 🚀 开始编译 %APP_NAME% %VERSION%
echo.

:: Linux 64位
echo 🔨 编译 Linux amd64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o %BUILD_DIR%/%APP_NAME%-linux-amd64
if %errorlevel% neq 0 goto :error
echo    ✅ 成功!
echo.

:: Windows 64位
echo 🔨 编译 Windows amd64...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o %BUILD_DIR%/%APP_NAME%-windows-amd64.exe
if %errorlevel% neq 0 goto :error
echo    ✅ 成功!
echo.

:: macOS Intel
echo 🔨 编译 macOS amd64...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o %BUILD_DIR%/%APP_NAME%-darwin-amd64
if %errorlevel% neq 0 goto :error
echo    ✅ 成功!
echo.

:: macOS Apple Silicon
echo 🔨 编译 macOS arm64...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags="-s -w" -o %BUILD_DIR%/%APP_NAME%-darwin-arm64
if %errorlevel% neq 0 goto :error
echo    ✅ 成功!
echo.

echo ================================
echo ✅ 编译完成!
echo ================================
echo 📁 输出目录: %BUILD_DIR%\
echo.
dir /b %BUILD_DIR%
echo.
pause
exit /b 0

:error
echo.
echo ❌ 编译失败!
echo.
pause
exit /b 1
