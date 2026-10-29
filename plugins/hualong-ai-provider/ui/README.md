# 账号面板 UI

`gonavi.plugin.json` 的 `window.entry` 指向本目录的 `index.html`，宿主会把该页面加载进
一个隔离的插件窗口进程，并在任何插件脚本之前注入 `window.gonaviPlugin` 桥。

- `index.html`：窗口结构，包含未登录 / 已登录 / 出错三种视图
- `app.js`：纯原生 ES 模块，只用 `window.gonaviPlugin.invoke(command, args)` 调后端命令
- `styles.css`：全部样式，含浅色与深色配色；页面内没有内联样式

约定：

- 后端命令返回 `{ok:true,data}` 或 `{ok:false,error:{code,message}}`，`app.js` 统一在这里拆信封
- 登录后每 2 秒轮询 `account.poll`，最多 5 分钟；读到 `ready` 即停止轮询并刷新 `account.status`
- Key 值默认打码为 `sk-xxxx****xxxx`，行内「显示」「复制」两个动作按需展开
- 页面不直接访问网络、不读写 `localStorage`，令牌只存在于后端的数据目录
