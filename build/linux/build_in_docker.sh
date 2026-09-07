#!/bin/sh
# 在 Docker 容器里完成 Linux 版构建与测试（与 release.yml 的 build-linux 作业同构）。
#
# 前置：Docker 已安装并启动。Apple Silicon 上用 Docker Desktop / OrbStack
# （默认 Rosetta 转译 amd64 容器）；纯 QEMU 模拟会非常慢。
#
# 流程：go mod tidy → wails build (linux/amd64 + webkit2_41) → go vet → go test
#       → AppImage 打包。产物落在仓库 build/bin/ 下。
#
# frontend/node_modules 由独立 named volume 遮蔽：容器内按 amd64 安装依赖，
# 不会污染宿主机的 arm64 node_modules；Go 模块与编译缓存同样放在 named
# volume 里跨次复用。
#
# 注意：AppImage 打包依赖 appimagetool 这个 type-1 ELF 二进制。Docker Desktop
# on Apple Silicon 的 x86_64 emulation 只翻译 PIE 二进制、不翻译非 PIE 旧格式
# ELF，所以 macOS Docker 上 AppImage 阶段会自动跳过、由 CI/Linux runner 完成。
# 真正想本地出 AppImage：在原生 Linux 机器、Linux VM、WSL2 或 CI 上调用
# build/linux/build_appimage.sh。
set -eu
cd "$(dirname "$0")/../.."

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

IMAGE=clipsync-build:linux-amd64
echo "==> 构建镜像 ${IMAGE}（首次执行需下载依赖，耐心等待）"
"$DOCKER" build --platform linux/amd64 $BUILD_PROXY_ARGS \
    -f build/linux/Dockerfile -t "$IMAGE" build/linux

echo "==> 容器内构建 + 测试"
"$DOCKER" run --rm --platform linux/amd64 \
    -v "$(pwd)":/src -w /src \
    -v clipsync-gomod:/root/go \
    -v clipsync-gocache:/root/.cache/go-build \
    -v clipsync-node:/src/frontend/node_modules \
    $RUN_PROXY_ARGS \
    "$IMAGE" bash -euo pipefail -c '
echo "==> [1/5] go mod tidy"
go mod tidy
echo "==> [2/5] wails build linux/amd64 (webkit2_41)"
wails build -platform linux/amd64 -tags webkit2_41
echo "==> [3/5] go vet"
go vet ./...
echo "==> [4/5] go test"
go test ./...
echo "==> [5/5] AppImage 打包"
# 探测 appimagetool 能否真在该仿真层跑：macOS Docker on Apple Silicon
# 不翻译非 PIE x86_64 ELF（appimagetool 是 type-1 旧格式），本地会报
# "Exec format error"。探测失败则跳过、由 CI/Linux runner 接管。
if ! appimagetool --appimage-version >/dev/null 2>&1; then
    echo "==> 跳过 AppImage 打包：appimagetool 在当前仿真层无法执行"
    echo "    (Docker Desktop on Apple Silicon 不翻译非 PIE x86_64 ELF)"
    echo "    本机裸二进制 build/bin/ClipSync 已就绪；AppImage 由 CI 发布"
    exit 0
fi
sh build/linux/build_appimage.sh
'
