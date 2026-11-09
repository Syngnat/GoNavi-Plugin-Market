# Nginx Inspector 1.0.3

1.0.3 修复独立窗口在真实宿主里只显示 HTML 骨架、样式与脚本不加载的问题。1.0.2 的核心诊断修复保持不变。

## 修复

- 宿主把窗口入口文档直接挂在 `/`，子窗口代理只放行 `/`、`/plugin-assets/` 与
  `/__gonavi/plugin-window/` 三类路径；插件页面此前使用相对 `styles.css` / `app.js`，
  会解析成 `/styles.css`、`/app.js` 并 404，窗口只剩「跳到主内容」、连接下拉和测试按钮。
- 入口改为 `/plugin-assets/ui/styles.css` 与 `/plugin-assets/ui/app.js`：宿主把该前缀映射到
  包内路径，因此必须保留 `ui/` 前缀且使用绝对路径。
- 移除页面自带 CSP。宿主已通过响应头注入完整策略，页面的 `meta` CSP 会挡掉宿主注入的
  内联 bootstrap 与偏好（主题/语言）脚本。
- 新增回归测试：窗口入口的每个 `href`/`src` 必须走宿主资产前缀、对应文件真实存在，
  且页面不得再声明 CSP。

## 验证

- 本地按宿主路径语义复现（`/` 返回入口 HTML，仅 `/plugin-assets/*` 提供静态文件），
  Playwright 打开后左栏、九个分区、连接管理与空态完整渲染。
- `go build`、`go vet`、`go test ./plugins/nginx-inspector/...` 全绿。

## 安装要求

- 宿主 **GoNavi ≥ 1.2.1**（与 1.0.2 相同）。
- 已安装 1.0.2 的用户在插件市场更新到 1.0.3 后，关闭并重新打开独立窗口即可。
