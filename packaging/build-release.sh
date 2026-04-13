#!/bin/sh
set -eu

DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$DIR/.." && pwd)
VERSION=$(cat "$ROOT/VERSION")
DIST_DIR="$ROOT/dist"
ASSETS_DIR="$ROOT/assets/icons"

ICON_PNG="$ASSETS_DIR/token-manager-tools.png"
ICON_ICO="$ASSETS_DIR/token-manager-tools.ico"
ICON_ICNS="$ASSETS_DIR/token-manager-tools.icns"

if [ ! -f "$ICON_PNG" ] || [ ! -f "$ICON_ICO" ] || [ ! -f "$ICON_ICNS" ]; then
  python3 "$DIR/generate-app-icon.py"
fi

cd "$ROOT"

build_cli_binary() {
  GOOS="$1" GOARCH="$2" CGO_ENABLED=0 go build -o "$3" ./cmd/token-manager
}

build_desktop_windows() {
  GOARCH="$1" GOOS=windows GOARCH="$1" CGO_ENABLED=0 go build -tags production -o "$2" ./cmd/token-manager-desktop
}

build_desktop_darwin() {
  GOARCH="$1" TOKEN_MANAGER_DESKTOP_OUT_DIR="$2" GOOS=darwin GOARCH="$1" "$DIR/build-token-manager-desktop.sh" >/dev/null
}

copy_common_files() {
  TARGET_DIR="$1"
  cp "$ROOT/docs/USER_README_zh.txt" "$TARGET_DIR/README-zh.txt"
}

copy_cli_scripts() {
  GOOS_NAME="$1"
  TARGET_DIR="$2"
  case "$GOOS_NAME" in
    windows)
      cp "$ROOT/packaging/start-token-manager.bat" "$TARGET_DIR/"
      cp "$ROOT/packaging/status-token-manager.bat" "$TARGET_DIR/"
      cp "$ROOT/packaging/stop-token-manager.bat" "$TARGET_DIR/"
      ;;
    *)
      cp "$ROOT/packaging/start-token-manager.sh" "$TARGET_DIR/"
      cp "$ROOT/packaging/status-token-manager.sh" "$TARGET_DIR/"
      cp "$ROOT/packaging/stop-token-manager.sh" "$TARGET_DIR/"
      chmod +x "$TARGET_DIR/start-token-manager.sh" "$TARGET_DIR/status-token-manager.sh" "$TARGET_DIR/stop-token-manager.sh"
      ;;
  esac
}

copy_desktop_launcher() {
  GOOS_NAME="$1"
  TARGET_DIR="$2"
  case "$GOOS_NAME" in
    windows)
      cp "$ROOT/packaging/start-token-manager-desktop.bat" "$TARGET_DIR/"
      ;;
    darwin)
      cp "$ROOT/packaging/start-token-manager-desktop.sh" "$TARGET_DIR/"
      chmod +x "$TARGET_DIR/start-token-manager-desktop.sh"
      ;;
  esac
}

write_macos_app_bundle() {
  TARGET_DIR="$1"
  DESKTOP_BIN="$2"
  APP_DIR="$TARGET_DIR/Token Manager Tools.app"
  CONTENTS_DIR="$APP_DIR/Contents"
  mkdir -p "$CONTENTS_DIR/MacOS" "$CONTENTS_DIR/Resources"
  cp "$DESKTOP_BIN" "$CONTENTS_DIR/MacOS/token-manager-desktop"
  chmod +x "$CONTENTS_DIR/MacOS/token-manager-desktop"
  cp "$ICON_ICNS" "$CONTENTS_DIR/Resources/token-manager-tools.icns"
  cat >"$CONTENTS_DIR/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>zh_CN</string>
    <key>CFBundleDisplayName</key>
    <string>Token Manager Tools</string>
    <key>CFBundleExecutable</key>
    <string>token-manager-desktop</string>
    <key>CFBundleIconFile</key>
    <string>token-manager-tools.icns</string>
    <key>CFBundleIdentifier</key>
    <string>com.henryinsomniac.token-manager-tools.desktop</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Token Manager Tools</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>NSHighResolutionCapable</key>
    <true/>
  </dict>
</plist>
EOF
}

make_zip() {
  TARGET_DIR="$1"
  ARCHIVE_PATH="$2"
  (cd "$DIST_DIR" && zip -qry "$ARCHIVE_PATH" "$(basename "$TARGET_DIR")")
}

package_platform() {
  GOOS_NAME="$1"
  GOARCH_NAME="$2"
  PKG_DIR="$DIST_DIR/token-manager-tools-$VERSION-$GOOS_NAME-$GOARCH_NAME"
  CLI_NAME="token-manager"
  if [ "$GOOS_NAME" = "windows" ]; then
    CLI_NAME="token-manager.exe"
  fi

  rm -rf "$PKG_DIR"
  mkdir -p "$PKG_DIR"

  copy_common_files "$PKG_DIR"
  copy_cli_scripts "$GOOS_NAME" "$PKG_DIR"

  build_cli_binary "$GOOS_NAME" "$GOARCH_NAME" "$PKG_DIR/$CLI_NAME"

  case "$GOOS_NAME" in
    darwin)
      DESKTOP_TMP="$DIST_DIR/desktop-$VERSION-$GOOS_NAME-$GOARCH_NAME"
      rm -rf "$DESKTOP_TMP"
      mkdir -p "$DESKTOP_TMP"
      build_desktop_darwin "$GOARCH_NAME" "$DESKTOP_TMP"
      write_macos_app_bundle "$PKG_DIR" "$DESKTOP_TMP/token-manager-desktop"
      copy_desktop_launcher "$GOOS_NAME" "$PKG_DIR"
      cp "$ICON_PNG" "$PKG_DIR/app-icon.png"
      rm -rf "$DESKTOP_TMP"
      ;;
    windows)
      build_desktop_windows "$GOARCH_NAME" "$PKG_DIR/token-manager-desktop.exe"
      copy_desktop_launcher "$GOOS_NAME" "$PKG_DIR"
      cp "$ICON_ICO" "$PKG_DIR/app-icon.ico"
      cp "$ICON_PNG" "$PKG_DIR/app-icon.png"
      ;;
    linux)
      cp "$ICON_PNG" "$PKG_DIR/app-icon.png"
      ;;
  esac

  make_zip "$PKG_DIR" "$(basename "$PKG_DIR").zip"
}

package_platform darwin amd64
package_platform darwin arm64
package_platform linux amd64
package_platform linux arm64
package_platform windows amd64
package_platform windows arm64

(
  cd "$DIST_DIR"
  rm -f SHA256SUMS.txt
  shasum -a 256 \
    "token-manager-tools-$VERSION-darwin-amd64.zip" \
    "token-manager-tools-$VERSION-darwin-arm64.zip" \
    "token-manager-tools-$VERSION-linux-amd64.zip" \
    "token-manager-tools-$VERSION-linux-arm64.zip" \
    "token-manager-tools-$VERSION-windows-amd64.zip" \
    "token-manager-tools-$VERSION-windows-arm64.zip" > SHA256SUMS.txt
)

printf '%s\n' "已生成 release 资产：$DIST_DIR"
