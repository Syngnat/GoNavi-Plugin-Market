# 華龍算力 AI 供应商插件

把 GoNavi 的对话请求以 OpenAI 兼容的流式协议转发到 [華龍算力](https://api.hualong.online) 网关。
插件本身不实现模型，只做 `start` / `pull` / `cancel` 三种操作上的传输与 SSE 解析。

## 能力

- 供应商 id：`hualong`，名称：華龍算力
- 支持工具调用（`supportsTools: true`）
- 不支持图片输入（`supportsImages: false`）
- 12 个静态模型，见 `gonavi.plugin.json` 的 `contributes.aiProviders[0].models`

## 配置

宿主通过 `start.Config` 注入以下字段，插件不做持久化：

| 字段 | 说明 |
|---|---|
| `baseUrl` | 留空时使用默认网关 `https://api.hualong.online/v1`；填写时指向自建代理，仍按 `completionURL` 规则拼接 `/chat/completions` |
| `apiKey` | 以 `Authorization: Bearer <apiKey>` 发送，**只能**来自宿主注入，插件内无任何硬编码密钥 |
| `model` | 必须是上面 12 个模型 id 之一 |
| `headers` | 逐条写入上游请求头 |

`baseUrl` 只接受 `http` / `https`，且拒绝带 userinfo、fragment 或 opaque 形式的地址。

## 协议行为

- 请求体固定 `stream: true`、`stream_options.include_usage: true`、`n: 1`
- `Accept: text/event-stream`，上游必须返回 `200` 且 `Content-Type` 为 `text/event-stream`
- 任何非 200、非 SSE、超限或协议不符的情况统一返回失败码 `generation_failed` / `limit_exceeded` / `invalid_request`，**不回显**上游原文、URL、请求头或密钥
- 上限：响应体 16 MiB、单行 128 KiB、单事件 128 KiB、工具调用 64 个

## 账号面板

插件声明了一个独立窗口（`window.entry = ui/index.html`），提供「一键登录 + 余额 + API Key」。
界面是 `ui/` 下的原生 HTML/CSS/JS，无构建步骤、无第三方库。

| 命令 | 作用 |
|---|---|
| `account.login` | 启动（或复用）独立 Chrome，打开 `https://api.hualong.online/login`，立即返回 `{started:true}`，不等待登录 |
| `account.poll` | 经 CDP 读取面板 `localStorage`；读到 `auth_token` 则落盘并返回 `{state:"ready",user}`，否则 `{state:"pending"}` |
| `account.status` | 返回余额快照（email / balance / frozen / totalRecharged / concurrency / status / expiresAt）；未登录返回 `{state:"signed_out"}` |
| `account.keys` | 返回 Key 列表（id / name / key / status / groupName / rateMultiplier / platform / quota / quotaUsed / lastUsedAt） |
| `account.groups` | 返回可选用的密钥分组（id / name / rateMultiplier / platform） |
| `account.createKey` | 新建 API Key，参数 `{name, groupId?}`，返回新建后的 Key 项 |
| `account.updateKey` | 更新 Key 的名称或分组，参数 `{id, name, groupId?}`；`groupId` 传 0 表示清空分组 |
| `account.deleteKey` | 删除一把 Key，参数 `{id}` |
| `account.listModels` | 用账号下每一把 Key 各查询一次 `GET /v1/models`，聚合去重后返回模型 id（不同分组可见模型不同）；单把失败不影响整体，另返回 `failedKeys` |
| `account.logout` | 清本地账号存储并关闭本插件启动的 Chrome |

运行时细节：

- Chrome 路径可用 `GONAVI_HUALONG_CHROME` 覆盖；否则按平台探测（macOS 应用路径、Windows `Program Files`/`LOCALAPPDATA`、Linux `google-chrome`/`chromium` 等）
- 浏览器资料目录固定在 `$GONAVI_PLUGIN_DATA/chrome-profile`，不接触用户日常 profile
- 调试端口随机分配，记录在 `$GONAVI_PLUGIN_DATA/session.json`；复用时先探测 `/json/version` 是否存活
- 令牌只写 `$GONAVI_PLUGIN_DATA/account.json`（权限 `0600`），任何日志、错误消息与返回值都不含令牌原文
- 新建 Key：`POST /api/v1/keys`，请求体 `{name, group_id?}`；改名/改分组：`PUT /api/v1/keys/{id}`；删除：`DELETE /api/v1/keys/{id}`；分组：`GET /api/v1/groups/available`
- 模型扫描：`GET /v1/models` 以一把 Key 作 Bearer，按分组返回该 Key 可见的模型
- 访问令牌按 `token_expires_at` 判断，进入 90 秒余量后先用刷新令牌续期：`POST /api/v1/auth/refresh`，请求体 `{"refresh_token":"…"}`，响应 `data` 含 `access_token` / `refresh_token` / `expires_in`

## 构建与打包

```bash
go build ./plugins/hualong-ai-provider/
go vet ./plugins/hualong-ai-provider/
go test ./plugins/hualong-ai-provider/

go run ./cmd/buildtool \
  -manifest plugins/hualong-ai-provider/gonavi.plugin.json \
  -output plugin-dist/hualong-ai-provider \
  -zip plugin-dist/gonavi-plugin-hualong-ai-1.1.0.zip
```

`-output` 指向的目录必须事先不存在。

## 测试

测试全部使用 `httptest` 与可替换的 `RoundTripper`，不会连接真实的 `api.hualong.online`。
覆盖 manifest 结构、默认与自定义 `baseUrl`、SSE 增量聚合、usage、工具调用组装，
以及非 200 / 非 SSE 上游不泄漏响应体。

## 许可

Apache-2.0
