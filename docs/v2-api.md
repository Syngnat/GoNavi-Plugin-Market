# 插件市场 v2：账号与发布闭环（源码重建契约）

本文件是本次「把线上 v2 的登录→发布闭环补回仓库」的共享契约。线上 v2 源码已丢失，本次按线上行为重建，但允许实现细节差异；以本文件为准。

## 边界

- 市场仓（`GoNavi-Plugin-Market`）：新增 `/login`、`/publish`、`/account`、`/account/plugins`、`/account/submissions`、`/account/plugins/:id`，以及 `TopNav` 登录态。**不重做现有首页/详情页的视觉**，保持现有 paper/copper 风格。
- GoNavi 仓（`internal/market` + `cmd/gonavi-market-server`）：新增网站会话认证与发布者接口。公开目录/详情/版本/下载/评分接口保持不变。
- 桌面客户端 `internal/pluginmarket` 不改；发布只在网站完成。

## 认证

- 浏览器登录：`GET /api/v1/auth/web/start?returnTo=/publish` → 303 GitHub OAuth；回调 `GET /api/v1/auth/web/callback` 成功后写 HttpOnly 会话 Cookie 并 303 回 `returnTo`。
- 会话 Cookie：`__Host-gonavi_market_session`（`GONAVI_MARKET_PUBLIC_URL` 为 https 时）；本地 http 用 `gonavi_market_session`。`Path=/; HttpOnly; SameSite=Lax; Secure=https; Max-Age=86400`。
- CSRF：`X-GoNavi-CSRF` 请求头。token = base64url(sha256("gonavi-market-csrf:" + sessionToken))，由 `GET /auth/web/session` 返回。Cookie 认证的 POST/PUT/DELETE 必须校验；Bearer 客户端跳过。
- 登录态复用 `market_sessions` 表；新增 `CreateSession(ctx, tokenHash, accountID, expiresAt)`。
- OAuth 复用现有 `IdentityProvider` / `AuthFlow{Kind:"oauth"}`；`returnTo` 放在短时 HttpOnly Cookie，不放进 state。

### 端点

`GET /api/v1/auth/web/session`
- 无有效会话：200 `{"authenticated":false}`
- 有效：200 `{"authenticated":true,"account":{"id":"...","login":"...","role":"publisher","publisherId":"gh-login","official":false},"csrfToken":"..."}`

