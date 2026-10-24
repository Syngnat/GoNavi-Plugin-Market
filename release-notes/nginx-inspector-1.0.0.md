# Nginx Inspector 1.0.0 发行说明

- 插件 ID:`com.gonavi.nginx-inspector`
- 版本:1.0.0(semver)
- 许可:Apache-2.0 · 作者:GoNavi
- 源码:`github.com/Syngnat/gonavi-plugins`(官方插件 monorepo,`plugins/nginx-inspector/`)
- 发布 tag:`nginx-inspector-v1.0.0`
- 安装引用:`github:Syngnat/gonavi-plugins@nginx-inspector-v1.0.0`(`@tag` 必须带,否则 latest 语义会取到其他插件的 release)
- `minHostVersion`:1.1.5(正式版满足;CLI 调试需 `-ldflags "-X GoNavi-Wails/internal/cli.Version=1.1.5"`)
- 内容摘要(canonical digest):`7d862c3b000e81a465fb294586083473591de45706bbc63acf2d4b56ecb923b6`

## 发行产物

| 文件 | 大小 | 传输 SHA-256(zip 原始字节) |
|---|---|---|
| `gonavi-plugin-nginx-inspector-1.0.0.zip` | 21,293,930 B(20.3 MB) | `50d92ee65cc0fc5224cd1abc5cf8980080992138b81cd51fbd5cfadf311974a5` |

通用包,单 ZIP 内含 5 个平台二进制(`backend.platforms`):windows/amd64、linux/amd64、linux/arm64、darwin/amd64、darwin/arm64。
下载直链(Release 建立后生效):`https://github.com/Syngnat/gonavi-plugins/releases/download/nginx-inspector-v1.0.0/gonavi-plugin-nginx-inspector-1.0.0.zip`

## 验证记录(2026-10-24,Windows 开发机)

- monorepo 单 module(伪版本 SDK `github.com/Syngnat/GoNavi/sdk/go v0.0.0-20261024091534-21b063ee5732`):`go test ./...`、`go vet ./...` 全绿。
- 宿主静态检查 `gonavi plugin check`:ZIP `valid:true`。
- CLI 闭环:`install --digest --trust` → `enable` → `invoke connections.list` 返回 `{"connections":[]}`。
- 此前的独立仓版本(gonavi-plugin-nginx,digest `49a0699f…`)已由 monorepo 版本取代,未发布过,无迁移影响。
- 真实 SSH 目标(远程 nginx 实例的 config/logs/certs/upstreams/validate/reload)未在本机执行,属上线前人工验收项。

## 发布步骤(仓库维护者)

1. 推送 monorepo 到 `github.com/Syngnat/gonavi-plugins`(main)。
2. 打 tag `nginx-inspector-v1.0.0`,GitHub Release 上传 `gonavi-plugin-nginx-inspector-1.0.0.zip`,把传输 SHA-256 填入 Release 说明。
3. 市场仓 `public/catalog.json` 增补条目(目录当前清空待首批上架):真实 SHA-256、content digest、`installSpec: github:Syngnat/gonavi-plugins@nginx-inspector-v1.0.0`。
4. 市场服务上线后按主仓库 `deploy/plugin-market/admin-api.md` 提交该版本送审。
