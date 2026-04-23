# syntax=docker/dockerfile:1.6
#
# 多阶段构建 Dockerfile —— 适用于 Zeabur / GitHub 直接部署
# ============================================================
# 从源码构建后端 + 前端,无需宿主机预编译。
# 迁移 SQL 已内嵌到 Go 二进制中,启动时自动建表。
# ============================================================

# ---- Stage 1: Build Go backend ----
FROM golang:1.26-alpine AS go-builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY . .
# go mod tidy 确保 go.sum 完整(适配新增依赖后首次构建)
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w" -o /out/gpt2api ./cmd/server

# ---- Stage 2: Build frontend ----
FROM node:20-alpine AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund --loglevel=error

COPY web/ .
RUN npm run build

# ---- Stage 3: Runtime ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl \
    && update-ca-certificates \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# Go 二进制(内嵌迁移 SQL)
COPY --from=go-builder /out/gpt2api /app/gpt2api
RUN chmod +x /app/gpt2api

# 前端构建产物
COPY --from=web-builder /web/dist /app/web/dist

# 默认配置(可选,Zeabur 上走环境变量即可不需要此文件)
COPY configs/config.example.yaml /app/configs/config.yaml

RUN mkdir -p /app/data/backups /app/logs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1

CMD ["/app/gpt2api", "-c", "/app/configs/config.yaml"]
