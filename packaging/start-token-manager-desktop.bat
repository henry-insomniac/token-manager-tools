@echo off
setlocal
cd /d "%~dp0"

if exist "%CD%\token-manager-desktop.exe" (
  start "" "%CD%\token-manager-desktop.exe"
  exit /b 0
)

echo 当前包里没有桌面客户端。请改用浏览器入口或下载包含桌面预览的分发包。
pause
exit /b 1
