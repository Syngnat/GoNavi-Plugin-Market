# Bero 部署（plugin.gonavi.org）

源站为 Bero-6H16-DE（94.103.173.47，SSH 别名见本机 ~/.ssh/config，经 CSTServer 跳板）。
DNS 为 Cloudflare DNS-only（灰云直连），公网入口 `https://plugin.gonavi.org`（:443）。

## 流量链路

```
:443  nginx stream ssl_preread（stream-conf.d/gonavi-sni.conf 的 SNI map）
        ├─ plugin.gonavi.org → 127.0.0.1:2053（本站，nginx http 终止 TLS）
        ├─ gonavi.org / cli / docker / komari / tv → 127.0.0.1:2053
        ├─ ssh.syngnat.top → 127.0.0.1:37167
        └─ 其余 SNI      → 127.0.0.1:1443（sing-box AnyTLS）
:80   ACME HTTP-01（/var/www/letsencrypt webroot）+ 301 → https
```

## 发布步骤

1. 本地构建并打包（记录 tag 与 SHA-256）：

   ```bash
   npm run build
   tar -czf plugin-market-<sha>-<ts>.tar.gz -C dist .
   sha256sum plugin-market-<sha>-<ts>.tar.gz
   ```

2. 上传到 `/root/plugin-market-staging.tar.gz`，远端 `sha256sum -c` 校验；
   解压到 `/srv/gonavi-plugin-market/releases/<tag>/`（远端命令一律用**字面绝对路径**，
   不要传 `\$VAR` —— 转义会穿透两层 shell 变成字面量）；
   `ln -sfn <releases>/<tag> current.tmp && mv -Tf current.tmp current` 原子切换；
   删除 staging 包。旧的 `current` 指向错的目标时先修正再继续。

3. `deploy/plugin.gonavi.org.conf` scp 到
   `/etc/nginx/sites-available/plugin.gonavi.org.conf`（sites-enabled 有软链）。
   首次部署分两步：先只上 :80 的 ACME 块 → `deploy/plugin.gonavi.org.acme.conf`，
   reload 后 `certbot certonly --webroot -w /var/www/letsencrypt -d plugin.gonavi.org
   --cert-name plugin.gonavi.org --non-interactive --agree-tos`，再上完整配置。

4. 新域名需要在 `/etc/nginx/stream-conf.d/gonavi-sni.conf` 的 map 里加一行
   `plugin.gonavi.org  127.0.0.1:2053;`（`deploy/insert-sni-map.py` 幂等插入，改动前备份）。

5. `nginx -t && systemctl reload nginx`，然后跑 `deploy/verify-origin.sh`（scp 到
   /root 执行）核对：入口 200、assets immutable、index no-cache、SPA 回退、
   POST 403。最后从外网访问 `https://plugin.gonavi.org/` 复核。

## 证书

Let's Encrypt ECDSA，cert-name `plugin.gonavi.org`，webroot 续期（certbot 自动），
当前有效期至 2027-01-22。

## 服务器布局

- `/srv/gonavi-plugin-market/releases/<tag>/` — 每次发布一个目录
- `/srv/gonavi-plugin-market/current` — 软链，nginx root
- `/etc/nginx/sites-available/plugin.gonavi.org.conf` — 站点配置（含 `/api/v1/` 反代 → 127.0.0.1:8090）
- `/etc/nginx/stream-conf.d/gonavi-sni.conf` — 443 SNI 分流 map
- `/var/log/nginx/gonavi-plugin-market-{access,error}.log`

## 市场服务（/api/v1，2026-10-24 上线）

主仓库 `internal/market` + `cmd/gonavi-market-server` 的生产实例，systemd 单元 `gonavi-market`：

- PostgreSQL 16 跑在 docker 容器 `gonavi-market-pg`（仅 `127.0.0.1:25433`，数据卷 `gonavi-market-pgdata`）；库 `gonavi_market`，迁移/引导用 operator 角色，服务运行用最小权限 `gonavi_market_app` 角色。
- 服务监听 `127.0.0.1:8090`，经站点 nginx 的 `/api/v1/` 反代对外（65 MiB 请求上限、120s 上游超时）。
- 工件目录 `/srv/gonavi-plugin-market/artifacts`（700，gonavi-market 用户私有）；日志 `/var/log/gonavi-market`。
- 环境/凭据：`/etc/gonavi-market/env`（600）与 `/root/gonavi-market-credentials.txt`（600，含 operator 连接串）。**不要把这两个文件的内容提交进仓库。**
- 开通脚本的留档：`deploy/provision-market.sh`（首run专用；服务二进制由主仓 `go build ./cmd/gonavi-market-server` 交叉编译上传 `/srv/gonavi-plugin-market/server/`）。
- 常用运维：`systemctl status|restart gonavi-market`、`journalctl -u gonavi-market -e`、
  `curl -s http://127.0.0.1:8090/api/v1/plugins`（服务器上自检）。
- 公网验收：`https://plugin.gonavi.org/api/v1/plugins` 应 200 返回 `{"items":[]}`（未上架服务端条目前为空是正常态，站点目录仍走静态 catalog.json）。

### 登录/提交（待启用，需要 GitHub OAuth App）

公共读已可用；登录（提交/审核/评分）在 OAuth 配置前返回 503 `auth_unavailable`。启用步骤：

1. GitHub → Settings → Developer settings → OAuth Apps → New OAuth App：
   Homepage `https://plugin.gonavi.org`，Callback
   `https://plugin.gonavi.org/api/v1/auth/github/callback`，拿到 Client ID/Secret。
2. 填入服务器 `/etc/gonavi-market/env` 的两行注释处，`systemctl restart gonavi-market`。
3. 授予管理员（Syngnat 数字 ID 92659908）：
   `ssh Bero-6H16-DE "cd /srv/gonavi-plugin-market/server && GONAVI_MARKET_DATABASE_URL=\"<operator串>\" ./gonavi-market-server -grant-github-id 92659908 -grant-role admin"`
   （operator 连接串在 `/root/gonavi-market-credentials.txt`。）
4. 设备码换 token（浏览器登录一次）：`POST /api/v1/auth/device` → 打开 verificationUri 输入 userCode → GitHub 授权 → `POST /auth/device/token`。
5. 之后第三方发布走 `POST /api/v1/submissions`（multipart：pluginId/version/package），管理员在 `/api/v1/admin/submissions` 审核，详见主仓库 `deploy/plugin-market/admin-api.md`。服务端目录有数据后，站点可切 `VITE_API_BASE=https://plugin.gonavi.org` 重新构建发布。

### 官方插件上架（当前路径，静态目录）

站点运行时 `fetch('catalog.json')`，上架官方插件＝更新服务器一个文件：

1. 仓里改 `public/catalog.json`（真实 SHA-256/摘要，Release 先建好直链可下载）并提交推送；
2. `python deploy/update-catalog.py`（校验→scp→原子替换→线上复核）。

