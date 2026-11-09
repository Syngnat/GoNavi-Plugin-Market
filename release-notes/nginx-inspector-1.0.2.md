# Nginx Inspector 1.0.2

1.0.2 是一次可用性与正确性修复版。1.0.1 只补了窗口权限声明，配置树、证书巡检、日志路径与访问统计
在标准 nginx 配置下仍会给出错误或空白结果；1.0.0 则会被 v1.2.1+ 宿主直接拒装。本版修复这些缺陷，
补齐路径白名单与帧预算，并把插件发布真正接上市场目录。

## 安装要求

- 宿主 **GoNavi ≥ 1.2.1**。清单新增 `ui.html-panel` 权限（宿主自 v1.2.1 起强制要求它才能打开插件窗口），
  `minHostVersion` 同步提高到 1.2.1。旧宿主请先升级 GoNavi。

## 主要修复

### 发布链

- 清单保留 `ui.html-panel`，修正 1.0.0 在 v1.2.1+ 宿主上“window requires the ui.html-panel permission”的拒装；
  1.0.1 虽有该权限，但从未写入市场目录，客户端仍只看到 1.0.0。
- 新增 tag 触发的 Release 流水线：五平台构建、`SHA256SUMS.txt`、按 tag 生成 Release，并用构建产物的真实
  sha256/size 更新 `public/catalog.json`（版本、installSpec、artifact 一次写全）。
- 新增插件 CI（gofmt/vet/test/build + catalog 一致性校验）与 `.gitattributes`（Go 文件 LF），消除 CRLF 噪声。

### 配置树

- `include /etc/nginx/conf.d/*.conf;` 这类通配 include 之前因远端 shell 不展开引号内 glob 而被静默丢弃；
  改为远端 `for f in ...` 展开；超过 64 个文件时在 `warnings` 里报告跳过数量并置 `truncated`。
- include 去重从全局 `seen` 改为递归栈：同一 snippet 被多个 server include 时会各自展开，不再丢指令。
- 解析器按 nginx 词法重写：词内 `#` 不再被当注释（`proxy_pass http://host/#frag` 不再整棵树解析失败）、
  精确的反斜杠转义集、引号可在词中开合、变量保留。
- 单文件超过 512 KiB 或词法错误时回退到最后一个完整指令边界并返回部分结果（`error`/`warnings`），
  不再整页失败；节点预算改为全局共享并如实置 `truncated`。
- `FindServers` 不再下降进 `upstream`/`mail`；`listen 443 ssl` 才判定 TLS；`location` 路径不再返回 `~`/`=` 修饰符。

### 证书

- `ssl_certificate` 改为递归收集（之前只看顶层，标准站点永远返回空列表），支持相对路径按 prefix 解析、
  绝对路径去重、`$变量` 标记为跳过。
- 计数拆分为 `skipped` / `unreadable` / `parseErrors`；补齐 `file`、`commonName`、`parseError`，
  `daysLeft` 为 0 不再被省略；PEM 首块不是证书（key+cert 拼接）也能正确找到证书块。

### 安全

- 所有按路径读取（配置、日志、证书）统一走白名单（`/etc/nginx`、`/etc/ssl`、`/etc/letsencrypt`、
  `/var/log/nginx`、OpenResty/Homebrew 等 prefix），越界返回 `PATH_NOT_ALLOWED` 并给出允许根目录；
  拒绝 `..`、NUL/CR/LF 与根路径。来自远端配置的 `access_log` 绝对路径同样受白名单约束。
- 连接保存改为字段级合并：编辑时留空密码/私钥口令不再清空凭据；省略 `fingerprint` 会保留已钉住的 TOFU 指纹
  （1.0.0 每次保存都会静默解除指纹钉扎）。显式传入 `fingerprint` 才会重新钉住。
- SSH keepalive、取消后关闭迟到连接、命令超时不再无限等待。

### 结果可信度

- `access.stats` 只分析传输能承载的最新字节，并如实回报 `requestedBytes`/`analyzedBytes`；
  之前请求 20 MiB 实际只解析最旧 2 MiB 却仍报 20 MiB。`request_time` 与 `upstream_response_time`
  拆成两个指标，支持 nginx 默认 `\xHH` 转义与多值耗时；gzip 日志标记 `gzipUnsupported` 而不是假装解析。
- 日志路径优先取自合并后的配置（`access_log`/`error_log`，含 `http` 级），`off`/`syslog:`/`memory:`/`$变量`
  会给出 `source`+`note`；回退 `nginx -V`。
- `logs.search` 增加 `windowLine` 与 `windowStartOffset`，明确行号是窗口内相对值（旧 `line` 字段保留一版）。
- `logs.tail`/`logs.search` 输出限制在 512 KiB（宿主单帧 1 MiB），避免大日志行把整个调用打成帧错误；
  尾部按字节窗口读取并在本地裁剪行数，截断如实置 `truncated`。
- `service.reload` 在信号/systemd 返回后读取 `error_log` 尾部，只有未出现 `[emerg]`/`[alert]` 才报 `applied`，
  否则 `unknown`；自动识别 nginx/openresty/tengine unit，systemd 失败回退 `-s reload` 并置 `fallbackUsed`；
  非 root 导致的权限类 `-t` 失败单独返回 `RELOAD_BLOCKED reason=permission`。
- `upstreams.check`：20 秒整体上限、8 并发、3 秒单探；`down` 成员不探测，`backup` 单独列出，unix socket 跳过。
- `host.info` 缓存 10 分钟并支持 `refresh`，发现脚本合并为一次 SSH 往返，新增 `container` 字段。

### 界面

- 切页使用代际令牌，旧响应不会覆盖新页面；切页不再触发连接测试。
- 连接行内提供测试/编辑/删除，删除使用模态确认；连接表单端口校验，留空密码不提交。
- 日志截断、配置截断、统计“实际分析字节”、证书跳过/解析失败、上游跳过均有可见提示。
- 增加 CSP、跳过导航链接、`aria` 属性、焦点陷阱与 Esc 关闭、`prefers-reduced-motion`。

## 兼容性说明

- 新增字段均为增量：`PATH_NOT_ALLOWED` 错误码、`config.tree.error/warnings`、`stats.requestedBytes/analyzedBytes/requestTime/upstreamTime/gzipUnsupported`、
  `logs.*.source/note/windowLine`、`certs.skipped/parseErrors`、`service.*.status/unit/fallbackUsed`、`hostinfo.container`、
  `connections` 输入的 `fingerprint`。旧字段至少保留一版。
- 读取范围收紧：日志/证书目录之外的自定义路径现在返回 `PATH_NOT_ALLOWED`（可在 `internal/commands/paths.go` 的
  允许根目录中扩展）。
- 空 `fingerprint` 表示“保留旧值”，暂不支持显式清除；需要清除时删除连接后重建。

## 验证

- `go build`、`go vet`、`go test ./plugins/nginx-inspector/...` 全绿（commands/ngxconf/sshx/store）。
- 新增本地 fake SSH server 集成测试，覆盖证书递归与白名单、访问统计截断与 gzip、upstream 探测语义、
  PEM 前导块、连接保存保留密码与指纹。
- 新增配置树/解析器回归测试（词内井号、修饰符、upstream 幽灵 server、include 去重与上限）。

## 已知边界

- 路径白名单是词法检查：白名单目录内的符号链接仍可指向别处（受 SSH 用户自身文件权限约束）。
- 连接凭据仍以 0600 明文保存在插件数据目录，未接入 OS keychain。
- SSH 连接池无 single-flight，并发首连可能拨号两次。
