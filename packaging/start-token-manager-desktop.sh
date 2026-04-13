#!/bin/sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ -d "$DIR/Token Manager Tools.app" ]; then
  open "$DIR/Token Manager Tools.app"
  exit 0
fi

if [ -x "$DIR/token-manager-desktop" ]; then
  "$DIR/token-manager-desktop" >/dev/null 2>&1 &
  exit 0
fi

printf '%s\n' '当前包里没有桌面客户端。请使用浏览器入口或下载桌面预览包。'
exit 1
