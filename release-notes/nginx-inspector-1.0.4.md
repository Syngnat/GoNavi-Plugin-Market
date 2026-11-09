# Nginx Inspector 1.0.4

## Added

- 新建或编辑 SSH 连接时，私钥认证路径右侧新增「选择…」按钮，直接调用 GoNavi 插件窗口的宿主原生文件选择器选取本机私钥文件；选择后自动回填路径并聚焦输入框，取消则保持原值。
- 普通浏览器或不提供文件选择桥的宿主会禁用该按钮，并提示手动输入路径，不影响原有流程。

## Verified

- `node --check plugins/nginx-inspector/ui/app.js`
- `go test ./plugins/nginx-inspector/...`
- 本地窗口模拟 + Playwright：按钮可用、点击后路径回填、输入框与按钮单行对齐。

## Compatibility

- 宿主最低版本仍为 1.2.1，无新增权限，仅 UI 改动。
- 安装后在插件详情页重新打开独立窗口即可使用；旧的 1.0.3 窗口需关闭重开。
