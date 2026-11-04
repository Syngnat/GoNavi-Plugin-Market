# 插件侧边栏节点（sidebar）

> 状态：**已实现（宿主 v1）**。清单新增 `contributes.sidebar` 与 `ui.sidebar` 权限；宿主在连接树上方渲染分组，节点由 `listCommand` 返回并在运行时重新校验。`navigation.location` 仍只接受 `tools`。本文定义宿主侧能力，插件作者据此声明自己的树节点；宿主不为任何具体插件写死。

## 1. 目标

让插件把自己的工具挂进 GoNavi 左侧连接树：

- 一个插件可贡献一个或多个分组节点（如「SSH」「Mongo」）；
- 分组可展开/折叠，子节点由插件命令返回，宿主只做文本渲染；
- 点击节点在主工作台标签（数据预览所在区域）打开插件页，或打开插件独立窗口；
- 插件禁用、更新、故障时节点立即回收，不残留失效入口。

SSH 场景即：分组「SSH」展开后列出主机配置，点击某一台在主工作台打开终端页，而不是先弹独立窗口。

## 2. 非目标

- 不把 SSH、Mongo 等具体工具内置进宿主；
- 不允许插件向连接树注入宿主连接、凭据或任意文件路径；
- 不允许插件自定义节点 HTML、CSS 或图标字体；
- 不改变现有 `connectionActions`、`navigation` 的语义。

## 3. 现有能力盘点

| 能力 | 位置 | 凭证/桥 | 是否可承载终端 |
|---|---|---|---|
| `window` | 独立 Wails 窗口 | window bridge | 是，但不在主工作台 |
| `pages` | 主工作台标签（`WorkbenchTabContent` 的 `pluginPage`） | page bridge（`plugin.invoke`） | 可以，受桥限制 |
| `panels` | 主窗口内嵌声明式/HTML 面板 | 无桥 | 只读展示 |
| `navigation` | 标题栏「插件」菜单，`location` 仅 `tools` | 按 target | 入口，不是树节点 |
| `connectionActions` | 连接/库/集合右键菜单 | 按 target | 入口，不是树节点 |

结论：**「在主工作台展示」已经具备**（pages + workbench 标签），缺的是「插件向左侧树贡献节点」。因此本提案只新增 `sidebar`，打开目标复用既有 page/window 机制。

## 4. 清单契约

```json
{
  "permissions": ["ui.sidebar", "ui.pages"],
  "contributes": {
    "sidebar": [
      {
        "id": "ssh",
        "title": "SSH",
        "icon": "icons/ssh.png",
        "listCommand": "profile.list",
        "emptyText": "还没有 SSH 配置",
        "open": { "type": "page", "page": "terminal" }
      }
    ]
  }
}
```

### 4.1 分组字段

| 字段 | 约束 |
|---|---|
| `id` | `localIDPattern`，同插件内唯一 |
| `title` | 非空文本，≤128 字节 |
| `icon` | 可选，包内栅格图（复用现有 icon 规则，≤5 MiB） |
| `listCommand` | 必填，必须是本插件已声明的 command；宿主调用时不传参数 |
| `emptyText` | 可选，空列表文案，≤128 字节 |
| `open` | 可选默认打开目标；节点可覆盖 |
| `order` | 可选，`-1000..1000`，同一插件内排序 |

上限：每个插件最多 4 个分组，全部节点合计 ≤500 个、深度 ≤4、序列化后 ≤256 KiB。

### 4.2 节点模型（`listCommand` 返回值）

```json
{
  "items": [
    {
      "id": "prod-web-01",
      "label": "prod-web-01",
      "description": "root@10.0.0.1",
      "badge": "已连接",
      "children": [
        { "id": "sftp", "label": "SFTP", "open": { "type": "page", "page": "terminal", "arguments": { "view": "sftp" } } }
      ],
      "open": { "type": "page", "page": "terminal", "arguments": { "profileId": "prod-web-01" } }
    }
  ]
}
```

- `id`：插件内稳定标识，≤128 字节，不允许路径分隔符；
- `label`、`description`、`badge`：纯文本，宿主按文本渲染，不接受 HTML；
- `children`：可选，递归结构，受深度与总数限制；
- `open`：可选，缺省继承分组；`type` 为 `page` 或 `window`；
- `open.arguments`：≤4 KiB 的 JSON 对象，原样进入页面 `window.__gonaviLaunch` 或窗口 launch context；
- 未知字段拒绝，重复 `id` 拒绝。

### 4.3 打开目标

- `page`：要求该 page 在清单中已声明且插件具备 `ui.pages`；宿主调用现有 `openPluginPageTab`，同一 `(page, arguments)` 标签聚焦而非重复打开；
- `window`：要求清单声明 `window`；宿主复用现有窗口打开路径，同一 `windowKey` 复用；
- 缺省：分组没有 `open` 且节点也没有 `open` 时，节点仅作为分组展示。

### 4.4 刷新

- 宿主在分组首次展开时调用 `listCommand`，结果缓存 30 秒；
- 插件贡献失效事件（安装、更新、启用、禁用、卸载）立即清空缓存；
- 手动刷新按钮触发一次重新拉取；
- `listCommand` 超时 10 秒、结果超过 256 KiB 视为失败，该分组显示错误与重试，不影响连接树其他部分。

