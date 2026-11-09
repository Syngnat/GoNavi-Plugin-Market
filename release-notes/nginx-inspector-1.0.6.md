# Nginx Inspector 1.0.6

## Fixed

- 修复「校验」页误报请求失败：内部校验结果在成功时返回的是 typed-nil 指针，被 SDK 包装成非空错误，界面因此显示“请检查 SSH 连接与私钥路径”。现在成功时正确返回 `ok=true`，只有真实失败才报错。
- 修复配置、证书、日志、upstream 页面长时间停留在“加载中”：配置树改为优先使用一次 `nginx -T` 的配置转储（保留逐文件读取作为回退），并在插件进程内按连接缓存 2 分钟；打开「配置」页可点“刷新”绕过缓存。
- 修复 `sshx.ReadFile` 每个文件两次 SSH 往返（先 `wc -c` 再 `cat`），改为单次有上限读取；证书文件并发读取。
- 修复「访问统计」实际只分析 2 MiB 却显示 20 MB：现在同时显示请求窗口与实际分析字节。
- 修复 upstream 探测把 SSH 传输失败或 `AllowTcpForwarding no` 误标为 down；跳过成员与 backup 成员单独标记。
- 修复 upstream 裸 `host:port` 未校验端口范围（如 `host:0`、`host:99999`）而误报 down 的问题，非法端口现在标记为 skipped。
- 修复「配置」文件列表对同一 include 文件重复渲染的问题：现在按路径去重，并在文件名后标注被 include 的次数。
- 修复错误码 `PATH_NOT_ALLOWED` 在界面显示为原始错误码的问题。

## Changed

- 「服务」页改为 KPI 卡片，展示版本、状态、PID、运行时长、systemd/容器/unit。
- 「访问统计」改为双列 KPI 与更紧凑的状态码/路径表。
- 「日志」页明确显示当前日志来源（配置解析结果或 `nginx -V` 默认值）、路径、窗口行号与截断提示。
- 「证书」页增加概览 KPI、不可读/解析失败/过期分组。
- 「upstream 探测」进入页面后自动探测，展示 skipped/disabled/backup 状态与原因。
- 「连接管理」增加用途说明：SSH 凭据保存在本机插件数据目录，用于远程只读诊断；密码/口令不回传到界面。

## Verified

- BeroHost（root@94.103.173.47:37167，nginx 1.22.1）实测：`nginx -T` 约 1.7–2.3s；配置树 28 个解析条目（23 个唯一文件）、132522 字节；证书 0.68s、日志 0.71s、upstream 0.16s、访问统计 0.88s。
- `go build ./plugins/nginx-inspector/...`
- `go test ./plugins/nginx-inspector/...`
- `node --check plugins/nginx-inspector/ui/app.js`
- `node scripts/verify-plugin-catalog.mjs`（发布并更新 catalog 后）

## Compatibility

- 宿主最低版本 1.2.1，权限不变（ai.tools、ai.skills、host.log、ui.html-panel）。
- 更新后重新打开独立窗口；已有连接无需重建。
- 无网络访问；`nginx -T` 需要远端账号可执行 nginx 并读取配置；不可执行时自动回退旧的逐文件读取。
