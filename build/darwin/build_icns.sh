#!/usr/bin/env bash
# build/darwin/build_icns.sh
#
# Wails 默认只会把 build/appicon.png 拷成 icns,且只导出一组 @2x 尺寸,
# 在低 DPI 的 Finder 视图下 .app 图标退化成通用占位。
# 这个脚本用 build/appicon.png 重新打一份"@1x + @2x"全尺寸 icns,
# 覆盖 Wails 打包时生成的那份。需要在 wails build 之后运行。
#
# 路径解析不依赖调用方 cwd：通过 BASH_SOURCE 反推自身位置,再回退一级得到项目根。
set -euo pipefail

SELF="${BASH_SOURCE[0]:-$0}"
ROOT="$(cd "$(dirname "$SELF")/../.." && pwd)"
SRC="${ROOT}/build/appicon.png"
OUT="${ROOT}/build/bin/clipsync.app/Contents/Resources/iconfile.icns"

if [[ ! -f "$SRC" ]]; then
  echo "[build_icns] 未找到 $SRC,跳过。" >&2
  exit 0
fi
if [[ ! -f "$OUT" ]]; then
  echo "[build_icns] 未找到 $OUT,跳过。" >&2
  exit 0
fi

WORK="$(mktemp -d)/clipsync.iconset"
mkdir -p "$WORK"

# @1x (Finder 默认显示必需)
for sz in 16 32 64 128 256 512; do
  sips -z "$sz" "$sz" "$SRC" --out "$WORK/icon_${sz}x${sz}.png" >/dev/null
done
# @2x (Retina 显示)
sips -z 32   32   "$SRC" --out "$WORK/icon_16x16@2x.png"   >/dev/null
sips -z 64   64   "$SRC" --out "$WORK/icon_32x32@2x.png"   >/dev/null
sips -z 256  256  "$SRC" --out "$WORK/icon_128x128@2x.png" >/dev/null
sips -z 512  512  "$SRC" --out "$WORK/icon_256x256@2x.png" >/dev/null
sips -z 1024 1024 "$SRC" --out "$WORK/icon_512x512@2x.png" >/dev/null

iconutil -c icns "$WORK" -o "$OUT"
echo "[build_icns] 重新生成 $OUT (含 @1x + @2x 全尺寸)"