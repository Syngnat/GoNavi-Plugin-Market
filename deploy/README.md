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

### 迁移与同步的运维前提

- 迁移用**表 owner** 连接执行：`-migrate` 会优先读 `GONAVI_MARKET_MIGRATION_DATABASE_URL`，
  缺省才退回运行角色的 `GONAVI_MARKET_DATABASE_URL`。运行角色 `gonavi_market_app` 不是表
  owner，直接拿它跑 `ALTER TABLE` 会报 `must be owner of table`。
- 官方插件在同一个仓里，`release_tag_prefix` 决定同步看哪条 tag 流（如 `mongo-shell-v`）。
  `publish-official-plugin.sh` 会按 tag 自动推导并写入；没有前缀时同步会跳过该插件而不是
  拿仓库 latest 乱发。

## 上架官方插件版本（多插件仓库必走）

站点的插件列表来自 `/api/v1`（数据库），不是 `public/catalog.json`；`catalog.json` 只描述
可安装来源。后台的 GitHub release 定时同步只认带 `sourceUrl` 的插件，且读的是**仓库 latest
release**——官方插件都挤在 GoNavi-Plugin-Market 一个仓里，latest 属于最后打 tag 的那个插件，
所以官方插件永远不会自动同步，必须逐个版本导入：

```bash
deploy/publish-official-plugin.sh --plugin mongo-shell --tag mongo-shell-v1.0.4 \
    --reviewer <admin-github-id>
```

脚本：下载该 tag 的资产 → 上传 → 以 `-publish-official` 走投稿管线入库 → 回读
`/api/v1/plugins` 校验版本与清单一致。`--dry-run` 只下载与校验。改完 `public/catalog.json`
后仍需发布站点（CI 或 `deploy/publish-site.sh`），两者缺一：目录新了而接口还旧。

`nginx-inspector` 的 `nginx-inspector-v*` tag 流水线已在 GitHub Release 创建后自动执行
这条导入并回读校验；已入库的版本会跳过，手工命令保留给补历史版本、其他插件或流水线失败后重跑。

## 推送即发布（CI）

`.github/workflows/deploy.yml` 在 `main` 上**测试通过后发布整套**：先 `deploy/publish-server.sh`
更新 Go 服务，再 `deploy/publish-site.sh` 更新站点，两者取自同一个提交（也可在 Actions 页面
手动 `Run workflow`）。顺序是有意的：站点读 `/api/v1`，新站点可能依赖新接口；新服务继续服务
旧站点是安全的，反过来不是。服务端安装或健康检查失败会回滚并中止整次发布，站点不会在坏后端
之上被发布。

触发器覆盖服务端路径（`internal/**`、`cmd/**`、`go.mod`、`go.sum`）与站点路径；只改其中一侧
也会整套发布，保证线上两半始终同一提交。纯测试的 pull request 仍由 `.github/workflows/server-tests.yml`
检查，它不再在 push 时重复跑一遍。仓库 secrets：

| Secret | 含义 |
|---|---|
| `DEPLOY_HOST` | 服务器 IP / 域名 |
| `DEPLOY_USER` | SSH 用户（root） |
| `DEPLOY_PORT` | SSH 端口 |
| `DEPLOY_SSH_KEY` | 部署私钥（对应公钥在 `/root/.ssh/authorized_keys`） |
| `DEPLOY_KNOWN_HOSTS` | 主机指纹，避免首次连接被拦 |

本地手工发布仍可直接跑脚本（走 `~/.ssh/config` 的别名，无需上述 secrets）。

## 一键发布（推荐）

```bash
deploy/publish-server.sh                  # 构建 origin/main 的服务端并发布
deploy/publish-site.sh                    # 构建 origin/main 的站点并发布
deploy/publish-server.sh --dry-run        # 只构建二进制，不连服务器
deploy/publish-site.sh --dry-run          # 只构建打包，不连服务器
deploy/publish-site.sh --ref <ref>        # 指定提交/分支
deploy/publish-site.sh --keep 10          # 保留最近 10 个 release（默认 5）
deploy/publish-site.sh --skip-verify      # 跳过源站与线上目录校验
```

服务端脚本行为：临时 worktree 里从**已提交的 ref** 交叉编译 `CGO_ENABLED=0 GOOS=linux
GOARCH=amd64` 静态二进制 → 上传并远端校验 SHA-256 → 备份当前二进制 → 原子替换 →
用 operator 凭据跑 `-migrate`（迁移已内嵌在二进制里）→ `systemctl restart gonavi-market` →
轮询 `127.0.0.1:8090/api/v1/plugins` 至多 30 秒；迁移或健康检查失败则恢复旧二进制并重启，
整次发布失败。备份按 `--keep` 保留，可手工回滚：

```bash
ssh BeroHost-6h16-DE 'ls -1dt /srv/gonavi-plugin-market/server/gonavi-market-server.bak-*'
```

站点脚本行为：在临时 worktree 里从**已提交的 ref** 构建（本地未提交改动不会上线）→
打包 `dist` 并算 SHA-256 → 上传 `/root/plugin-market-staging.tar.gz` 远端校验 →
解包到 `releases/site-<sha>-<ts>/` → 原子切换 `current` → 按 `--keep` 清理旧版本 →
跑 `verify-origin.sh` 与线上 `catalog.json` 比对。远端根目录固定
`/srv/gonavi-plugin-market`，主机默认 `BeroHost-6h16-DE`。

