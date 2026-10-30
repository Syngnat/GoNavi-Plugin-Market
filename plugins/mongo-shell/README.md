# GoNavi Plugin: MongoShell

GoNavi 官方插件：在独立插件窗口中打开 MongoDB 的 JavaScript Shell。窗口通过
`host.connections.credentials` 拿到宿主已保存的 MongoDB 连接配置，Go 后端用
`go.mongodb.org/mongo-driver/v2` 建连，用 [goja](https://github.com/dop251/goja)
执行脚本；UI 不执行任何 `eval`。

插件 ID：`com.gonavi.mongo-shell`（Apache-2.0）。

## 功能

| 模块 | 说明 |
|---|---|
| 右键入口 | MongoDB 连接 / 数据库 / 集合右键菜单出现 **MongoShell**，打开独立窗口 |
| 启动上下文 | 读取 `window.gonaviPlugin.ready.context` 的 `connectionAction`：连接名、库、集合、环境、只读标记和完整连接配置 |
| 自动连接 | 窗口打开后自动 `connect`：10 秒连接 + ping + 数据库列表；顶部展示连接、库、集合、只读状态 |
| JS 脚本 | `db.getCollectionNames()`、`db.getCollection()`、动态 `db.<collection>`、`find/findOne/countDocuments/aggregate/insertOne/updateOne/deleteOne/runCommand`、`print`、`use` |
| 结果展示 | 最近一次查询的列/行表格（按 MongoDB 返回顺序）、`print` 输出、最后表达式结果、耗时与截断标记 |
| 只读保护 | 启动上下文为只读时，写方法、`$out`/`$merge` 聚合和写型 `runCommand` 在请求发出前即拒绝 |
| 资源边界 | 查询/聚合默认 200 行、最多 2000 行；集合名最多 1000 个；输出最多 500 行；命令 30 秒超时 |
| Agent 技能 | `ai.skills` 附带 `mongo-shell-recipes` 只读参考文档（显式加载，不注入提示词） |

## 工作台入口

MongoShell 同时提供工作台标签页入口：

- `contributes.pages` 声明 `shell` 页面，内容为内联 HTML（不加载外部资源）。
- `contributes.connectionActions` 中的 `open-shell-tab` 使用 `target: { "type": "page", "page": "shell" }`，在连接/库/集合右键菜单打开工作台标签。
- 页面通过 `plugin.invoke` 调用本插件已声明的 `connect`/`run` 命令；宿主注入连接配置，页面永远拿不到凭据。
- 页面启动上下文来自 `window.__gonaviLaunch`，只含 connectionId、库、集合、只读标记等非敏感字段。

## 窗口入口


- `contributes.pages.shell`：`ui/page.html`（内联进清单，仅工作台标签使用）
- 入口动作：`connectionActions` → `open-shell`，`connectionTypes=["mongodb"]`，
  `scopes=["connection","database","collection"]`，`target.type="window"`
- 窗口 UI 只通过 `window.gonaviPlugin.invoke` 调后端；不访问 `window.go`、Wails 绑定、
  宿主 store 或网络。
  ```html
  <link rel="stylesheet" href="/__gonavi/plugin-window/ui/style.css" />
  <script type="importmap">
    {"imports":{"@gonavi/plugin-ui":"/__gonavi/plugin-window/ui/index.js"}}
  </script>
  ```

  当前 UI 是零依赖 ES module + 宿主设计令牌（`--gonavi-*`）。宿主 UI SDK 的 React
  组件层需要 `react`，而宿主当前只映射 `@gonavi/plugin-ui` 一个入口，未导出
  `react`/`createRoot`；窗口按"只用 `gonaviPlugin.invoke`"的契约先用原生 DOM 实现。

## 权限

| 权限 | 用途 |
|---|---|
| `host.connections.credentials` | 从窗口启动上下文读取保存的 MongoDB 配置（含密码），用于建连；配置不落盘、不回传 UI |
| `host.log` | 写有界连接诊断（主机、只读状态；不含凭证） |
| `ai.skills` | 附带 `mongo-shell-recipes` 技能文档 |

`minHostVersion` 为 `1.2.0`：`connectionActions` 与插件窗口启动上下文是 1.2.0
新增能力，1.1.5 等低版本安装后不会渲染该右键入口。

插件进程拥有当前用户的 OS 权限；进程隔离与限额是可靠性边界，不是安全沙箱。

## 后端命令

stdio JSON-RPC v1（一行一帧，stdout 只写协议）。所有命令返回统一封装：

```json
{ "ok": true, "data": { } }
{ "ok": false, "error": { "code": "read_only", "message": "..." } }
```

### `connect`

参数：

```json
{
  "config": { "type": "mongodb", "host": "127.0.0.1", "port": 27017,
              "user": "root", "password": "***", "database": "app",
              "uri": "", "hosts": [], "authSource": "admin",
              "replicaSet": "", "readPreference": "", "mongoSrv": false,
              "mongoAuthMechanism": "", "connectionParams": "",
              "useSSL": false, "sslMode": "", "sslCAPath": "",
              "sslCertPath": "", "sslKeyPath": "", "protection": {} },
  "readOnly": false,
  "database": "app",
  "collection": "users",
  "connectionId": "conn-1",
  "connectionName": "local",
  "environmentType": "dev"
}
```

返回 `data`：`sessionId/connectionName/host/database/collection/readOnly/databases[]`。
每次成功 connect 返回独立的 256 位随机 `sessionId`（64 位十六进制字符串），不会替换其他页面或窗口的客户端。
10 秒内完成建连、ping 和数据库列表；失败返回 `connection_failed` 并释放客户端。

### `disconnect`

参数：`{ "sessionId": "<connect 返回的 ID>" }`。只关闭该会话并取消其在途请求；其他会话不受影响。

### `ping`

参数：`{ "sessionId": "<connect 返回的 ID>" }`。
返回 `{ "latencyMs": 3, "database": "app" }`。

### `run`

参数：`{ "sessionId": "<connect 返回的 ID>", "script": "...", "database": "app", "collection": "users", "maxRows": 200 }`。

返回：

```json
{
  "database": "app",
  "readOnly": false,
  "output": ["hello"],
  "result": null,
  "table": { "label": "users.find", "columns": ["_id", "name"],
             "rows": [["65f...", "a"]], "truncated": false },
  "elapsedMs": 12,
  "maxRows": 200,
  "timeoutMs": 30000
}
```

### 会话生命周期与协议兼容

- `run` / `ping` / `disconnect` 必须显式传入 `sessionId`；缺失、未知、已关闭或已回收的 ID 返回 `not_connected`，绝不回退到其他连接；字段类型错误返回 `invalid_argument`。
- 单进程最多 32 个会话（包含建连中和断开清理中），超限返回 `session_limit`；不驱逐仍活跃的新会话。
- 页面/窗口在 `pagehide` / `beforeunload` 尽力发送 disconnect；连接响应在卸载后才返回时也会回收。宿主已经移除 iframe 桥时，该消息可能丢失。
- 为回收丢失的卸载消息，每次 connect 最多回收一个空闲超过 30 分钟且没有在途请求的旧会话；锁外执行断开，最多占一个 5 秒清理预算，不启动后台 goroutine。被回收的旧页面再次执行会收到 `not_connected`，需重新打开连接；30 分钟内的遗留会话仍计入上限。
- SDK 关闭时先 Abort 使所有 ID 失效并取消请求，再 Close 对所有客户端执行有界清理；迟到 connect 不会重新发布会话。
- 工作台页和原生窗口均按 `{ok,data,error}` 解包；失败或缺少 sessionId 均不会进入 ready。
- 这是后端命令的显式协议变更：已有调用方必须保存 connect 返回的 sessionId 并在后续命令传回。旧 UI 与新后端、新 UI 与旧后端均应失败关闭；发布/部署时必须将 UI、manifest 内联页和二进制一起更新。
- 包内原有单客户端 `Shell` API 保持兼容，仅协议入口改用 `Sessions` 包层；只读策略仍由宿主注入及每个 Shell 的原有保护执行。

## JavaScript API

```js
print(...values)                 // 输出一行
use("databaseName")              // 切换当前库，返回 db
db.getName()
db.getCollectionNames()
db.getCollection("name")
db.runCommand({ ... })

db.users.find(filter, projection, limit) // 数组；limit 不超过 maxRows
db.users.findOne(filter, projection)     // 文档或 null
db.users.countDocuments(filter)
db.users.aggregate(pipeline)             // 数组；按 maxRows 截断
db.users.insertOne(document)
db.users.updateOne(filter, update, { upsert: true })
db.users.deleteOne(filter)
```

BSON 值在返回前转换为 JSON 安全值：`ObjectId` → 24 位十六进制字符串，
`Date` → RFC3339，`Decimal128` → 字符串，`Binary` → base64，嵌套文档/数组递归转换。

## 连接支持

- 直接填写 `host`/`port` 或 `hosts`（多种子），或提供完整 `uri`
  （`mongodb://` / `mongodb+srv://`，设置 URI 后其余连接字段不再拼接）。
- 支持 `user/password`、`authSource`（默认 `admin`）、`replicaSet`、`readPreference`、
  `authMechanism`（`NONE` 表示无认证）、`connectionParams`（覆盖默认 URI 参数）。
- TLS：`useSSL` / `sslMode`（`skip-verify` 映射 `tlsInsecure=true`）；
  `sslCAPath` 与客户端证书/私钥会构造成自定义 `tls.Config`。
- **不支持**：SSH 隧道、HTTP 代理/隧道、`mongoReplicaUser` 备用账号回退。检测到
  SSH/代理配置时连接直接失败并返回 `unsupported_transport`（不静默降级）。

## 资源与错误边界

| 项 | 值 |
|---|---|
| 连接超时 | 10 秒（`connectTimeoutMS` / `serverSelectionTimeoutMS`） |
| 命令超时 | 30 秒（goja `Interrupt` + driver context） |
| `find`/`aggregate` 行数 | 默认 200，最大 2000；`maxRows` 参数按此钳制 |
| 集合名数量 | 1000（超出标记 `truncated`） |
| `print` 输出 | 500 行，单行 4000 字符 |
| 错误码 | `invalid_argument`、`not_connected`、`unsupported_connection`、`unsupported_transport`、`connection_failed`、`query_failed`、`read_only`、`script_error`、`syntax_error`、`timeout`、`session_limit`、`internal` |

宿主窗口桥单次调用上限是 1 MiB，因此即使行数合规，单个超大文档仍可能让响应超限；
建议用 projection 或聚合收窄字段。

## 从源码构建

本插件位于 GoNavi 官方插件仓 `github.com/Syngnat/GoNavi-Plugin-Market`：

```sh
cd plugins/mongo-shell
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
go test -race ./...
node --test ui/page.test.mjs ui/window.test.mjs
```

打包发布版（默认编译 Windows amd64、Linux amd64/arm64、macOS amd64/arm64 五个平台；
`plugin-dist/` 已在 `.gitignore`，不要提交产物；`-output` 目录必须不存在）：

```sh
# 在 GoNavi-Plugin-Market 仓库根；每次选择尚不存在的输出目录。
go run ./cmd/buildtool \
  -manifest plugins/mongo-shell/gonavi.plugin.json \
  -output plugin-dist/mongo-shell-1.0.2 \
  -zip plugin-dist/gonavi-plugin-mongo-shell-1.0.2.zip
```

只验证本机时可加 `-platforms darwin/arm64`，产物名相应改为
`gonavi-plugin-mongo-shell-1.0.2-darwin-arm64.zip`。

1.0.2 起使用市场仓统一 `cmd/buildtool`。工具保留原始清单中的页面、右键动作等声明，
只替换生成的 `backend.platforms`，避免旧版 SDK 类型往返导致贡献字段丢失。
后端按 `CGO_ENABLED=0` 交叉编译；UI、技能、README 和许可证随同一个 ZIP 分发。

插件包静态检查（在 GoNavi 宿主仓执行；不执行插件代码）：

```sh
cd /path/to/GoNavi
go run ./cmd/gonavi plugin check /path/to/GoNavi-Plugin-Market/plugin-dist/mongo-shell
```

`go test` 默认只使用 fake 的 client/database/collection 接口，不依赖真实 MongoDB，
也不联网。指向一台一次性真实实例即可跑完整 driver + goja 链路：

```sh
MONGO_SHELL_TEST_URI=mongodb://127.0.0.1:27018 \
  CGO_ENABLED=0 go test ./internal/shell -run TestRealMongoRoundTrip -v
```

`CGO_ENABLED=0` 可绕开本机 macOS 的 SDK/链接器错配问题。

## 安装与手动验证

1. 插件管理 → 本地安装，选择上面的 ZIP，核对清单与权限后信任安装并启用。
2. 在 MongoDB 连接（或库/集合）右键 → **MongoShell**，窗口应自动连接并列出数据库。
3. 试跑：

   ```js
   db.getCollectionNames()
   db.getCollection("users").find({}, {}, 5)
   db.getCollection("users").countDocuments({})
   print("hello from GoNavi")
   ```

4. 在只读连接上执行 `db.users.insertOne({a: 1})`，应返回 `read_only` 且没有数据写入。

## 已知限制（v1）

- 只实现任务要求的 Shell 子集；`show dbs`、`db.stats()`、游标链式 `.limit()`、
  `EJSON`、`load()`、多语句事务等 mongosh 专有能力未实现。
- `runCommand` 的只读拦截基于命令名黑名单 + 聚合 `$out`/`$merge` 检测；只读会话下
  仍建议优先用查询方法。
- 窗口 UI 没有使用宿主 React 组件层（原因见"窗口入口"），后续宿主导出 `react` 后可直接
  切到 `GonaviThemeProvider`/`GonaviCodeEditor`。
- 30 秒命令超时与宿主窗口桥的 30 秒调用预算相同，极端情况下宿主可能先取消请求；
  正常超时会在插件侧返回 `timeout` 封装。
