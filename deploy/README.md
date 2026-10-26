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
- 公网验收：`https://plugin.gonavi.org/api/v1/plugins` 应 200；站点默认读取同源 `/api/v1/plugins`，服务不可用时才回退 `catalog.json`。

### 登录/发布（v2，需要 GitHub OAuth App）

公共读无需登录；网站登录、发布、评分写入使用 HttpOnly 会话 Cookie + `X-GoNavi-CSRF`。GitHub OAuth 回调与设备码流程共用已注册的 `https://plugin.gonavi.org/api/v1/auth/github/callback`；网站发起入口是 `/api/v1/auth/web/start`。

1. GitHub → Settings → Developer settings → OAuth Apps：Homepage `https://plugin.gonavi.org`，Callback 为上面的 `/api/v1/auth/github/callback`，拿到 Client ID/Secret。
2. 填入服务器 `/etc/gonavi-market/env` 的 `GONAVI_MARKET_GITHUB_CLIENT_ID` / `_SECRET`，`systemctl restart gonavi-market`。
3. 首次网站登录自动分配 `gh-<github-login>` 发布者命名空间；插件 ID 必须以该命名空间加英文句点开头。
4. 发布：网站 `/publish` 上传 ZIP/TGZ → `/api/v1/publisher/inspect` 静态预检 → 确认摘要 → `POST /api/v1/submissions`；校验通过即上架（官方徽标由运营单独授予，违规可 yank）。
5. 作者工作台 `/account` 读取 `/api/v1/publisher/plugins`、`/submissions`，可在 `/api/v1/publisher/plugins/{id}/source` 登记或清空 GitHub Release 自动同步来源。完整契约见 `docs/v2-api.md`。
6. 管理员仍可用 `/api/v1/admin/submissions` 审核/下架、`/api/v1/admin/audit` 查审计；运营授权用 `gonavi-market-server -grant-github-id ... -grant-role admin|publisher`。

### 官方插件上架

服务端目录是主路径：`gonavi-market-server -publish-official <zip>` 走同一条静态校验/发布流水线（官方徽标由运营授予）。`public/catalog.json` 作为服务不可用时的静态兜底，仍可用 `python deploy/update-catalog.py` 更新。

