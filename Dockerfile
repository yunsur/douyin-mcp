# syntax=docker/dockerfile:1.6

# ---- build stage ----
FROM golang:1.26 AS builder

WORKDIR /src
# 配置 Go 模块代理为国内源
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# VERSION 由 CI 通过 --build-arg 传入，本地构建默认 dev
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/douyin-mcp .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/douyin-login ./cmd/login

# ---- run stage ----
# 纯 Go 实现（HTTP 传输层用 uTLS 模拟 Chrome），没有无头浏览器依赖，
# 所以运行镜像可以做得很小。Node 仅在抖音返回 __ac_signature 挑战页时用到
# （douyin/acrawler.go 会依次找 DY_NODE → PATH 上的 node → mise），装上是可选但省心的默认。
FROM alpine:3.20

# 设置时区
ENV TZ=Asia/Shanghai
RUN apk add --no-cache ca-certificates tzdata nodejs && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

WORKDIR /app

# 数据目录：cookie 与运行态都落在这里，通过卷挂载持久化。
RUN mkdir -p /app/data && chmod -R 777 /app/data

COPY --from=builder /out/douyin-mcp ./douyin-mcp
COPY --from=builder /out/douyin-login ./douyin-login

ENV DOUYIN_COOKIES_FILE=/app/data/cookies.txt

EXPOSE 18080

ENTRYPOINT ["/app/douyin-mcp"]
