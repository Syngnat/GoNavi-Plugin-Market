# 插件 UI SDK（第一切片）

GoNavi 为插件窗口提供与宿主同源的设计令牌、主题、布局、表格、编辑器、弹窗和 antd 组件，同时提供能力客户端。插件 UI 不导入宿主 store、Wails 绑定或主窗口代码；数据一律通过能力获取。

## 加载方式

插件窗口由宿主同源 loopback origin 提供。插件包内的 `ui/index.html` 用 import map 指向宿主运行时，不需要打包 React 或 antd：

```html
<link rel="stylesheet" href="/__gonavi/plugin-window/ui/style.css">
<script type="importmap">
{"imports":{"@gonavi/plugin-ui":"/__gonavi/plugin-window/ui/index.js"}}
</script>
<script type="module" src="app.js"></script>
```

宿主按流式方式提供这些资源，不设固定 MB 上限；插件包与包内资源也按条目流式校验。资源请求仍受窗口 token、代际和生命周期约束。

## 最小示例

```tsx
import React from 'react';
import { createRoot } from 'react-dom/client';
import {
  GonaviThemeProvider, GonaviPanel, GonaviToolbar, GonaviDataTable,
  GonaviCodeEditor, createGonaviClient, Button,
} from '@gonavi/plugin-ui';

const client = createGonaviClient();

function App() {
  const [rows, setRows] = React.useState([]);
  const [sql, setSql] = React.useState('select 1');
  const run = async () => {
    const result = await client.call('query.read', {
      connectionId: 'conn-1', sql, maxRows: 100,
    });
    setRows(result.rows.map((values, index) => ({ key: index, values })));
  };
  return (
    <GonaviThemeProvider mode="dark">
      <GonaviToolbar><Button onClick={run}>Run</Button></GonaviToolbar>
      <GonaviPanel title="Console">
        <GonaviCodeEditor language="sql" value={sql} onChange={setSql} />
        <GonaviDataTable height={320} rows={rows} columns={[{ title: 'Row', dataIndex: 'key' }]} />
      </GonaviPanel>
    </GonaviThemeProvider>
  );
}

createRoot(document.getElementById('root')!).render(<App />);
```

## 窗口启动上下文

连接/库/集合右键动作打开的窗口，会通过 bootstrap 带一份宿主构造的 `context`：

```ts
const identity = await window.gonaviPlugin.ready;
if (identity.context?.kind === 'connectionAction') {
  const { connectionId, database, collection, config } = identity.context;
  await window.gonaviPlugin.invoke('connect', { connectionId, database, collection, config });
}
```

`config` 只在 manifest 声明 `host.connections.credentials` 时出现。窗口必须把凭据留在本进程内，不要写日志、DOM 或远程请求。

## 能力客户端

- 插件窗口：`window.gonaviPlugin.capability(method, args)` 返回 `{ ok, data, error }` envelope。
- 插件页面：`window.gonavi.call(method, args)` 直接返回数据。
- `createGonaviClient()` 自动选择当前通道并把失败 envelope 转成 `GonaviCapabilityError`，其 `code` 是稳定能力错误码。
- 能力清单和权限以运行时的 `app.capabilities` 为准；不要假设某个新能力一定存在。

## 已导出

- 主题：`GonaviThemeProvider`、`useGonaviTheme`、`--gonavi-*` CSS 变量。
- 布局：`GonaviPanel`、`GonaviToolbar`、`GonaviEmptyState`、`GonaviLoadingState`、`GonaviErrorState`。
- 数据：`GonaviDataTable`（只读）、`GonaviResultGrid`（按行推断列）、`GonaviJsonViewer`、`GonaviTreeView`、`GonaviCodeEditor`（轻量编辑器）、`GonaviConfirmDialog`。
- Hooks：`usePluginContext`（读取窗口启动上下文）、`useCapability`（单次能力调用状态）、`useGonaviTheme`。
- antd：Button、Input、Select、Modal、Tabs、Table 等常用组件与 `GonaviIcons`。
- 能力：`createGonaviClient`、`GonaviCapabilityError`。

复杂宿主组件（完整 DataGrid 编辑态、QueryEditor、TableDesigner、连接树）在后续切片通过 Host Surface 或适配器导出；当前先由宿主标签页/对话框渲染，插件通过能力打开。

## 边界

- 不直接暴露宿主 DOM、Zustand store、Wails 绑定或任意文件路径。
- 页面/窗口桥仍受 30 秒/1 MiB 的单次调用约束；大数据用游标/分批能力。
- UI SDK 版本随宿主发布；插件应通过 `app.info.capabilitiesVersion` 与 `uiSdkVersion`（后续补入）判断兼容性。
