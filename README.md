# trojan 管理平台（安全修复 + Docker 化版）

基于开源项目 [Jrohy/trojan](https://github.com/Jrohy/trojan) 的**安全加固 + 规范化 + Docker 一键部署**版本。
前端源码与后端源码分仓于同一个 monorepo，数据面仍为 trojan-go + MySQL。

## 目录结构

```
.
├── backend/            # Go 后端（管理面板 + CLI），module: trojan
│   ├── cmd/  core/  trojan/  util/  web/  asset/
│   ├── main.go  go.mod  go.sum
│   └── web/templates/  # 构建时由前端产物注入（.gitignore, go:embed）
├── frontend/           # Vue3 + Vite 前端源码
├── docker/
│   └── entrypoint.sh   # 容器启动脚本（渲染 config.json、等待 DB、起面板）
├── Dockerfile          # 多阶段：前端构建 → 后端内嵌构建 → 运行镜像
├── docker-compose.yml  # app + MySQL 8.4
└── .env.example        # 环境变量样例
```

## 安全修复要点（相对上游）

- **修复未授权接管**：`/auth/register` 加首装守卫，已存在管理员即拒绝（堵住任意重置 admin 口令的接管洞）。
- **全量参数化 SQL**：消除登录框等处的 SQL 注入。
- **强随机 + 口令哈希**：JWT 密钥改 `crypto/rand`；管理口令 bcrypt 存储 + 常量时间比较（兼容旧存量并自动迁移）。
- **CSRF/限速**：Cookie `HttpOnly`+`SameSite=Strict`+（SSL 下）`Secure`；登录失败限速。
- **导入不再毁库**：CSV 导入去掉 `DROP TABLE`，改事务内替换。
- **依赖升级**：后端 gin/golang-jwt/gorilla-websocket 等 + Go toolchain；前端全量升级并**本地打包**（去掉 CDN 外链，`npm audit` 0 漏洞）。

> 详细清单见提交历史与 `backend/` 各文件改动。

## Docker 一键部署

```bash
cp .env.example .env      # 按需修改数据库密码等
docker compose up -d --build
```

- 管理面板默认绑定 `127.0.0.1:8080`（`.env` 的 `WEB_BIND`）。**公网部署务必前置 HTTPS 反代，勿裸暴露。**
- 首次访问会提示创建管理员账号。
- 数据持久化于命名卷：`mysql-data`（数据库）、`trojan-config`（trojan 配置）、`trojan-manager`（面板 leveldb：管理员口令/密钥等）。

真实服务器上申请证书、安装/管理 trojan-go 走面板内的既有流程（容器内置 `systemctl` 兼容层）。

## 本地开发

前端：
```bash
cd frontend && npm install && npm run dev   # http://127.0.0.1:5173, 代理到后端
```

后端（需先放置前端产物到 `backend/web/templates`）：
```bash
cd frontend && npm run build && cp -r dist ../backend/web/templates
cd ../backend && go run . web --host 0.0.0.0 --port 8080
```

## 致谢

上游项目：[Jrohy/trojan](https://github.com/Jrohy/trojan) / [Jrohy/trojan-web](https://github.com/Jrohy/trojan-web)（GPL-3.0）。
