#!/bin/sh
# 把 wails build 的 Linux 产物打成 AppImage。
#
# 前置：
#   1. 已在 Linux 上完成 `wails build -platform linux/<arch> -tags webkit2_41`，
#      产物位于 build/bin/ClipSync；
#   2. PATH 中能找到 appimagetool（构建镜像 build/linux/Dockerfile 阶段
#      已装到 /usr/local/bin；CI 也由 GitHub Action apt 步骤装）。
# 用 --appimage-extract-and-run 调用 appimagetool，无需目标机安装 FUSE。
set -eu
cd "$(dirname "$0")/../.."

VERSION="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' wails.json | head -n1)"
if [ -z "$VERSION" ]; then
  echo "无法从 wails.json 解析版本号" >&2
  exit 1
fi

ARCH="${APPIMAGE_ARCH_OVERRIDE:-$(uname -m)}"
case "$ARCH" in
  x86_64) APPIMAGE_ARCH=x86_64 ;;
  aarch64 | arm64) APPIMAGE_ARCH=aarch64 ;;
  *)
    echo "不支持的架构: $ARCH" >&2
    exit 1
    ;;
esac

if [ ! -x "build/bin/ClipSync" ]; then
  echo "找不到 build/bin/ClipSync，请先执行 wails build -platform linux/$APPIMAGE_ARCH -tags webkit2_41" >&2
  exit 1
fi

# 组装 AppDir：AppRun + desktop 条目 + 图标 + 真正的二进制。
APPDIR="build/linux/ClipSync.AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin"
cp "build/bin/ClipSync" "$APPDIR/usr/bin/ClipSync"
# 内置 wl-clipboard 的两个工具：纯 Wayland 会话（无 XWayland）下剪贴板
# 走它们实现。AppRun 已把 AppDir/usr/bin 加入 PATH，运行时自动可见；
# 打包机没装 wl-clipboard 时跳过，仅影响纯 Wayland 环境的开箱体验。
for bin in wl-copy wl-paste; do
  if command -v "$bin" >/dev/null 2>&1; then
    cp "$(command -v "$bin")" "$APPDIR/usr/bin/"
  else
    echo "警告: 未找到 ${bin}，纯 Wayland 会话下需用户自行安装 wl-clipboard" >&2
  fi
done
cp "build/linux/AppRun" "$APPDIR/AppRun"
chmod +x "$APPDIR/AppRun"
cp "build/linux/clipsync.desktop" "$APPDIR/clipsync.desktop"
cp "build/appicon.png" "$APPDIR/clipsync.png"
ln -sf "clipsync.png" "$APPDIR/.DirIcon"

if ! command -v appimagetool >/dev/null 2>&1; then
  echo "未找到 appimagetool，请先安装（见 build/linux/Dockerfile）" >&2
  exit 1
fi

OUT="build/bin/ClipSync-$VERSION-linux-$APPIMAGE_ARCH.AppImage"
appimagetool --appimage-extract-and-run "$APPDIR" "$OUT"
echo "已生成 $OUT"
