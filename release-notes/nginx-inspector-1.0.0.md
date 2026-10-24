# Nginx Inspector 1.0.0 发行说明

- 插件 ID:`com.gonavi.nginx-inspector`
- 版本:1.0.0(semver)
- 许可:Apache-2.0 · 作者:GoNavi
- 源码:本仓库(`github.com/Syngnat/GoNavi-Plugin-Market`,`plugins/nginx-inspector/`)——官方插件与市场站点同仓
- 发布 tag:`nginx-inspector-v1.0.0`
- 安装引用:`github:Syngnat/GoNavi-Plugin-Market@nginx-inspector-v1.0.0`(`@tag` 必须带,否则 latest 语义会取到其他 release)
- `minHostVersion`:1.1.5(正式版满足;CLI 调试需 `-ldflags "-X GoNavi-Wails/internal/cli.Version=1.1.5"`)
- 内容摘要(canonical digest):`b4ac31560be017f2ce6812eefaa8d22944e36d1a5b8297e7d8ab1ff399fd9dac`

## 发行产物

| 文件 | 大小 | 传输 SHA-256(zip 原始字节) |
|---|---|---|
| `gonavi-plugin-nginx-inspector-1.0.0.zip` | 21,295,677 B(20.3 MB) | `dbb945c7e61e102532955fce061393730379b98f1036795eed4e88fa9963cfb8` |

通用包,单 ZIP 内含 5 个平台二进制(`backend.platforms`):windows/amd64、linux/amd64、linux/arm64、darwin/amd64、darwin/arm64。
下载直链(Release 建立后生效):`https://github.com/Syngnat/GoNavi-Plugin-Market/releases/download/nginx-inspector-v1.0.0/gonavi-plugin-nginx-inspector-1.0.0.zip`

## 验证记录(2026-10-24,Windows 开发机)

- 市场仓单 module(伪版本 SDK `github.com/Syngnat/GoNavi/sdk/go v0.0.0-20261024091534-21b063ee5732`):`go build ./...`、`go vet ./...`、`go test ./plugins/...` 全绿;站点 `npm run build` 不受影响。
- 宿主静态检查 `gonavi plugin check`:ZIP `valid:true`。
- CLI 闭环:`install --digest --trust` → `enable` → `invoke connections.list` 返回 `{"connections":[]}`。
- 历史版本 digest:独立仓版 `49a0699f…` → gonavi-plugins monorepo 版 `7d862c3b…` → 本仓版 `b4ac3156…`(module 路径嵌入二进制,每次换仓 digest 必变;本版为最终发布版)。
- 真实 SSH 目标(远程 nginx 实例的 config/logs/certs/upstreams/validate/reload)未在本机执行,属上线前人工验收项。

## 发布步骤(仓库维护者)

1. 打 tag `nginx-inspector-v1.0.0`,GitHub Release 上传 `plugin-dist/gonavi-plugin-nginx-inspector-1.0.0.zip`,把传输 SHA-256 填入 Release 说明。
2. `public/catalog.json` 增补条目:真实 SHA-256、content digest、`installSpec: github:Syngnat/GoNavi-Plugin-Market@nginx-inspector-v1.0.0`。
3. 市场服务上线后按主仓库 `deploy/plugin-market/admin-api.md` 提交该版本送审。
