@echo off
setlocal

cd /d "%~dp0.."

if "%GOOS%"=="" set "GOOS=windows"
if "%GOARCH%"=="" for /f "delims=" %%i in ('go env GOARCH') do set "GOARCH=%%i"
if "%TOKEN_MANAGER_DESKTOP_OUT_DIR%"=="" set "TOKEN_MANAGER_DESKTOP_OUT_DIR=%CD%\dist\desktop-local\%GOOS%-%GOARCH%"
if "%CGO_ENABLED%"=="" set "CGO_ENABLED=1"

if not exist "%TOKEN_MANAGER_DESKTOP_OUT_DIR%" mkdir "%TOKEN_MANAGER_DESKTOP_OUT_DIR%"

go build -tags production -o "%TOKEN_MANAGER_DESKTOP_OUT_DIR%\token-manager-desktop.exe" ./cmd/token-manager-desktop
if errorlevel 1 exit /b 1

echo 已生成桌面客户端：%TOKEN_MANAGER_DESKTOP_OUT_DIR%\token-manager-desktop.exe
