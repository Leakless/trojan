#!/usr/bin/env bash
set -e

CONFIG=/usr/local/etc/trojan/config.json
mkdir -p /usr/local/etc/trojan /var/lib/trojan-manager

MYSQL_HOST="${MYSQL_HOST:-mysql}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_DATABASE="${MYSQL_DATABASE:-trojan}"
MYSQL_USER="${MYSQL_USER:-trojan}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-trojan}"
WEB_PORT="${WEB_PORT:-80}"

# 首次启动生成服务端配置(已存在则沿用, 保证持久化)
if [ ! -f "$CONFIG" ]; then
  echo "生成 $CONFIG ..."
  cat > "$CONFIG" <<EOF
{
    "run_type": "server",
    "local_addr": "0.0.0.0",
    "local_port": 443,
    "remote_addr": "127.0.0.1",
    "remote_port": 80,
    "password": [],
    "log_level": 1,
    "ssl": {
        "cert": "",
        "key": "",
        "key_password": "",
        "cipher": "ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384",
        "cipher_tls13": "TLS_AES_128_GCM_SHA256:TLS_CHACHA20_POLY1305_SHA256:TLS_AES_256_GCM_SHA384",
        "prefer_server_cipher": true,
        "alpn": ["http/1.1"],
        "reuse_session": true,
        "session_ticket": false,
        "session_timeout": 600,
        "plain_http_response": "",
        "curves": "",
        "dhparam": ""
    },
    "tcp": {
        "prefer_ipv4": false,
        "no_delay": true,
        "keep_alive": true,
        "reuse_port": false,
        "fast_open": false,
        "fast_open_qlen": 20
    },
    "mysql": {
        "enabled": true,
        "server_addr": "${MYSQL_HOST}",
        "server_port": ${MYSQL_PORT},
        "database": "${MYSQL_DATABASE}",
        "username": "${MYSQL_USER}",
        "password": "${MYSQL_PASSWORD}",
        "cafile": ""
    }
}
EOF
fi

# 等待数据库就绪
echo "等待 MySQL ${MYSQL_HOST}:${MYSQL_PORT} ..."
for i in $(seq 1 60); do
  if (exec 3<>"/dev/tcp/${MYSQL_HOST}/${MYSQL_PORT}") 2>/dev/null; then
    exec 3>&- 3<&-
    echo "MySQL 已就绪"
    break
  fi
  sleep 2
done

echo "启动 trojan 管理面板 (0.0.0.0:${WEB_PORT}) ..."
exec /usr/local/bin/trojan web --host 0.0.0.0 --port "${WEB_PORT}"