`GET /api/v1/auth/web/start?returnTo=/publish`
- `returnTo` 必须同源路径：以 `/` 开头、不以 `//` 开头、不含 `\`，否则回退 `/account`。
- 303 到 GitHub；设置 oauth state Cookie 与 returnTo Cookie。

`GET /api/v1/auth/web/callback?code=&state=`
- state 不匹配 → 303 `/login?error=invalid_oauth_state`
- code 空 → 303 `/login?error=oauth_denied`
- 成功 → 303 `returnTo`
- 首次登录由 `UpsertIdentity` 自动分配 `gh-<login>` 发布者命名空间。

`POST /api/v1/auth/web/logout`（CSRF）
- 200 `{"revoked":true}`，清 Cookie。

## 发布者接口（均需登录，Cookie 或 Bearer）

### `POST /api/v1/publisher/inspect`
multipart：`package`（ZIP/TGZ/TAR.GZ，≤64 MiB，根含 `gonavi.plugin.json`）。
- 200：
```json
{
  "canPublish": true,
  "reason": "",
  "pluginId": "gh-login.demo",
  "version": "1.0.0",
  "name": "Demo",
  "description": "...",
  "category": "ai-provider",
  "minHostVersion": "1.1.5",
  "platforms": ["windows","macos","linux"],
  "sha256": "<64 hex>",
  "sizeBytes": 12345,
  "manifest": { "...": "validated manifest" }
}
```
- 命名空间不匹配（`pluginId` 不以 `account.publisherId + "."` 开头）或角色非 publisher/admin：200，`canPublish:false`，`reason:"forbidden"`。
- 包无效：400 `{"code":"artifact_invalid","message":"artifact_invalid"}`。
- 预检不落库、不预约版本。

### `GET /api/v1/publisher/plugins?limit=24&cursor=&pluginId=`
200：
```json
{"items":[{"id":"gh-login.demo","name":"Demo","latestVersion":"1.0.0","latestState":"published","publishedVersion":"1.0.0","sourceUrl":""}],"nextCursor":""}
```
- 只返回当前账号拥有的插件；`pluginId` 过滤用于详情页。
- `latestState` 使用 `draft|submitted|validating|pending_review|published|rejected|yanked`。
- 游标为 opaque 字符串；limit 1..50，默认 24。

### `GET /api/v1/publisher/submissions?limit=24&cursor=&pluginId=`
200：
```json
{"items":[{"id":"...","pluginId":"gh-login.demo","version":"1.0.0","state":"published","reason":"","sha256":"...","createdAt":"...","updatedAt":"..."}],"nextCursor":""}
```

### `POST /api/v1/submissions`
multipart：`package`、`pluginId`、`version`、`expectedDigest`（可选）、`source`（可选）。
- 沿用现有静态校验 + 自助上架。
- `expectedDigest` 非空且与实测 SHA-256 不同 → 409 `inspection_changed`，不落库。
- 201 返回现有 `Submission`（至少含 `id/pluginId/version/state/sha256`）。
- 版本不可覆盖：409 `conflict` / `version_locked` 由现有逻辑给出。

### `PUT /api/v1/publisher/plugins/{id}/source`（CSRF）
body `{"sourceUrl":"https://github.com/owner/repo"}`；空串表示关闭自动同步。
- 200 `{"sourceUrl":"..."}`；非法 URL 400 `invalid_request`；非本人插件 403/404。

## 前端页面

- `/login`：GitHub 登录按钮（`/api/v1/auth/web/start?returnTo=...`）；已登录显示“继续前往目标页面”；错误码映射。
- `/publish`：未登录显示登录引导；登录后显示账号/命名空间；选包或拖入 → 预检 → 展示包身份/版本/分类/大小/平台/SHA-256/权限；勾选确认 → 发布；处理 `unknown` 状态并引导“确认发布结果”；可选 GitHub 源仓库。
- `/account`：账号/命名空间/角色；所属插件列表（分页、状态、公开版本、来源入口）；发布记录列表（分页、原因、SHA-256）。
- `/account/plugins/:id`：插件信息、GitHub 源仓库保存、版本/发布记录。
- `TopNav`：登录/我的插件/退出；保留现有导航与发布按钮。
- 错误码中文映射至少覆盖：`authentication_required/unauthorized/forbidden/invalid_csrf/invalid_request/artifact_invalid/conflict/version_locked/inspection_changed/upload_busy/server_busy/rate_limited/auth_unavailable/not_found/already_published/version_conflict/source_update_failed/invalid_oauth_state/oauth_denied/identity_provider_failed`。

## 实现约束

- Go：不执行插件代码；上传文件用 `os.CreateTemp` 并在 handler 结束删除；复用 `ValidatePackage`、`categoryFromManifest`、`parseRepositoryURL`、`writeError`。
- Go：新增 `CreateSession` 到 `AuthRepository` 与 `Postgres`；`account()` 增加 Cookie 回退与 CSRF 校验；新增 `PublisherPlugin` 类型与仓储方法。
- 前端：复用现有 `src/api/client.ts` 的 `API_BASE`/错误处理思路；新增 `src/api/v2.ts`、`src/auth/AuthContext.tsx`；页面组件放 `src/pages/`；不要引入新依赖。
- 视觉参考：线上 bundle 美化版在 `/tmp/mk_site.pretty.js`（10500–11950 行有登录/发布/账号页的 JSX 与文案）。
- 验证：Go `go test ./internal/market/...`、`go build ./cmd/gonavi-market-server`；前端 `npm run build`。
- 提交：GoNavi worktree 分支 `feat/market-v2-api`；市场仓分支 `feat/v2-publish`。不要 push。
