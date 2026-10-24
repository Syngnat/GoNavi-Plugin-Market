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
- `/etc/nginx/sites-available/plugin.gonavi.org.conf` — 站点配置
- `/etc/nginx/stream-conf.d/gonavi-sni.conf` — 443 SNI 分流 map
- `/var/log/nginx/gonavi-plugin-market-{access,error}.log`
