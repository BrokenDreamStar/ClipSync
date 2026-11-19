#!/bin/sh
# 在 Docker 容器里完成 Linux 版构建与测试（与 release.yml 的 build-linux 作业同构）。
#
# 用法：sh build/linux/build_in_docker.sh [amd64|arm64]（默认 amd64，
# 也可用环境变量 ARCH=arm64 指定）。
#
# 前置：Docker 已安装并启动。Apple Silicon 上用 Docker Desktop / OrbStack：
# amd64 容器默认走 Rosetta 转译（纯 QEMU 模拟会非常慢）；arm64 容器原生
# 运行，无需转译，Apple Silicon 上本地即可产出 aarch64 AppImage。
# Intel Mac 上构建 arm64 需 QEMU 全模拟，慢但可用。
#
# 流程：wails3 task linux:build (<arch> + gtk3) → go vet → go test
#       → AppImage 打包。产物落在仓库 build/bin/ 下。
#
# frontend/node_modules 由独立 named volume 遮蔽：容器内按目标架构安装依赖，
# 不会污染宿主机的 node_modules（esbuild / rollup 的原生二进制按架构分发）；
# Go 模块缓存（纯源码）跨架构共用，编译缓存按架构分卷。
#
# 注意：AppImage 打包依赖 appimagetool。amd64 路径上它是 type-1 旧格式 ELF，
# Docker Desktop on Apple Silicon 的 Rosetta 只翻译 PIE 二进制，exec 会报
# "Exec format error"，该场景 AppImage 阶段自动跳过、由 CI/Linux runner 完成；
# arm64 容器原生运行时无此问题，可直接打出 aarch64 AppImage。
set -eu
cd "$(dirname "$0")/../.."

# 架构优先级：命令行参数 > 环境变量 ARCH > amd64。
# 不用 ${1:-${ARCH:-amd64}} 嵌套默认值：set -u 下内层 ${ARCH:-…} 会先被求值，
# $ARCH 未设时直接报 unbound variable。
if [ "$#" -gt 0 ]; then ARCH="$1"; fi
ARCH="${ARCH:-amd64}"
case "$ARCH" in
    amd64 | arm64) ;;
    *)
        echo "不支持的架构: ${ARCH}（可选 amd64 / arm64）" >&2
        exit 1
        ;;
esac
PLATFORM="linux/$ARCH"

DOCKER="${DOCKER:-$(command -v docker || echo /Applications/Docker.app/Contents/Resources/bin/docker)}"

# 检测 macOS 系统代理并透传给构建/运行：代理工具（Clash 等）的 fake-ip DNS
# 会让容器直连外网时通时断，显式经宿主机代理出网最稳。已手动设置
# HTTP_PROXY 时以环境变量为准；无代理环境两项为空，行为不变。
BUILD_PROXY_ARGS=""
RUN_PROXY_ARGS=""
if [ -z "${HTTP_PROXY:-}" ] && [ "$(uname -s)" = "Darwin" ] && command -v scutil >/dev/null 2>&1; then
    PROXY=$(scutil --proxy | awk '
        /HTTPEnable/ { enable = $3 }
        /HTTPProxy/  { host = $3 }
        /HTTPPort/   { port = $3 }
        END { if (enable == 1 && host != "" && port != 0) print host ":" port }
    ')
    if [ -n "$PROXY" ]; then
        case "$PROXY" in
            127.0.0.1:* | 0.0.0.0:* | localhost:*)
                PROXY="host.docker.internal:${PROXY#*:}"
                ;;
        esac
        BUILD_PROXY_ARGS="--build-arg HTTP_PROXY=http://$PROXY --build-arg HTTPS_PROXY=http://$PROXY"
        RUN_PROXY_ARGS="-e HTTP_PROXY=http://$PROXY -e HTTPS_PROXY=http://$PROXY"
        echo "==> 检测到系统代理 ${PROXY}，构建与运行将经由它出网"
    fi
fi

IMAGE=clipsync-build:linux-$ARCH
echo "==> 构建镜像 ${IMAGE}（首次执行需下载依赖，耐心等待）"
"$DOCKER" build --platform "$PLATFORM" $BUILD_PROXY_ARGS \
    -f build/linux/Dockerfile -t "$IMAGE" build/linux

echo "==> 容器内构建 + 测试（${PLATFORM}）"
"$DOCKER" run --rm --platform "$PLATFORM" \
    -v "$(pwd)":/src -w /src \
    -v clipsync-gomod:/root/go \
    -v clipsync-gocache-$ARCH:/root/.cache/go-build \
    -v clipsync-node-$ARCH:/src/frontend/node_modules \
    $RUN_PROXY_ARGS \
    "$IMAGE" bash -euo pipefail -c '
echo "==> [1/4] wails3 task linux:build ('"$ARCH"', gtk3)"
# task 依赖链自动完成 go mod tidy / 绑定生成 / 前端构建 / 图标生成。
# v3 Linux GTK3 后端 pkg-config 用 gtk+-3.0 + webkit2gtk-4.1，标签为 gtk3
# （v2 时代的 webkit2_41 标签已废弃）。
wails3 task linux:build ARCH='"$ARCH"' EXTRA_TAGS=gtk3
echo "==> [2/4] go vet"
go vet ./...
echo "==> [3/4] go test"
go test ./...
echo "==> [4/4] AppImage 打包"
# 探测 appimagetool 能否真在该环境跑：Rosetta 转译的 amd64 容器里
# type-1 旧格式 ELF 不被翻译，本地会报 "Exec format error"。
# 探测失败则跳过、由 CI/Linux runner 接管（原生 arm64 容器不会走到这里）。
if ! appimagetool --appimage-version >/dev/null 2>&1; then
    echo "==> 跳过 AppImage 打包：appimagetool 在当前仿真层无法执行"
    echo "    (Docker Desktop on Apple Silicon 不翻译非 PIE x86_64 ELF)"
    echo "    本机裸二进制 build/bin/ClipSync 已就绪；AppImage 由 CI 发布"
    exit 0
fi
sh build/linux/build_appimage.sh
'
