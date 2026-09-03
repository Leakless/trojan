# syntax=docker/dockerfile:1

############################
# 1) 前端构建 (Vue3 + Vite)
############################
FROM node:22-bookworm-slim AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

############################
# 2) 后端构建 (Go, 内嵌前端)
############################
FROM golang:1.26-bookworm AS backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# 把前端产物注入到 go:embed 目录
COPY --from=frontend /app/frontend/dist ./web/templates
ARG VERSION=docker
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags "-w -s -X 'trojan/trojan.MVersion=${VERSION}'" \
      -o /trojan .

############################
# 3) 运行镜像
############################
FROM debian:12-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl socat iptables cron openssl unzip python3 bash tzdata \
    && rm -rf /var/lib/apt/lists/*
# docker-systemctl-replacement: 让面板在容器内也能用 systemctl 管理 trojan-go
# 下载失败不阻断构建(本地仅测试面板+数据库时用不到); 真实部署时它提供 trojan-go 管理能力
RUN curl -fsSL "https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl3.py" \
      -o /usr/bin/systemctl && chmod +x /usr/bin/systemctl || echo "warn: systemctl replacement 未安装(可稍后手动放置)"
COPY --from=backend /trojan /usr/local/bin/trojan
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh \
    && echo "source <(trojan completion bash)" >> /root/.bashrc
EXPOSE 80 443
ENTRYPOINT ["/entrypoint.sh"]
