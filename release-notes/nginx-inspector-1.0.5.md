# Nginx Inspector 1.0.5

## Fixed

- 修复“配置好连接后报 bridge_request_failed”：Windows“复制为路径”会把引号一起粘贴进私钥路径，插件随后按带引号的路径读取私钥失败；现在保存、读取已有连接和使用私钥前都会去掉首尾成对的单/双引号。
- 已经保存过的带引号连接无需删除重建，更新后读取时自动规范化。
- 左侧连接区不再重复显示名称与主机：状态点与连接下拉框同一行，名称只在下拉框出现一次，账号和端点只在下方出现一次。
- 窗口桥仍会把插件错误折叠成 bridge_request_failed，界面现在至少给出可操作提示（检查连接与私钥路径），而不是只显示原始错误码。

## Verified

- 用本机已保存的 BeroHost 连接实测：connections.test 1.56s 成功；host.info 返回 nginx 1.22.1、prefix /usr/share/nginx；config.tree 返回 132522 字节。
- `node --check plugins/nginx-inspector/ui/app.js`
- `go test ./plugins/nginx-inspector/...`
- 回归测试覆盖带引号路径的保存与读取。

## Compatibility

- 宿主最低版本仍为 1.2.1，无新增权限。
- 更新后重新打开独立窗口即可；已有连接无需重建。
