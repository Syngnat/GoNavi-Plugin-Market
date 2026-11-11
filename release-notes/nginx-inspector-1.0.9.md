# Nginx Inspector 1.0.9

## Fixed

- **访问统计读错日志文件**:`access.stats` 之前只看 `nginx -V` 的编译期
  `--http-log-path`,站点在配置里改了 `access_log` 时会去分析默认的空文件。现在与日志页
  一致,优先使用配置中的 `access_log`,并在响应里回传 `source`(`config` / `nginx -V` /
  `explicit`)与 `note`;配置为 `off`、syslog 或含变量路径时直接说明原因,不再错误地分析
  默认文件。
- **统计窗口上限名不副实**:20/50/100 MB 档位实际被单条 SSH 命令 2 MiB 的输出上限截断,
  却没有任何提示。窗口改为流式读取、边读边聚合,请求多少分析多少;`truncated` 字段补上并
  在界面提示。
- **include 展开绕过路径白名单**:配置树里的 `include` 之前直接读取任意路径(如
  `include /etc/shadow;`),既可读到白名单外的文件,又和 `config.get` 的
  `PATH_NOT_ALLOWED` 自相矛盾。现在 include 与 `config.get` 用同一套校验,越界项以
  `#disallowed-include` 标记且不读取内容。
- **reload 权限不足时未本地化**:后端写的是 `details.reason`,前端查的是 `details.kind`,
  导致权限失败只能看到英文 stderr。已对齐(仍兼容旧字段)。
- **日志级别参数不可用**:`logs.tail` / `logs.search` 的 `level` 未在清单声明,Agent 按
  技能文档调用会直接被 `INVALID_ARGS` 拒绝;`logs.search` 解析了 `level` 却未生效。现在
  清单已声明,搜索会按级别过滤,日志页新增「日志级别」选择器。
- 证书列表的 `skipped` 标记缺失,被跳过的证书在界面上被误显示为「不可读」。
- 配置页现在展示后端返回的 `warnings` 与 `error`(include 未解析、解析降级、截断原因等)。

## Changed

- 访问统计对 Top IP / Top 路径的基数设上限(超出部分归入 `(other)`),避免唯一 URL 极多的
  日志让插件进程内存无界增长。
- 版本号 1.0.9;README 打包与安装示例中的压缩包名同步到 1.0.9。

## Verified

- `go build ./plugins/nginx-inspector/...`、`go vet`、`go test ./plugins/nginx-inspector/...` 通过。
- 新增回归测试:统计跟随配置 `access_log`、窗口超出帧上限仍全量分析、小窗口精确截取、
  `access_log off` 的说明、include 白名单拒绝、`logs.search` 级别过滤。
