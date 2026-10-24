# GoNavi Plugin: Nginx Inspector

GoNavi 插件:通过 SSH 对远程 nginx 做**只读诊断**——配置结构、`nginx -t` 校验、
错误/访问日志、访问统计、TLS 证书到期检查、上游健康探测。唯一的写命令是
`service.reload`,且强制先通过 `nginx -t` 并要求显式确认。

插件 ID:`com.gonavi.nginx-inspector`(Apache-2.0)。

## 功能

| 模块 | 说明 |
|---|---|
| 连接管理 | SSH 配置(密码/私钥)保存在本机插件数据目录,界面永不回显密码;首次连接记录服务器指纹(TOFU),指纹变化即拒绝 |
| 概览 | `nginx -V` 解析:版本、二进制、prefix、主配置、日志路径、systemd/容器环境 |
| 配置 | 主配置 + include 递归展开(环形检测、64 文件/8 层上限),server/upstream/location 摘要,按文件查看内容 |
| 校验 | 远程执行 `nginx -t`,逐条解析报错的 file:line |
| 服务 | 状态(systemd/pid/uptime);reload 带 `nginx -t` 门禁 + 显式确认 |
| 日志 | error/access tail(≤1000 行)+ 本地正则过滤 + 远程 grep 搜索(≤50 MB 窗口) |
| 访问统计 | combined 格式聚合:状态码/分类、方法、Top IP、Top 路径(含平均上游耗时)、时间窗、吞吐 |
| 证书 | 解析配置引用的 `ssl_certificate`(X.509),标记 ≤30 天到期/已过期/不可读 |
| 上游 | 从远程主机对每个 upstream server 做 TCP 探测(并发 ≤8,3 秒超时) |
| Agent | `ai.tools` 将命令暴露给 GoNavi Agent;`ai.skills` 提供 `nginx-triage` 技能文档 |

## 安装

GoNavi → 标题栏插件按钮 → 插件市场/本地安装:选择发行 ZIP,核对清单与权限后信任安装,
启用后在插件详情页点「打开窗口」。

或使用 CLI(先关闭桌面端插件运行时):

```sh
gonavi plugin --root <data-root>/plugins inspect gonavi-plugin-nginx-1.0.0.zip
gonavi plugin --root <data-root>/plugins install --digest <sha256> --trust gonavi-plugin-nginx-1.0.0.zip
gonavi plugin --root <data-root>/plugins enable com.gonavi.nginx-inspector
```

## 权限

| 权限 | 用途 |
|---|---|
| `ai.tools` | 将命令暴露给 Agent(写命令仍需逐次审批) |
| `ai.skills` | 提供 nginx-triage 技能文档(显式加载) |
| `host.log` | 写有界诊断日志 |

插件进程拥有当前用户的全部 OS 权限;进程隔离与限额是可靠性边界,不是安全沙箱。
请在信任前审查源码与权限清单。

## 从源码构建

本插件位于 GoNavi 官方插件源码仓 `github.com/Syngnat/GoNavi-Plugin-Market`
(本目录即 `plugins/nginx-inspector/`,官方插件与市场站点同仓)。依赖:Go ≥ 1.24;
SDK 以伪版本引用 `github.com/Syngnat/GoNavi/sdk/go`,无其他第三方依赖
(仅 `golang.org/x/crypto/ssh`)。

在**仓库根**执行:

```sh
go test ./plugins/nginx-inspector/...
go run ./cmd/buildtool -manifest plugins/nginx-inspector/gonavi.plugin.json -output plugin-dist/nginx-inspector -zip plugin-dist/gonavi-plugin-nginx-inspector-1.0.0.zip
```

`buildtool` 交叉编译 5 个平台(windows/amd64、linux/amd64、linux/arm64、
darwin/amd64、darwin/arm64),复制 `ui/`、`skills/`、README、LICENSE,并生成
带 `backend.platforms` 的最终 manifest。产物符合宿主静态检查(256 MiB/包、
64 MiB/文件上限)。发布 tag 约定 `nginx-inspector-v<semver>`。

## 目录结构

```
main.go                  # stdio JSON-RPC 装配
internal/commands/       # 15 个命令的实现(hostinfo/config/service/logs/stats/certs/upstreams/connections)
internal/sshx/           # SSH 连接池、TOFU 指纹、有界命令执行、SFTP-free 文件读取
internal/ngxconf/        # nginx 配置词法/递归下降解析器
internal/store/          # 连接配置持久化(0600,掩码回传,读 GONAVI_PLUGIN_DATA)
internal/rpcerr/         # 稳定错误码 → plugin.Error
ui/                      # 独立插件窗口(原生 HTML/JS,无外部资源)
skills/nginx-triage/     # Agent 技能文档
```

## 设计边界(有意不做)

- **不写配置文件**:GUI 配置生成/写回是另一个量级的工程与生产风险,本插件定位诊断。
- **不读私钥内容回传**;密码仅在保存与拨号时存在于插件进程内存。
- 日志读取全部有字节/行数上限,响应恒小于 1 MiB 协议帧。
