# 使用官方Go镜像作为构建环境
FROM golang:1.21 AS builder
# 设置七牛云代理
ENV GOPROXY=https://goproxy.cn,direct
# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件并下载依赖信息
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .
# 编译Go程序
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

# 使用alpine作为基础镜像
FROM alpine:latest
# 从构建者镜像中复制编译后的程序
COPY --from=builder /app/main .
COPY --from=builder /app/config/ /config/
# 运行程序
CMD ["./main"]
