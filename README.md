# GoNavi Plugin Market

GoNavi 插件市场官网（plugin.gonavi.org）：浏览与安装 GoNavi 插件；**同时是 GoNavi 官方插件的源码仓**——站点、上架目录与官方插件一个仓库。

- 设计定稿：暖纸赤铜（paper `#f6f1ea` / copper `#c4622d` / card `#fffcf8` / ink `#1c1612`），IBM Plex Sans。
- 页面：首页（双列精选 + 目录列表）、插件详情（功能 / 权限清单 / 评价 / SHA-256 核对）、设备码登录。

## 技术栈

- 站点：Vite + React 18 + TypeScript + Tailwind CSS（无 UI 组件库，静态构建，任意 CDN/边缘托管可部署）。
- 官方插件：Go（单 module，SDK 伪版本引用 `github.com/Syngnat/GoNavi/sdk/go`），`cmd/buildtool` 多平台打包。

## 官方插件开发（本仓约定）

**官方插件（GoNavi 作者拆宿主能力做的插件）一律在本仓开发发布，不新建仓库**；第三方作者一仓一插件或一仓多插件均可。

- 源码:`plugins/<name>/`(manifest + 后端 + `ui/` + `skills/`),模板参考 `plugins/nginx-inspector/`。
- 构建:仓库根 `go run ./cmd/buildtool -manifest plugins/<name>/gonavi.plugin.json -output plugin-dist/<name> -zip plugin-dist/<name>-<ver>.zip`(产物进 `plugin-dist/`,与站点 `dist/` 互不混淆;5 平台二进制打进单 ZIP)。
- 发布:tag `<插件名>-v<semver>`(如 `nginx-inspector-v1.0.0`),一 Release 一 ZIP;安装引用 `github:Syngnat/GoNavi-Plugin-Market@<tag>`(**必须带 @tag**,无 tag 解析 latest 会拿错 release)。
- 上架:`public/catalog.json` 增补条目,前提是 Release 已建、直链可下载、SHA-256 真实。
- 验证:`go test ./plugins/...`;宿主侧静态检查与 CLI 闭环见 `release-notes/`。

## 开发

```bash
npm install
npm run dev       # http://localhost:5173
npm run build     # tsc --noEmit && vite build → dist/
npm run preview
```

未配置 `VITE_API_BASE` 时运行在**演示模式**：目录数据来自 `src/data/seed.ts`，不发网络请求。

## API 约定（与 GoNavi 主仓库 internal/pluginmarket 契约对齐）

配置 `VITE_API_BASE`（如 `https://plugin.gonavi.org`）后，站点请求：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/plugins?category=&q=&sort=recent\|rating\|downloads&cursor=` | 目录分页，返回 `CatalogPage` |
| GET | `/api/v1/plugins/{id}` | 插件详情，返回 `Item`（含 artifact.sha256） |
| POST | `/api/v1/auth/device/verify` | 设备码登录确认，body `{ userCode }`，返回 `LoginStatus` |

类型镜像见 `src/api/types.ts`，字段与主仓库 `internal/pluginmarket/types.go` 的 JSON tag 一致。

## 安装深链契约

「安装」按钮唤起本机 GoNavi 客户端（客户端在 `internal/deeplink` 注册并解析）：

```
gonavi://plugin/install?spec=<github:owner/repo 或 npm 包名>&id=<插件ID>&version=<版本>
```

- `spec` 可选：客户端预填安装面板并自动执行检查；`id` 必填：市场服务接入前客户端仅预填输入框。
- `spec` 的 github 形态支持可选 tag：`github:owner/repo@tag`（tag 允许 `._+-/`，官方 monorepo 约定 `<插件名>-v<semver>`，如 `github:Syngnat/gonavi-plugins@nginx-inspector-v1.0.0`）。不带 tag 时解析仓库 latest release——**monorepo 必须带 tag**，否则 latest 是最后发版的插件；粘贴 `github.com/.../releases/download/...` 直链亦可，宿主跳过 release 解析直接下载。
- 链接只负责把用户送到客户端的安装面板；下载、静态检查、SHA-256 摘要确认全部在客户端完成。
- 2 秒内浏览器未弹出协议处理框（未装客户端）时，按钮回退为复制插件 ID。
- 市场服务上线后，目录条目携带真实来源时在 `buildInstallDeepLink` 调用处传入 `spec` 即可。

## 部署

`npm run build` 产出纯静态 `dist/`。SPA 路由需托管层将所有路径回退到 `index.html`（如 Nginx `try_files` / Cloudflare Pages 自动处理）。
