@echo off
setlocal
cd /d "%~dp0"
token-manager.exe start
echo.
echo 已启动。请打开 http://localhost:1455/
pause
