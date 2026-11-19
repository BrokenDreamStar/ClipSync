# ClipSync Linux 构建镜像
#
# 与 release.yml 的 build-linux 作业同构：Ubuntu 24.04 + GTK3 + WebKitGTK 4.1，
# 目标平台固定 linux/amd64。Apple Silicon 上依赖 Docker Desktop / OrbStack
# 的 Rosetta 转译运行 amd64 容器（纯 QEMU 全模拟会慢到不可用）。
# 用法：sh build/linux/build_in_docker.sh
ARG TARGETPLATFORM=linux/amd64
FROM --platform=${TARGETPLATFORM} ubuntu:24.04

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

# Go 版本与 go.mod / CI 的 go-version-file 对齐
ARG GO_VERSION=1.27.0
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Node 20，与 CI 的 node-version 对齐
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Wails CLI 版本与 CI 一致（v3：wails3 命令，构建走 Taskfile 体系）
ARG WAILS_VERSION=v3.0.0
RUN GOBIN=/usr/local/bin go install github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}

# appimagetool 在镜像构建时装好并改名为普通命令名，运行时直接调。
# 用 /releases/download/continuous/ 而不是 /latest/download/：后者在某些代理
# 环境下被改写、返回 404，continuous 走 GitHub 永久 storage 桶更稳。
# 不能用 /latest 还有一个原因：Docker Desktop on Apple Silicon 用 Rosetta
# 转译运行 amd64 容器，runtime 下载的 x86_64 二进制不在镜像里、不会被
# 转译，exec 会报 "Exec format error"；放进镜像后 exec 翻译就生效。
ARG APPIMAGETOOL_URL=https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
RUN curl -fsSL -o /usr/local/bin/appimagetool "${APPIMAGETOOL_URL}" \
    && chmod 755 /usr/local/bin/appimagetool

WORKDIR /src