## 5. 权限与安全

| 权限 | 作用 |
|---|---|
| `ui.sidebar` | 声明并渲染侧边栏分组；缺失即清单校验失败 |
| `ui.pages` | `page` 打开目标的前置条件 |
| `ui.html-panel` | 仅当 page 使用自定义 HTML 时需要，沿用现有规则 |

边界：

- 节点数据是**数据**，宿主只做文本渲染；不接受 HTML、URL scheme、图标字体或任意路径；
- `listCommand` 只能调用本插件已声明命令，宿主不注入连接配置或凭据；
- 节点 `arguments` 由插件提供、宿主透传，页面仍按现有页面网关逐项授权（`host.connections.read` 等），侧边栏不新增任何宿主能力；
- 插件必须处于 `running` 且代际匹配；禁用、更新、故障时节点随代际失效立即移除；
- 列表结果受数量、深度、字节上限约束，解析失败按分组错误处理。

## 6. 宿主实现

| 层 | 位置 | 内容 |
|---|---|---|
| 契约 | `sdk/go/manifest_validation.go` | `SidebarPermission`、`validateManifestSidebar`：宿主与市场共用的单一校验实现 |
| 宿主类型 | `internal/plugin/types.go`、`internal/plugin/sidebar.go` | `SidebarGroup`、`Contributes.Sidebar` 与 legacy 校验路径 |
| 运行时 | `internal/app/methods_plugin_sidebar.go` | `PluginSidebarNodes(pluginID, groupID)`：校验运行态/代际/权限 → 调用 `listCommand` → 解析并重新校验节点树；`PluginOpenSidebarWindow` 打开窗口目标 |
| 前端 | `frontend/src/components/sidebar/PluginSidebarSection.tsx`、`frontend/src/plugins/sidebarApi.ts` | 分组渲染、展开拉取、缓存 30 秒、空态/错误/重试 |
| 打开 | 复用 `openPluginPageTab` / 窗口打开路径 | page 标签携带 `{kind:"sidebar", pluginId, groupId, nodeId, arguments}`；window 由宿主构造同一上下文 |
| 失效 | 复用 `subscribePluginContributionInvalidation` | 插件安装、更新、启用、禁用时清空分组缓存 |

运行时失败码：`sidebar_not_running`、`sidebar_not_declared`、`sidebar_invalid_identity`、`sidebar_invalid_result`、`sidebar_result_too_large`、`sidebar_unavailable`、`sidebar_internal`。前端把它们映射到通用错误文案并允许重试。

实现约束：

- `PluginSidebarNodes` 不是通用反射入口，命令必须逐项校验；
- 解析节点树时拒绝未知字段与重复 id，深度/数量/字节超限一律失败；
- 分组渲染失败不得影响连接树其余节点，也不得阻塞启动。

## 7. 兼容性

- 宿主清单解析使用 `DisallowUnknownFields`，**旧宿主会拒绝含 `sidebar` 的清单**。因此发布顺序固定为：宿主先支持并发布 `ui.sidebar`，插件再声明并提升 `minHostVersion`；
- 市场服务端会用宿主二进制复核上传包，能阻止「声明了 sidebar 但目标宿主装不了」的包上架；
- 未声明 `sidebar` 的现有插件不受影响；
- `navigation.location` 保持只接受 `tools`，不新增 `sidebar` 位置，避免两套树入口语义重叠。

## 8. 验收

**清单**

- 缺 `ui.sidebar`、`listCommand` 未声明、`page` 未声明、`window` 未声明、超限、重复 id 全部拒绝；
- 未声明 `sidebar` 的清单保持通过。

**运行时**

- 插件禁用/更新/故障后分组立即消失，旧代际响应不复活节点；
- `listCommand` 超时、超限、返回非法 JSON、重复 id 均显示分组错误且可重试；
- 节点 `arguments` 原样到达页面 launch context / 窗口 launch context。

**前端**

- 展开/折叠、缓存、手动刷新、空态、错误态；
- 点击 page 节点打开主工作台标签并聚焦既有标签；
- 点击 window 节点按 windowKey 复用窗口；
- 连接树其他节点行为不变。

**端到端**

- `examples/plugins/sidebar` 示例插件：安装后在树中出现分组，展开列出节点，点击在数据预览区域打开页面。

## 9. 分阶段

1. **v1**：一次性返回有界树、page/window 目标、手动刷新、缓存 30 秒；
2. **v2**：`expandCommand` 懒加载子节点、分组内搜索、`badge` 随事件更新；
3. **v3**：节点引用已保存连接（仍由宿主解析，插件只拿脱敏标识）。

## 10. 开放问题

- 分组在树中的位置：建议放在连接树顶部的独立「插件」区，而不是混入连接列表；
- 是否允许同一插件多个分组：v1 允许最多 4 个，按 `order` 排序；
- 刷新策略：缓存 30 秒 + 手动刷新是否足够，还是需要插件推送事件；
- 与 `connectionActions` 的分工：树节点用于「浏览与打开」，右键菜单继续用于「对某个连接执行动作」。
