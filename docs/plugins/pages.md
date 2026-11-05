# 插件页面、菜单与宿主能力（v1 契约）

本文件是「插件开放菜单／页面／导航 + 宿主能力网关」的 **v1 冻结契约**，同时作为实现与验收依据。目标是让插件像第一方功能一样进入 GoNavi 主窗口：声明页面、把页面挂到菜单／导航、在页面里按权限调用宿主能力。v1 只落地安全、可验证的最小闭环；其余能力按本文的目录分阶段开放。

实现位置：宿主校验在 [internal/plugin/pages.go](<../../internal/plugin/pages.go>)，唯一绑定在 [internal/app/methods_plugin_pages.go](<../../internal/app/methods_plugin_pages.go>)，双语言 SDK 在 [sdk/go/pages_validation.go](<../../sdk/go/pages_validation.go>) 与 [sdk/typescript/src/pagesDeclaration.ts](<../../sdk/typescript/src/pagesDeclaration.ts>)，可检查示例在 [examples/plugins/pages](<../../examples/plugins/pages/README.md>)。权限口径见[权限参考](<permissions.md>)，接线细节见[协议的清单与页面桥](<protocol.md#manifest-pages-and-navigation>)，SDK 用法见 [SDK 指南](<sdk.md>)。

## 1. 清单贡献

在现有 `contributes` 上新增两个可选数组（其余贡献点不变）：

```json
{
  "contributes": {
    "pages": [
      { "id": "overview", "title": "Overview", "body": { "type": "text", "text": "hello" } },
      { "id": "dashboard", "title": "Dashboard", "html": { "source": "<h1>…</h1>" } }
    ],
    "navigation": [
      { "id": "overview-tools", "title": "Overview", "page": "overview", "location": "tools", "order": 0 }
    ]
  }
}
```

导航也可以把清单声明的 `window` 挂到标题栏，此时不引用任何页面，也不需要 `ui.pages`：

```json
{
  "window": { "entry": "index.html", "title": "My Plugin", "width": 1200, "height": 760 },
  "contributes": {
    "navigation": [
      { "id": "open-window", "title": "My Plugin", "target": "window", "location": "tools" }
    ]
  }
}
```

### 1.1 pages

- `id`：`localIDPattern`（`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`），同插件内唯一。
- `title`：非空文本，≤256 字节（首尾空白不算内容）。
- `body` 与 `html` **恰好二选一**；两者都缺失或同时出现都拒绝。
- `body`：沿用现有声明式 `UINode` 校验（类型限 text/alert/column/row/button/table；深度 ≤16、节点 ≤512；button 只能引用已声明 command）。节点预算在**整个 manifest 的所有页面之间累计**，与 `panels` 一致。
- `html.source`：沿用 `HTMLPanel` 校验（非空 UTF-8、≤256 KiB、不含 NUL）。
- 最多 32 个页面。

### 1.2 navigation

- `id`：`localIDPattern`，同插件内唯一（与 pages 的 id 命名空间独立）。
- `title`：非空文本，≤256 字节。
- `target`：可选，`"page"`（默认）或 `"window"`；其他值直接拒绝。
  - `""`／`"page"`：打开页面标签页，`page` 必须引用本插件已声明的 page id。
  - `"window"`：打开清单 `window` 声明的独立窗口，要求存在 `window`，且**禁止同时声明 `page`**。
- `page`：目标为页面时必须引用本插件已声明的 page id；目标为 `window` 时必须缺省或为空字符串。
- `location`：接受 `"tools"`（标题栏「插件」菜单）、`"statusbar"`（底部状态栏）与 `"command"`（命令面板入口）。未知值拒绝。三种 target 共用这些位置。

`"command"` 只声明一个入口：点击后在主工作台打开插件页面或独立窗口，**命令面板的外观、命令列表与交互由插件自己渲染**，宿主不提供命令列表 API，也不会把插件命令混入宿主自己的命令项。
- `order`：可选整数，`-1000..1000`，默认 0；同插件内按升序展示，相同 order 按声明顺序。
- 最多 32 条导航。

`target` 缺省等价于 `"page"`，所以在该字段出现之前写下的清单保持原义。窗口目标与 `connectionActions` 的 `{"target":{"type":"window"}}` 是两套独立声明：前者挂标题栏入口，后者挂连接右键菜单，但两者都只打开清单自己声明的窗口，因此都不要求 `ui.pages`。

### 1.2.1 settingsSections

把插件页面嵌入宿主**固定位置**的设置区。宿主保留槽位、标题、说明与未安装时的安装引导，插件只提供页面内容：

```json
{
  "contributes": {
    "pages": [{ "id": "background", "title": "背景图片", "html": { "source": "<section>…</section>" } }],
    "settingsSections": [
      { "id": "background-settings", "slot": "appearance.background", "page": "background", "order": 0 }
    ]
  }
}
```

- `id`：`localIDPattern`，同插件内唯一。
- `slot`：v1 只接受 `"appearance.background"`（设置中心「主题与配色 → 背景图片」）。槽位由宿主拥有，位置、标题与未安装时的安装提示不随插件变化。
- `page`：必须引用本插件已声明的 page id。
- `title` / `description`：可选，≤128 / ≤512 字节；宿主槽位已有固定文案时以宿主为准。
- `order`：可选整数，`-1000..1000`，默认 0；同一槽位只允许一个提供者。
- 最多 8 个设置区。
- 未安装、停用或移除插件时，槽位仍显示原入口，并引导用户打开插件管理安装；已应用的背景由宿主继续绘制。

### 1.3 权限

| 权限 | 作用 |
|---|---|
| `ui.pages` | 渲染 `pages`、把页面导航挂到主窗口，并在设置中心声明的 `settingsSections` 位置渲染页面；声明了 `pages`、`settingsSections` 或存在非 `window` 目标的导航时缺失即校验失败 |
| `ui.html-panel` | 允许声明 `html` 页面（复用现有权限语义；v1 起同时允许页面通过桥调用已授权宿主能力） |
| `host.connections.read` | 读取脱敏后的保存连接列表 |
| `host.metadata.read` | 读取已保存连接的数据库／表／列元数据 |
| `host.query.read` | 通过已保存连接执行单条只读查询（见 2.3；v1 已接通只读连接、SQL 审计与超时路径） |
| `ui.sidebar` | 声明 `contributes.sidebar` 分组，把插件节点挂到左侧连接树（见 [sidebar](<sidebar.md>)） |
| `host.files` | 允许页面桥调用 `files.pick` / `files.save` 弹出宿主原生文件/目录选择与保存对话框；插件自行完成文件 I/O |
| `host.events` | 订阅宿主事件（偏好、插件生命周期、连接列表变化），通过长轮询取回 |
| `host.clipboard` | 读写系统剪贴板纯文本（读 64 KiB、写 256 KiB 上限） |
| `host.appearance.read` | 读取背景状态、内置图库目录与缩略图（页面桥，前端宿主能力） |
| `host.appearance.write` | 修改背景、透明度、位置、启用开关并管理内置图库（页面桥，前端宿主能力） |

- `ui.pages` 是页面、页面导航与设置区的前置条件；`host.*` 权限只影响对应能力调用，不隐式授予页面渲染。
- 全部导航都指向 `target: "window"` 时不需要 `ui.pages`：窗口目标只打开插件自己的窗口，与窗口连接动作同一套规则。
- 未声明 `ui.pages` 的包仍可只使用旧的面板／窗口能力。
- 只有处于 `running` 的插件才展示入口；静态包（无 backend）在运行时所有者启动后同样进入 `running`，只是没有子进程。

### 1.4 面板视图（views）

`views` 把已声明的页面停靠到主工作台右侧面板区，页面仍走页面桥与逐项能力授权：

```json
{
  "permissions": ["ui.pages"],
  "contributes": {
    "views": [
      { "id": "terminal", "title": "终端", "page": "terminal", "location": "right", "order": 0 }
    ]
  }
}
```

- `id`：`localIDPattern`，同插件内唯一；
- `title`：非空文本，≤128 字节；
- `page`：必须引用本插件已声明的 page；
- `location`：接受 `"right"`（右侧 dock）与 `"bottom"`（底部 dock）；
- `icon`：可选，包内栅格图（复用现有图标规则）；
- `order`：可选，`-1000..1000`；
- 最多 8 个视图。

入口可以是 `sidebar` 节点的 `open: {"type":"panel","view":"terminal"}`，也可以是宿主后续开放的其他入口。面板关闭后不保留状态；插件禁用、更新或卸载时宿主立即移除对应面板。

### 1.5 快捷键（keybindings）

`keybindings` 把已声明的页面或窗口挂到宿主全局快捷键：

```json
{
  "permissions": ["ui.pages"],
  "contributes": {
    "pages": [{ "id": "terminal", "title": "终端", "html": { "source": "…" } }],
    "keybindings": [
      { "id": "open-terminal", "title": "打开终端", "keys": "CmdOrCtrl+Shift+T",
        "target": { "type": "page", "page": "terminal" } }
    ]
  }
}
```

- `keys`：修饰键（`CmdOrCtrl`、`Ctrl`、`Cmd`、`Alt`、`Shift`）加一个主键，如 `CmdOrCtrl+Shift+T`；`CmdOrCtrl` 在 macOS 为 Cmd、其他平台为 Ctrl；
- `target`：`page` 需要 `ui.pages` 且页面已声明；`window` 需要清单声明 `window`；
- 最多 16 条，keys（忽略大小写）在同插件内唯一；
- 焦点在输入框/文本域/contenteditable 内时不触发；
- 与宿主快捷键冲突时宿主优先，插件应避免占用宿主已有组合。

### 1.6 页面资源与页面 UI Kit

沙箱页面可以加载**宿主同一 origin** 的两类资源；除此之外仍然不能加载任何外部资源：

- 包内文件：`/__gonavi/plugin-page-assets/<pluginId>/<package-relative-path>`。只有处于 `running` 的插件可读，路径经宿主包解析器校验，MIME 白名单为 js/mjs/css/html/json/png/jpg/jpeg/webp/gif/woff2，单文件 ≤32 MiB，响应 `no-store`；
- 页面 UI Kit：`/__gonavi/plugin-page-ui/ui/page-ui.js` 与 `/__gonavi/plugin-page-ui/ui/style.css`，以 IIFE 暴露 `window.GoNaviUI`（React、createRoot 与窗口 UI Kit 的全部基础组件）。

```html
<link rel="stylesheet" href="/__gonavi/plugin-page-ui/ui/style.css">
<script src="/__gonavi/plugin-page-ui/ui/page-ui.js"></script>
<script>
  const { React, createRoot, GonaviPanel, GonaviDataTable } = window.GoNaviUI;
</script>
```

页面 CSP 仍保持 `connect-src 'none'`、`base-uri 'none'`、`frame-src 'none'`：页面不能 fetch/XHR/WebSocket，也不能加载其他 origin；iframe 仍是不透明源，读不到宿主存储与 DOM。宿主源按运行平台放行：macOS 的 `wails://wails`（同时给出 `wails:` scheme-source），Windows/Linux 的 `http://wails.localhost`；`img-src` 额外允许 `blob:` 与 `data:`，`font-src` 允许宿主源与 `data:`。Monaco、chart、xterm 等重组件不在页面入口中，页面需要时自行从包内资源加载。

## 2. 宿主能力网关

### 2.1 绑定

```go
// 唯一的新增 Wails 绑定；其余能力都经它分派。
func (a *App) PluginPageCall(id, pageID, method, argumentsJSON string) connection.QueryResult
```

- 成功：`{ success: true, data: <能力结果> }`。
- 失败：`{ success: false, message: <i18n 文案>, data: { code } }`。
- 限制：参数 ≤64 KiB；结果序列化后 ≤256 KiB；执行超时 30s（网关计时：支持 context 的驱动可被中断，legacy 驱动可能继续执行，见 2.3）；Web 运行时未开启插件管理时整体拒绝并返回 `management_denied`。
- 校验顺序：插件存在 → 期望启用且实际 running → page 在该插件 manifest 中声明 → 方法已登记 → 调用方具备该方法的权限 → 参数合法 → 执行。
- 该绑定不是通用反射入口：方法必须逐项登记，未登记即 `unknown_method`。

### 2.2 错误码

失败码以 `page_` 为前缀返回：`data.code` 形如 `page_not_running`，文案键为 `plugin.backend.error.page_not_running`。条件码集合：`invalid_identity`、`not_running`、`not_declared`、`permission_denied`、`unknown_method`、`invalid_arguments`、`result_too_large`、`internal`；`unsupported` 保留给尚无法安全接通的扩展能力（当前已登记方法都不返回它）。前端同时兼容带前缀与裸码两种拼写。`management_denied` 沿用现有键且不带 `page_` 前缀，用于 Web 运行时未开启插件管理时的整体拒绝。前端桥本地错误用 `plugin.pages.error.bridge`／`plugin.pages.error.timeout`；页面拿到的 Promise 拒绝原因就是本地化后的文案。

### 2.3 能力目录 v1

> 本节保留 v1 的 7 个页面能力作为稳定基线。宿主随后扩展了 `connections.test`、`metadata.views/objects/allColumns/indexes/foreignKeys/triggers/createStatement/tableInfo/serverVersion/tableExists/databaseForeignKeys/charsets/collations`、`app.capabilities` 等只读能力；完整目录与通道/权限以 [capabilities](<capabilities.md>) 和运行时的 `app.capabilities` 为准。

| 方法 | 权限 | 参数 | 结果 |
|---|---|---|---|
| `app.info` | 无（需 page） | `{}` | `{ hostVersion, pluginId, pluginVersion, locale, runtime: "desktop" \| "web", capabilitiesVersion }` |
| `connections.list` | `host.connections.read` | `{}` | 脱敏数组：`{ id, name, type, host, port, database, environmentType, readOnly }`；不含用户名、密码、SSH／代理凭据、DSN |
| `metadata.databases` | `host.metadata.read` | `{ connectionId }` | `[string]`，≤2000 项 |
| `metadata.tables` | `host.metadata.read` | `{ connectionId, database }` | `[{ name, type? }]`，≤5000 项 |
| `metadata.columns` | `host.metadata.read` | `{ connectionId, database, table }` | `[{ name, type, nullable? }]`，≤5000 项 |
| `plugin.invoke` | `ui.pages` | `{ command, arguments?, connectionId? }` | 调用本插件 manifest 已声明的命令并绑定当前代际；持 `host.connections.credentials`、带 `connectionId` 且命令声明 `config` 时，宿主可向插件后端参数注入保存配置；后端不得把凭据回传页面 |
| `query.read` | `host.query.read` | `{ connectionId, sql, maxRows? }` | `{ columns: [{name,type?}], rows: [[…]], truncated, elapsedMs }`；maxRows 1..1000，默认 100；扫描层行预算为 maxRows+1 |
| `ui.notify` | 无（前端本地处理） | `{ message, type? }` | 无；在页面桥内直接展示 antd message |
| `system.open-external` | `system.open-external` | `{ url }` | `{ opened: true }`；只接受绝对 http(s) 链接且不含 `user:pass@`，其余返回 `invalid_arguments`；交给系统默认浏览器打开，不导航宿主 WebView；Web／headless 运行时返回 `unsupported` |

- 连接一律用**保存连接 ID** 解析；连接列表、元数据和读取查询不向插件返回凭据，宿主从现有仓储／secretstore 解析配置。`plugin.invoke` 的凭据注入仅面向明确授权的可信插件后端，不是凭据不会被插件代码接触的保证。
- `connections.list` 的 `readOnly` 是**有效只读**：连接模式本身支持只读且被强制只读时才为 `true`，不能当成「这次查询不会改数据」的承诺。多主机／副本集连接可能没有单一 `host`／`port`，这两个字段可为空。
- `query.read` 只接受单条只读语句（SELECT／WITH／SHOW／DESCRIBE／EXPLAIN），拒绝多语句、写入与可执行注释；在宿主现有审计路径（来源 `plugin_page`）中，必须使用驱动提供的**数据库强制只读事务**，不能仅依赖 SQL 分类。PostgreSQL 家族与 MySQL 已有专用执行方法；不支持只读事务的驱动拒绝调用，旧代理若没有 `query_read_only` RPC 也拒绝，绝不降级普通查询。更新宿主时需配套更新相关驱动代理。
- **只读 ≠ 无副作用**：只读事务阻止数据库写入，但 SELECT 仍可能触发锁等待或外部函数副作用。额外拒绝 `FOR UPDATE`／`FOR NO KEY UPDATE`／`FOR SHARE`／`FOR KEY SHARE`／`LOCK IN SHARE MODE`／`UPDLOCK`／`HOLDLOCK` 与 `nextval(`／`setval(`／`pg_terminate_backend(`／`pg_read_file(`，命中返回 `page_invalid_arguments`；`pg_advisory_lock`、`pg_sleep`、用户自定义函数等其他副作用未穷举，仍需按受信插件代码边界对待。
- `query.read` 在只读事务的**扫描层**绑定行预算（`maxRows+1`，用于区分恰好 maxRows 与超限），到预算即停止物化，查询结束始终回滚事务；结果序列化仍受 256 KiB 网关上限约束。
- **30s 超时**由网关计时，执行与扫描均传播请求 context；代理使用专用 RPC 携带超时与预算。缺少安全执行能力时拒绝调用。页面桥自身另有 30s 本地超时。
- 若未来某能力无法复用安全路径，应返回 `unsupported` 并归入阶段 2，**不得**降级为无审计直连。

### 2.4 能力扩展

新增能力 = 一条登记项（方法名、权限、参数上限、处理器）+ 文档 + 测试；处理器放 `internal/app/plugin_capability_*.go`，并登记到 `internal/app/plugin_capabilities.go` 的唯一注册表。目录按域分批开放（连接、元数据、查询、导出、AI、数据同步、驱动、设置、市场、JVM…），每批必须单独评审权限与审计边界。

## 3. 主窗口呈现

- 新增工作台标签页类型 `plugin-page`，`TabData.pluginPage = { pluginID, pageID, title }`。
- 页面内容：声明式页面复用 `DeclarativePanel`；HTML 页面复用 `sandboxedPanelDocument` 的 CSP 包装并注入桥运行时。
- 入口：
  1. 标题栏新增「插件」快捷菜单，列出所有 running 插件的 `navigation`（location=tools）：页面目标点击打开对应标签页，窗口目标打开清单声明的独立窗口；懒加载，首次打开菜单时读取，不改变启动时插件运行时初始化时机。
  2. 设置中心 →「插件」页每个插件卡展示其页面列表与「打开」按钮。
  3. `settingsSections` 声明的页面嵌入宿主固定设置槽位（v1：`appearance.background`），不额外生成标签页。
- 插件被禁用／移除／换代时：已打开页面显示不可用状态并停止桥调用；错误码 `not_running`。
- 页面标签页标题使用 navigation.title；同一 (pluginID,pageID) 复用已有标签页。窗口目标不使用标签页，沿用插件窗口既有的复用语义（同一插件已有窗口时复用，不另开一个）。
- 页面标签页**不跨重启恢复**：`plugin-page` 不在工作台 session snapshot 的恢复集合内，重启后需重新从菜单打开。
- Web 运行时且 `managementAllowed=false` 时：标题栏「插件」菜单不列出可点击条目，插件卡里的「打开」按钮禁用并显示提示（`plugin.pages.web_unavailable`）；桌面端或允许管理时保持原行为。

### 3.1 页面桥协议（HTML 页面）

- iframe：`sandbox="allow-scripts"`，不透明源，CSP 与现有 HTML 面板一致（禁止网络／外部资源／表单／嵌套 frame）。
- **网络边界只覆盖主动请求**：CSP 阻止页面 fetch／XHR／WebSocket、外部子资源、表单提交与嵌套 frame，但**不阻止页面自我导航**。为此宿主为每个 frame 生成随机 `token` 并注入桥 closure；`gonavi:page-call` 必须携带该 token，缺失或不符的消息直接丢弃（不回 `page-result`，也不触发宿主调用）。注入脚本执行后自移除，且出站 `postMessage` 在页面脚本之前绑定，页面无法从 DOM 读回 token 或替换发送入口。
- iframe 的 `load` 计数>1（srcdoc 初始加载之后再次 load）视为自导航逃逸：立即卸载 iframe、撤销桥并显示不可用态（`plugin.pages.unavailable` + `plugin.pages.error.navigation`）。
- 因此页面仍可能通过自我导航把**已经获得的数据**带出（例如跳转外部 URL）；这是受信插件代码的边界，不是网络隔离承诺。`host.*` 返回的数据应理解为「暴露给已声明该权限的插件代码」，不是「绝不会离开进程」。
- 注入运行时：`window.gonavi.call(method, args) -> Promise`、`window.gonavi.notify(message, type)` 与 `window.gonavi.resize(height)`。
- `resize(height)` 只对嵌入 settingsSections 的页面生效：页面把自身内容高度报给宿主，宿主据此设置 iframe 高度（120–4000 px）；标签页里的页面始终占满工作台。
- `appearance.*` 能力由**前端宿主窗口**执行（背景状态与图库适配器在主窗口），不经过 Go 页面网关；权限仍按清单校验。`appearance.background.set-custom` 的参数上限为 2 MiB，图片须是宿主可验证的 JPEG data URL（≤1.5M 字符）。
- 消息：页面 → 宿主 `{type:"gonavi:page-call", id, token, method, arguments}`；宿主 → 页面 `{type:"gonavi:page-result", id, ok, value|error}`。
- 宿主只接受 `event.source === iframe.contentWindow` 的消息；单页最多 8 个并发请求、每次调用 30s 超时、参数 ≤64 KiB（`appearance.background.set-custom` 例外，上限 2 MiB）；组件卸载后丢弃迟到响应。
- 限流：页面桥调用 30 次／10s 滑动窗口，`notify` 单独 10 次／10s；超限统一回 `plugin.pages.error.bridge`。
- 声明式页面没有 `window.gonavi`：它只能通过 `button.command` 调用插件自己的命令。

## 4. i18n

新增键（六语言同步）：

- `plugin.pages.title`、`plugin.pages.open`、`plugin.pages.empty`、`plugin.pages.unavailable`、`plugin.pages.menu`、`plugin.pages.loading`
- 页面桥运行时错误：`plugin.pages.error.bridge`（含限流）、`plugin.pages.error.timeout`、`plugin.pages.error.navigation`（自导航逃逸）
- Web 入口提示：`plugin.pages.web_unavailable`
- 权限标签：`plugin.permission.ui_pages`、`plugin.permission.host_connections_read`、`plugin.permission.host_metadata_read`、`plugin.permission.host_query_read`
- 后端文案：`plugin.backend.error.page_` + 上面 9 个 code（`management_denied` 已存在，不重复）

## 5. 测试与验收

- Go：`go test ./internal/plugin -run 'Page|Navigation|Manifest'`；`go test ./internal/app -run 'PluginPage'`；`go build ./...`；`go vet` 相关包。
- SDK：`cd sdk/go && go test ./...`；`node frontend/node_modules/typescript/bin/tsc -p sdk/typescript/tsconfig.json`；`node frontend/node_modules/vitest/vitest.mjs run --root sdk/typescript --config vitest.config.mjs`；示例包 `cd examples/plugins && go test ./pages`。
- 前端：`npm run test -- src/plugins src/pluginPages`；`npx tsc --noEmit`；六语言键对齐测试。
- 文档：`go test ./internal/plugin -run 'Docs|Link'`；`tools/i18n-scan` 不新增硬编码中文。
- 真实渲染（Wails 窗口）单独标注为未验证，不以单测代替。

## 6. 非目标（v1 明确不做）

- 不把全部 483 个 Wails 绑定直接暴露给插件；能力必须逐项登记、授权、审计。
- 不提供原生菜单栏／右键菜单注入（`location` 先只做 tools）。
- 不提供页面自定义网络请求；HTML 页面的 CSP 保持 `connect-src 'none'`。
- CSP 不是网络隔离：只阻止页面主动请求，不阻止自我导航。导航后宿主撤销桥并显示不可用，但已获得的数据仍可能被带出（受信插件边界）。
- 不提供插件间页面互调。