回滚：`ssh BeroHost-6h16-DE 'ln -sfn /srv/gonavi-plugin-market/releases/<旧tag> /srv/gonavi-plugin-market/current.tmp && mv -Tf /srv/gonavi-plugin-market/current.tmp /srv/gonavi-plugin-market/current'`。

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

本仓 `internal/market` + `cmd/gonavi-market-server` 的生产实例（服务文档见 [`server/README.md`](server/README.md)），systemd 单元 `gonavi-market`：

- PostgreSQL 16 跑在 docker 容器 `gonavi-market-pg`（仅 `127.0.0.1:25433`，数据卷 `gonavi-market-pgdata`）；库 `gonavi_market`，迁移/引导用 operator 角色，服务运行用最小权限 `gonavi_market_app` 角色。
- 服务监听 `127.0.0.1:8090`，经站点 nginx 的 `/api/v1/` 反代对外（65 MiB 请求上限、120s 上游超时）。
- 工件目录 `/srv/gonavi-plugin-market/artifacts`（700，gonavi-market 用户私有）；日志 `/var/log/gonavi-market`。
- 环境/凭据：`/etc/gonavi-market/env`（600）与 `/root/gonavi-market-credentials.txt`（600，含 operator 连接串）。**不要把这两个文件的内容提交进仓库。**
- 开通脚本的留档：`deploy/provision-market.sh`（首run专用）。服务二进制由 `deploy/publish-server.sh` 交叉编译并原子替换到 `/srv/gonavi-plugin-market/server/gonavi-market-server`，push 到 `main` 时随站点一起自动发布。
- 常用运维：`systemctl status|restart gonavi-market`、`journalctl -u gonavi-market -e`、
  `curl -s http://127.0.0.1:8090/api/v1/plugins`（服务器上自检）。
- 公网验收：`https://plugin.gonavi.org/api/v1/plugins` 应 200；站点默认读取同源 `/api/v1/plugins`，服务不可用时才回退 `catalog.json`。

### 登录/发布（v2，需要 GitHub OAuth App）

公共读无需登录；网站登录、发布、评分写入使用 HttpOnly 会话 Cookie + `X-GoNavi-CSRF`。网站发起入口是 `/api/v1/auth/web/start`，GitHub OAuth 回调复用已注册的 `https://plugin.gonavi.org/api/v1/auth/github/callback`。

1. GitHub → Settings → Developer settings → OAuth Apps：Homepage `https://plugin.gonavi.org`，Callback 为上面的 `/api/v1/auth/github/callback`，拿到 Client ID/Secret。
2. 填入服务器 `/etc/gonavi-market/env` 的 `GONAVI_MARKET_GITHUB_CLIENT_ID` / `_SECRET`，`systemctl restart gonavi-market`。
3. 首次网站登录自动分配 `gh-<github-login>` 发布者命名空间；插件 ID 必须以该命名空间加英文句点开头。
4. 发布：网站 `/publish` 优先填写 GitHub 仓库/Release 链接 → `/api/v1/publisher/release/inspect` 静态预检 → 确认摘要 → `/api/v1/publisher/release/publish`。单插件仓库可用仓库根链接；官方 `Syngnat/GoNavi-Plugin-Market` monorepo 必须给具体 Release/Tag。上传 ZIP/TGZ 走 `/publisher/inspect` + `/submissions`，仅作兜底。校验通过即上架（官方徽标由运营单独授予，违规可 yank）。
5. 作者工作台 `/account` 读取 `/api/v1/publisher/plugins`、`/submissions`，可在 `/api/v1/publisher/plugins/{id}/source` 登记或清空 GitHub Release 自动同步来源。完整契约见 `docs/v2-api.md`。
6. 管理员仍可用 `/api/v1/admin/submissions` 审核/下架、`/api/v1/admin/audit` 查审计；运营授权用 `gonavi-market-server -grant-github-id ... -grant-role admin|publisher`。

### 官方插件上架

服务端目录是主路径：`gonavi-market-server -publish-official <zip>` 走同一条静态校验/发布流水线（官方徽标由运营授予）。`public/catalog.json` 作为服务不可用时的静态兜底，由 `python3 deploy/update-catalog.py` 推送。

**静态兜底为什么不能只靠 `deploy.yml`**：`plugin-release.yml` 用 `GITHUB_TOKEN` 推送 `chore(market)` 提交，而 GitHub 不会为 `GITHUB_TOKEN` 触发的 push 启动工作流，所以 `deploy.yml` 从不因这次提交重建站点，线上 `current/catalog.json` 会一直停留在上一个版本（1.0.9 发布时线上仍是 1.0.6）。因此 `plugin-release.yml` 的 `release` job 在推完 catalog 提交后直接调用本脚本把文件送到站点；脚本会校验远端摘要、原子替换并回读公网摘要确认。

手动修复或本地演练：

```bash
python3 deploy/update-catalog.py --dry-run          # 只打印目标，不写入
python3 deploy/update-catalog.py                     # 推送并校验 https://plugin.gonavi.org/catalog.json
python3 deploy/update-catalog.py --host user@host --port 2222   # CI 用法
```

## Host checker

Uploads are re-checked with a real `gonavi plugin check` run, so the server needs a
gonavi binary. Install or refresh it with:

```bash
deploy/install-host-checker.sh --version 1.2.0 --host BeroHost-6h16-DE
```

The script verifies the release checksum, installs under `<root>/host-tools/gonavi`
and writes `GONAVI_MARKET_HOST_BINARY` into `/etc/gonavi-market/env` before
restarting the service. `deploy/publish-server.sh` prints the installed checker
version at the end of every release, so an outdated one shows up immediately.
