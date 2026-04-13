#!/bin/sh
set -eu

DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$DIR/.." && pwd)

GOOS_VALUE=${GOOS:-$(go env GOOS)}
GOARCH_VALUE=${GOARCH:-$(go env GOARCH)}
OUT_DIR=${TOKEN_MANAGER_DESKTOP_OUT_DIR:-"$ROOT/dist/desktop-local/$GOOS_VALUE-$GOARCH_VALUE"}
BIN_NAME=token-manager-desktop

if [ "$GOOS_VALUE" = "windows" ]; then
  BIN_NAME="$BIN_NAME.exe"
fi

mkdir -p "$OUT_DIR"

export CGO_ENABLED=${CGO_ENABLED:-1}

if [ "$GOOS_VALUE" = "darwin" ]; then
  case " ${CGO_CFLAGS-} " in
    *" -mmacosx-version-min=10.13 "*) ;;
    *) export CGO_CFLAGS="${CGO_CFLAGS-} -mmacosx-version-min=10.13" ;;
  esac
  case " ${CGO_LDFLAGS-} " in
    *" -framework UniformTypeIdentifiers "*) ;;
    *) export CGO_LDFLAGS="${CGO_LDFLAGS-} -framework UniformTypeIdentifiers -mmacosx-version-min=10.13" ;;
  esac
fi

cd "$ROOT"
go build -tags production -o "$OUT_DIR/$BIN_NAME" ./cmd/token-manager-desktop

printf '%s\n' "已生成桌面客户端：$OUT_DIR/$BIN_NAME"
