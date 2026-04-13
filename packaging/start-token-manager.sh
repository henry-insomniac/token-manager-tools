#!/bin/sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
"$DIR/token-manager" start
printf '%s\n' '已启动。请打开 http://localhost:1455/'
