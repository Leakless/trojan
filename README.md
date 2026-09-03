# trojan 管理平台 · 安全加固版

> 基于 [Jrohy/trojan](https://github.com/Jrohy/trojan) + [Jrohy/trojan-web](https://github.com/Jrohy/trojan-web) 的**安全修复 / 依赖升级 / 前后端单仓库 / Docker 一键部署**版本。
>
> 上游项目已停更且存在多处严重漏洞（未授权接管、SQL 注入、命令注入等）。本仓库在保持 trojan-go 数据面不变的前提下，把这些洞逐一堵死，并把前端源码合入同一仓库、给出容器化部署方案。

---

## ✨ 相比上游做了什么

**安全修复**
- 🔴 **未授权接管** — `/auth/register` 曾无鉴权即可重置 admin 口令；现加首装守卫，已存在管理员一律 `403`。
- 🔴 **SQL 注入** — 全量参数化（含登录框这一未授权注入点），彻底消除字符串拼接 SQL。
- 🟠 **弱随机密钥** — JWT 密钥与随机串改用 `crypto/rand`（原为 `math/rand`）。
- 🟠 **明文口令** — 管理口令改 `bcrypt` 存储 + 常量时间比较，兼容旧存量并在登录时自动迁移。
- 🟠 **命令注入** — 证书申请的域名字段加白名单校验，杜绝 `-d <域名>` 注入 `bash`。
- 🟡 **CSRF / 限速** — Cookie `HttpOnly`+`SameSite=Strict`+（HTTPS 下）`Secure`；登录失败限速。
- 🟡 **导入毁库** — CSV 导入去掉 `DROP TABLE`，改事务内替换 + 列数校验。
- 🐛 **证书签发 bug** — 修 `GetLocalIP` 结尾换行导致的“域名/本机 IP 不一致”死循环；acme.sh 加 `--reloadcmd` 使续期后自动重载。

**依赖升级**
- 后端：gin、golang-jwt、gorilla/websocket 等升级，`toolchain go1.26.6`，`govulncheck` **0 漏洞**。
- 前端：vite 8 / vue 3.5 / vue-router 5 / vue-i18n 11 / element-plus 2.14 / axios 1.20 等全量升级；**改为本地打包**（去掉 CDN 外链，自包含），`npm audit` **0 漏洞**。

**工程化**
- 前端源码（`frontend/`）与后端源码（`backend/`）合入同一仓库。
- 多阶段 `Dockerfile` + `docker-compose.yml`，一条命令拉起面板 + MySQL。

---

## 📁 目录结构

```
.
├── backend/              # Go 后端(管理面板 + CLI), go module: trojan
│   ├── cmd/ core/ trojan/ util/ web/ asset/
│   ├── main.go go.mod go.sum
│   └── web/templates/    # 构建时由前端产物注入(.gitignore + go:embed)
├── frontend/             # Vue3 + Vite 前端源码
├── docker/entrypoint.sh  # 容器启动:渲染 config.json → 等待 DB → 起面板
├── Dockerfile            # 多阶段:前端构建 → 后端内嵌 → 运行镜像
├── docker-compose.yml    # 面板 + MySQL 8.4
└── .env.example
```

---

## 🚀 快速开始（Docker，推荐）

```bash
git clone https://github.com/Leakless/trojan.git
cd trojan
cp .env.example .env        # 按需修改数据库密码等
docker compose up -d --build
```

- 面板默认绑定 `127.0.0.1:8080`（见 `.env` 的 `WEB_BIND`）。
- 首次访问会引导创建管理员账号。
- 数据持久化于命名卷：`mysql-data`（数据库）、`trojan-config`（trojan 配置）、`trojan-manager`（面板 leveldb，含管理员口令与密钥）。

> ⚠️ **公网部署务必前置 HTTPS 反代、加访问控制，切勿把面板裸暴露在 `0.0.0.0`。**

真实服务器上申请证书、安装/管理 trojan-go 走面板内既有流程（容器已内置 `systemctl` 兼容层）。

---

## 🛠 本地开发

**前端**（热更新，代理到后端）：
```bash
cd frontend
npm install
npm run dev            # http://127.0.0.1:5173
```

**后端**（需先把前端产物放进 `backend/web/templates`）：
```bash
cd frontend && npm run build && cp -r dist ../backend/web/templates
cd ../backend && go run . web --host 0.0.0.0 --port 8080
```

---

## ⚙️ 环境变量

| 变量 | 说明 | 默认 |
|---|---|---|
| `MYSQL_ROOT_PASSWORD` | MySQL root 口令 | `changeme_root` |
| `MYSQL_DATABASE` / `MYSQL_USER` / `MYSQL_PASSWORD` | 业务库/账号/口令 | `trojan` / `trojan` / `changeme_trojan` |
| `WEB_BIND` | 面板绑定地址 | `127.0.0.1:8080` |
| `TROJAN_BIND` | trojan-go 代理端口 | `443` |

---

## 🔄 从旧部署迁移

只需迁移**用户数据库**（`mysqldump trojan`），导入新栈的 MySQL 即可；面板管理员、JWT 密钥、TLS 证书建议在新机全新初始化 / 重新签发（旧机若被入侵，私钥与密钥应视为泄露）。

---

## 📄 许可

沿用上游 **GPL-3.0**。感谢原作者 [Jrohy](https://github.com/Jrohy)。
