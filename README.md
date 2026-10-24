# GoNavi Plugin Market

GoNavi 插件市场官网（plugin.gonavi.org）：浏览与安装 GoNavi 插件。

- 设计定稿：暖纸赤铜（paper `#f6f1ea` / copper `#c4622d` / card `#fffcf8` / ink `#1c1612`），IBM Plex Sans。
- 页面：首页（双列精选 + 目录列表）、插件详情（功能 / 权限清单 / 评价 / SHA-256 核对）、设备码登录。

## 技术栈

Vite + React 18 + TypeScript + Tailwind CSS（无 UI 组件库，静态构建，任意 CDN/边缘托管可部署）。

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

## 部署

`npm run build` 产出纯静态 `dist/`。SPA 路由需托管层将所有路径回退到 `index.html`（如 Nginx `try_files` / Cloudflare Pages 自动处理）。
