@echo off
setlocal enabledelayedexpansion
chcp 65001 > nul

:: ================================
:: 配置区域
:: ================================
set APP_NAME=instrument-server
set BUILD_DIR=build
set MAIN_FILE=./cmd/server

:: 获取 Git 信息
for /f "delims=" %%i in ('git describe --tags --always --dirty 2^>nul') do set GIT_VERSION=%%i
if not defined GIT_VERSION set GIT_VERSION=v0.0.0

for /f "delims=" %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i
if not defined GIT_COMMIT set GIT_COMMIT=unknown

:: 获取当前时间
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set datetime=%%I
set BUILD_TIME=%datetime:~0,4%-%datetime:~4,2%-%datetime:~6,2%T%datetime:~8,2%:%datetime:~10,2%:%datetime:~12,2%

:: 编译选项 (注意：包路径前缀根据你的 go.mod 中的 module 名称调整)
set LDFLAGS=-s -w -X main.Version=%GIT_VERSION% -X main.BuildTime=%BUILD_TIME% -X main.GitCommit=%GIT_COMMIT%

:: ================================
:: 显示信息
:: ================================
cls
echo.
echo ========================================
echo    Go 跨平台编译脚本
echo ========================================
echo.
echo 应用名称: %APP_NAME%
echo 版本信息: %GIT_VERSION%
echo 构建时间: %BUILD_TIME%
echo Git提交:  %GIT_COMMIT%
echo.
echo ========================================
echo.

:: 清理旧文件
if exist %BUILD_DIR% (
    echo 🧹 清理旧文件...
    rd /s /q %BUILD_DIR%
    echo    ✅ 清理完成!
    echo.
)
mkdir %BUILD_DIR%

:: ================================
:: 编译各平台
:: ================================

:: Linux amd64
call :build linux amd64 ""
if %errorlevel% neq 0 goto :error

:: Linux arm64
call :build linux arm64 ""
if %errorlevel% neq 0 goto :error

:: Windows amd64
call :build windows amd64 ".exe"
if %errorlevel% neq 0 goto :error

:: Windows 386
call :build windows 386 ".exe"
if %errorlevel% neq 0 goto :error

:: macOS amd64
call :build darwin amd64 ""
if %errorlevel% neq 0 goto :error

:: macOS arm64
call :build darwin arm64 ""
if %errorlevel% neq 0 goto :error

:: ================================
:: 生成校验和
:: ================================
echo 🔐 生成校验和...
cd %BUILD_DIR%

echo # SHA256 Checksums > checksums.txt
echo # Generated at %BUILD_TIME% >> checksums.txt
echo. >> checksums.txt

for %%f in (%APP_NAME%-*) do (
    echo 处理: %%f
    certutil -hashfile "%%f" SHA256 | find /v ":" | find /v "CertUtil" >> checksums.txt
    echo %%f >> checksums.txt
    echo. >> checksums.txt
)

cd ..
echo    ✅ 完成!
echo.


:: ================================
:: 完成
:: ================================
echo ========================================
echo ✅ 编译完成!
echo ========================================
echo 📁 输出目录: %BUILD_DIR%\
echo.
dir %BUILD_DIR%
echo.
echo 按任意键退出...
pause > nul
exit /b 0

:: ================================
:: 编译函数
:: ================================
:build
set OS=%~1
set ARCH=%~2
set EXT=%~3
set OUTPUT=%BUILD_DIR%\%APP_NAME%-%OS%-%ARCH%%EXT%

echo 🔨 编译 %OS%/%ARCH%...

set GOOS=%OS%
set GOARCH=%ARCH%
set CGO_ENABLED=0

go build -ldflags="%LDFLAGS%" -trimpath -o "%OUTPUT%" %MAIN_FILE%

if %errorlevel% neq 0 (
    echo    ❌ 失败!
    echo.
    echo 调试信息:
    echo   GOOS=%OS%
    echo   GOARCH=%ARCH%
    echo   输出=%OUTPUT%
    echo   源码=%MAIN_FILE%
    echo.
    exit /b 1
)

:: 显示文件大小
for %%A in ("%OUTPUT%") do set SIZE=%%~zA
set /a SIZE_MB=!SIZE! / 1048576
set /a SIZE_KB=!SIZE! / 1024

if !SIZE_MB! gtr 0 (
    echo    ✅ 成功! 大小: !SIZE_MB! MB
) else (
    echo    ✅ 成功! 大小: !SIZE_KB! KB
)
echo.

exit /b 0

:: ================================
:: 错误处理
:: ================================
:error
echo.
echo ========================================
echo ❌ 编译失败!
echo ========================================
echo.
pause
exit /b 1
