FROM golang:1.23-alpine AS builder

# Add build arguments for proxy settings
ARG HTTPS_PROXY
ARG HTTP_PROXY
ARG NO_PROXY

# Set environment variables for proxy if provided
ENV HTTPS_PROXY=${HTTPS_PROXY}
ENV HTTP_PROXY=${HTTP_PROXY}
ENV NO_PROXY=${NO_PROXY}

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o docker-mcp ./cmd/docker-mcp

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# 从builder阶段复制编译好的二进制文件
COPY --from=builder /app/docker-mcp .

# 设置时区
ENV TZ=Asia/Shanghai

# 使用非root用户运行
RUN adduser -D -h /home/mcp mcp
USER mcp
WORKDIR /home/mcp

COPY --from=builder /app/docker-mcp /usr/local/bin/

ENTRYPOINT ["docker-mcp"]