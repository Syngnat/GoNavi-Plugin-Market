# 连接右键动作（connectionActions）

`connectionActions` 把插件工具挂到连接、数据库或集合的右键菜单。宿主负责解析节点、校验权限、构造启动上下文并打开窗口；插件只拿到宿主给的那一份上下文，不能自行选择连接或路径。

## 清单

```json
{
  "window": { "entry": "ui/index.html", "title": "MongoShell" },
  "permissions": ["host.connections.credentials"],
  "contributes": {
    "connectionActions": [{
      "id": "open-shell",
      "title": "MongoShell",
      "description": "Open a JavaScript shell for this MongoDB connection",
      "connectionTypes": ["mongodb"],
      "scopes": ["connection", "database", "collection"],
      "target": { "type": "window" }
    }]
  }
}
```

- `connectionTypes`：与保存连接的大小写不敏感匹配，最多 32 个；只允许小写 ASCII、`.`、`_`、`:`、`-`。
- `scopes`：`connection`、`database`、`collection`；省略等价于 `["connection"]`。
- `target.type`：`window` 需要 manifest 声明 `window`；`page` 需要 `ui.pages` 且页面已声明，点击后在主工作台打开独立插件页标签。
- 动作 id 在插件内唯一，最多 16 个；窗口动作打开独立窗口，同一插件可同时打开多个窗口（宿主上限 16 个窗口）；页面动作按“动作/连接/库/集合”打开独立标签，同一实例会聚焦而不是重复打开。

## 启动上下文

窗口 bootstrap 返回：

```ts
type LaunchContext = {
  kind: 'connectionAction';
  actionId: string;
  connectionId: string;
  connectionName: string;
  connectionType: string;
  database?: string;
  collection?: string;
  environmentType?: string;
  readOnly: boolean;
  config?: ConnectionConfig;
};
```

窗口通过 `window.gonaviPlugin.ready.context` 读取。`config` 含宿主已解析的完整连接配置与凭据，只在 manifest 声明 `host.connections.credentials` 时出现；它是原生插件窗口，不是沙箱页面，插件作者必须把配置留在本进程内，不要写日志或上传。

页面动作通过主工作台标签打开，页面脚本读取 `window.__gonaviLaunch`。页面上下文**永远不含 `config`/凭据**，只有 connectionId、库、集合、标题与只读标记；页面需要数据时用 `query.read` 等能力按 connectionId 请求，宿主在进程内解析配置。

## 宿主行为

1. 只对 `desiredEnabled=true` 且实际 `running` 的插件显示菜单。
2. 选择节点后构造 `windowKey`：插件、动作、scope、连接、库、集合；同一 key 已打开时复用，不同 key 可并列打开。
3. 目标窗口继承插件代际；插件替换、禁用或故障时窗口被撤销。
4. 动作不新增宿主能力，也不授予连接授权；窗口仍只能调用 manifest 声明的权限与窗口桥。

## 安全边界

- 连接配置来自宿主保存记录，前端只提交节点标识，不能提交任意配置或路径。
- `readOnly` 是宿主计算的只读意图；插件必须遵守，宿主不会替插件改写命令。
- 凭据只应在 backend 内存中使用；不要写入 `localStorage`、DOM 或插件日志。
