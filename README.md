# GoNavi Plugin Market

GoNavi 插件市场官网（plugin.gonavi.org）：浏览与安装 GoNavi 插件；**同时是 GoNavi 官方插件的源码仓**——站点、上架目录与官方插件一个仓库。

- 设计定稿：暖纸赤铜（paper `#f6f1ea` / copper `#c4622d` / card `#fffcf8` / ink `#1c1612`），IBM Plex Sans。
- 页面：首页（目录与分类）、插件详情（功能 / 权限清单 / 评分 / SHA-256 核对）、GitHub 登录、作者工作台（`/publish`）、我的插件（`/account`）、开发者与社区。

第三方作者入口：

- [站内文档中心](https://plugin.gonavi.org/docs)：作者指南、FAQ、SDK、协议、权限与各类贡献专项契约。
- [插件作者完整指南](https://plugin.gonavi.org/docs/author-guide)：从插件形态、SDK、打包检查、GitHub Release 到上架与更新。
- [插件作者 FAQ](https://plugin.gonavi.org/docs/faq)：常见概念、错误与发布问题。

插件文档的权威来源已迁到本仓 `docs/`，由站点构建为独立 Markdown 资源并在 `/docs` 站内渲染；GoNavi 主仓只保留内部实现契约和运行时代码。

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

未配置 `VITE_API_BASE` 时默认请求同源 `/api/v1`；市场服务不可用时回退到 `public/catalog.json`（静态目录）。账号与发布接口需要市场服务的 HttpOnly 会话 Cookie，因此生产环境应同源部署。

## API 约定（v2 账号与发布）

目录/详情对客户端保持只读；网站登录与发布走 v2 接口，完整契约见 [`docs/v2-api.md`](docs/v2-api.md)。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/plugins?category=&q=&sort=updated\|rating\|downloads&cursor=` | 目录分页，返回 `CatalogPage` |
| GET | `/api/v1/plugins/{id}` | 插件详情，返回 `Item`（含 artifact.sha256） |
| GET | `/api/v1/auth/web/session` | 网站登录态；返回 `authenticated/account/csrfToken` |
| GET | `/api/v1/auth/web/start?returnTo=/publish` | 发起 GitHub 登录，303 到 GitHub |
| POST | `/api/v1/auth/web/logout` | 退出（需 `X-GoNavi-CSRF`） |
| POST | `/api/v1/publisher/release/inspect` | GitHub Release 发布预检（JSON `sourceUrl`），不落库 |
| POST | `/api/v1/publisher/release/publish` | 从同一 Release 发布（CSRF），摘要变化拒绝 |
| POST | `/api/v1/publisher/inspect` | 上传包兜底预检（multipart `package`），不落库 |
| POST | `/api/v1/submissions` | 发布（multipart `package/pluginId/version/expectedDigest/source`） |
| GET | `/api/v1/publisher/plugins` / `submissions` | 作者工作台列表 |
| PUT | `/api/v1/publisher/plugins/{id}/source` | 设置/清空 GitHub Release 自动更新来源 |

类型镜像见 `src/api/types.ts`；网站会话与发布者接口由主仓库 `internal/market` 提供。

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
