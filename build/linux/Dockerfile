# ClipSync Linux 构建镜像
#
# 与 release.yml 的 build-linux 作业同构：Ubuntu 24.04 + GTK3 + WebKitGTK 4.1。
# 目标架构由 --platform 决定（linux/amd64 或 linux/arm64，默认 amd64）。
# Apple Silicon 上 amd64 容器依赖 Docker Desktop / OrbStack 的 Rosetta 转译
# （纯 QEMU 全模拟会慢到不可用）；arm64 容器则原生运行，
# 本地即可产出 aarch64 AppImage。用法：sh build/linux/build_in_docker.sh [amd64|arm64]
ARG TARGETPLATFORM=linux/amd64
FROM --platform=${TARGETPLATFORM} ubuntu:24.04

# 重新声明后才能在构建阶段内引用（全局 ARG 只在 FROM 行可见）
ARG TARGETPLATFORM

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        file \
        git \
        pkg-config \
        libgtk-3-dev \
        libwebkit2gtk-4.1-dev \
        wl-clipboard \
    && rm -rf /var/lib/apt/lists/*

# Go 版本与 go.mod / CI 的 go-version-file 对齐；tarball 架构跟随容器平台。
ARG GO_VERSION=1.27.0
RUN set -eux; \
    case "${TARGETPLATFORM}" in \
        linux/arm64) GO_TARARCH=arm64 ;; \
        *) GO_TARARCH=amd64 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_TARARCH}.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Node 20，与 CI 的 node-version 对齐
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Wails CLI 版本与 CI 一致（v3：wails3 命令，构建走 Taskfile 体系）。
# 带 gtk3 标签：CLI 自身编译 internal/operatingsystem 时默认要 gtk4 /
# webkitgtk-6.0（本镜像只装了 GTK3 / 4.1 开发库），标签切到 gtk+-3.0 +
# webkit2gtk-4.1 才能编过（CI 的 build-linux 作业同理）。
ARG WAILS_VERSION=v3.0.0
RUN GOBIN=/usr/local/bin go install -tags gtk3 github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}

# appimagetool 在镜像构建时装好并改名为普通命令名，运行时直接调。
# 用 /releases/download/continuous/ 而不是 /latest/download/：后者在某些代理
# 环境下会被改写、返回 404，continuous 走 GitHub 永久 storage 桶更稳。
# 不能用 /latest 还有一个原因：Docker Desktop on Apple Silicon 用 Rosetta
# 转译运行 amd64 容器，runtime 下载的 x86_64 二进制不在镜像里、不会被
# 转译，exec 会报 "Exec format error"；放进镜像后 exec 翻译就生效。
# 下载 URL 跟随容器架构（x86_64 / aarch64），APPIMAGETOOL_URL 可整体覆盖。
ARG APPIMAGETOOL_URL=
RUN set -eux; \
    if [ -n "${APPIMAGETOOL_URL}" ]; then \
        url="${APPIMAGETOOL_URL}"; \
    else \
        case "${TARGETPLATFORM}" in \
            linux/arm64) url=https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-aarch64.AppImage ;; \
            *) url=https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage ;; \
        esac; \
    fi; \
    curl -fsSL -o /usr/local/bin/appimagetool "${url}" \
    && chmod 755 /usr/local/bin/appimagetool

WORKDIR /src
