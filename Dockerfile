# 使用官方Go镜像作为构建环境
FROM golang:1.26.5-alpine AS builder

ARG UNIMAP_VERSION=dev
ARG UNIMAP_GIT_COMMIT=unknown
ARG UNIMAP_BUILD_TIME=unknown

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 安装 C 工具链（go-sqlite3 需要 CGO）
RUN apk add --no-cache build-base

# 复制源代码
COPY . .

# 构建应用（Web）
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X github.com/unimap/project/internal/appversion.Version=${UNIMAP_VERSION} -X github.com/unimap/project/internal/appversion.GitCommit=${UNIMAP_GIT_COMMIT} -X github.com/unimap/project/internal/appversion.BuildTime=${UNIMAP_BUILD_TIME}" \
    -o unimap-web ./cmd/unimap-web

# 使用alpine作为运行环境（固定版本）
FROM alpine:3.21

# 设置工作目录
WORKDIR /app

# 安装依赖（HTTPS + chromedp 截图需要 Chromium）
RUN apk add --no-cache ca-certificates chromium font-noto-cjk ttf-freefont

# 复制构建结果
COPY --from=builder /app/unimap-web /app/

# 复制配置文件
COPY configs /app/configs

# 允许镜像不依赖宿主机 bind mount 直接启动；生产环境仍可挂载自定义配置。
RUN cp /app/configs/config.docker.yaml /app/configs/config.yaml

# 复制Web文件
COPY web /app/web

# 创建非root用户
RUN addgroup -S unimap && adduser -S -G unimap -h /app unimap

# 设置目录所有权
RUN mkdir -p /app/data /app/screenshots /app/chrome-profile /app/logs /app/backups && chown -R unimap:unimap /app

ENV UNIMAP_CHROME_PATH=/usr/bin/chromium \
    UNIMAP_CHROME_USER_DATA_DIR=/app/chrome-profile \
    UNIMAP_DATA_DIR=/app/data

# 切换到非root用户
USER unimap:unimap

# 暴露端口
EXPOSE 8448

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8448/health/ready || exit 1

# 启动应用
CMD ["./unimap-web"]
