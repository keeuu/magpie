#!/usr/bin/env bash
# 构建你自己的 magpie.app，并替换当前跑着的那个。
#
#   ./dev-build.sh            构建 + 用本地签名签好 + 装到 ~/Applications
#   ./dev-build.sh --cli      只构建 CLI（更快，2 分钟）
#   ./dev-build.sh --inplace  直接覆盖 /Applications/magpie.app（需要你的密码/权限）
#
# 构建出来的 app 是 adhoc 签名，TeamIdentifier 为空，所以 magpie 自带的
# 自动更新（sameSigner）会拒绝官方的 notarised 构建来替换它 —— 你的改动
# 不会被官方更新冲掉。想升级官方版本时，跑 sync-upstream.sh 重新编译。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
export PATH="/opt/homebrew/bin:$PATH"

DEST="$HOME/Applications"

case "${1:-}" in
  --cli)
    echo "→ 构建 CLI（nogui，无 cgo，最快）…"
    go build -tags nogui -trimpath -ldflags="-s -w" -o magpie-cli .
    echo "✓ ./magpie-cli  ($(du -h magpie-cli | cut -f1))"
    exit 0
    ;;
  --inplace) DEST="/Applications" ;;
esac

echo "→ 编译 GUI 版（cgo + Wails，约 2 分钟）…"
go build -tags production -trimpath -ldflags="-s -w" -o magpie .

echo "→ 打包 magpie.app…"
rm -rf magpie.app
mkdir -p magpie.app/Contents/MacOS magpie.app/Contents/Resources
cp magpie magpie.app/Contents/MacOS/magpie
cp build/darwin/magpie.icns magpie.app/Contents/Resources/magpie.icns
cp build/darwin/Assets.car magpie.app/Contents/Resources/Assets.car
VERSION=$(git describe --tags --always 2>/dev/null | tr -cd 'A-Za-z0-9._+-')
sed "s/@VERSION@/$VERSION/" build/darwin/Info.plist > magpie.app/Contents/Info.plist

echo "→ 本地 adhoc 签名（TeamIdentifier 为空 → 官方更新不会覆盖它）…"
codesign --force --deep --sign - magpie.app

echo "→ 安装到 $DEST…"
mkdir -p "$DEST"
rm -rf "$DEST/magpie.app"
cp -R magpie.app "$DEST/magpie.app"

# 正在跑的旧实例会占着 127.0.0.1:3425，先让它退出
if pgrep -f "magpie.app/Contents/MacOS/magpie" >/dev/null 2>&1; then
  echo "→ 停掉正在运行的 magpie…"
  pkill -f "magpie.app/Contents/MacOS/magpie" || true
  sleep 2
fi

echo "→ 启动…"
open "$DEST/magpie.app"

# 登录自启（LaunchAgent）还指着旧路径的话改过来
PLIST="$HOME/Library/LaunchAgents/com.yetone.magpie.plist"
if [ -f "$PLIST" ]; then
  NEW="$DEST/magpie.app/Contents/MacOS/magpie"
  /usr/libexec/PlistBuddy -c "Set :ProgramArguments:0 $NEW" "$PLIST" 2>/dev/null && \
    echo "✓ 登录自启指向 $NEW" || echo "! 登录自启没改成功，去 系统设置 → 通用 → 登录项 里看一下"
fi

echo
echo "✓ 完成。版本：$(/usr/bin/strings "$DEST/magpie.app/Contents/MacOS/magpie" | grep -m1 '^v0\.' || echo "$VERSION")"
echo "  签名：$(codesign -dv "$DEST/magpie.app" 2>&1 | grep -i TeamIdentifier || echo 'adhoc')"
