# MongoShell 1.0.4

传输大小：38,134,116 bytes；SHA-256：`4c4561e35d7fcf9c9c5a7c6376fc2a4028598437fc4b0e0c438f8839802a0922`。

- 只保留工作台入口：移除独立窗口（`window.entry` 与窗口动作），入口统一为 MongoDB 连接右键 → MongoShell。
- 工作台页按同一套设计令牌重写：扁平白底、发丝线、8px 圆角、等宽数据列，深浅主题跟随宿主。
- 工作台补齐交互：Max rows 可调、Clear、结果元信息、截断提示、只读/环境徽标与底栏状态。
- 六语言词典覆盖工作台全部文案。

# MongoShell 1.0.3

- 界面按「Precision Database Canvas」重做：扁平白底、1px 发丝线、8px 圆角、等宽数据列，窗口与工作台页同一套令牌。
- 结果集边界：达到 Max rows 显示截断提示条，零行时保留表头并给出「未命中」说明，无列文档单独提示。
- 引导态：不是从连接右键打开时隐藏工具栏与工作区，改为居中说明入口。
- 完整状态覆盖：连接中 / 执行中（状态胶囊脉冲、Run 转 spinner、旧结果保留并淡化）/ 失败（红色状态 + 错误条 + 虚线空态）/ 只读 / 断开，Output 抽屉可折叠。
- 界面接入宿主语言：读取宿主注入的 `window.gonaviHostPrefs.locale`，支持 zh-CN / zh-TW / en-US / ja-JP / de-DE / ru-RU，缺省回退英文。
- 静态文案（工具栏、面板标题、页脚）走 `data-i18n` 标记统一替换；运行状态与错误横幅改走同一词典。
- `invalid_context`、`empty_script`、`bridge_unavailable` 等自抛错误改为本地化说明，不再直接把英文抛给用户。
- 未从连接右键打开时会提示改用连接菜单，而不是只显示「launch context did not include…」。
- 五平台 ZIP：`gonavi-plugin-mongo-shell-1.0.3.zip`；最低宿主版本仍为 1.2.0。
- 传输大小：38,149,346 bytes；SHA-256：`54b547a42cc9506cf7ca28ce195230fb233b8162853f1ecf26146e35e9ec93ed`。
- 回归：窗口/工作台页脚本测试（`ui/page.test.mjs`、`ui/window.test.mjs`）与后端 `go test ./...` 全绿。

# MongoShell 1.0.2

- 修复多窗口／工作台标签页共享连接：每次 connect 返回独立 sessionId，run／ping／disconnect 按会话路由，保留各自只读策略。
- 修复工作台页未解包后端响应：连接或脚本失败不再误报 ready／done，数据库列表与只读状态正常展示。
- 增加会话上限、关闭取消、迟到连接回收与空闲会话回收；失败连接清理受 5 秒预算约束。
- 窗口、工作台页、清单和后端必须配套更新；旧调用方需保存并传回 connect 返回的 sessionId。
- 回归包含双连接隔离、只读保护、会话关闭／容量／清理，以及真实页面和窗口脚本响应处理。
- 五平台 ZIP：`gonavi-plugin-mongo-shell-1.0.2.zip`；最低宿主版本仍为 1.2.0，开发版宿主按现有兼容规则安装。
- 传输大小：38,136,129 bytes；展开大小：96,168,442 bytes。
- SHA-256：`b6b7411b3122aa6d84ab682de42cd8ec3b7eaea038252096db8c2515d5db48e3`。
- 规范内容摘要：`83ca638db5a53c1be7d7f5d627b46de96f68b43a4bf42d5374dd50363c6bddeb`。
- 使用市场仓统一 `cmd/buildtool` 打包，回归验证清单贡献字段不会丢失。

# MongoShell 1.0.1

- 新增工作台入口：连接/库/集合右键 **MongoShell (Workbench)** 打开工作台标签页。
- 工作台页通过 `plugin.invoke` 调用本插件已声明的 `connect`/`run`；宿主注入连接配置，页面不接触凭据。
- 保留独立窗口入口 **MongoShell**。
- 五平台 ZIP：`gonavi-plugin-mongo-shell-1.0.1.zip`
- 大小：38,093,767 bytes
- SHA-256：`19152a69d4f1fe543c3e67dd6d6b6b0aabf0e49598ee0894c22d363f6491155c`
- `minHostVersion` 仍为 1.2.0。

# MongoShell 1.0.0 发布清单

## 产物

- ZIP：`plugin-dist/gonavi-plugin-mongo-shell-1.0.0.zip`
- SHA-256：`f94dfd0f0dae303712f1482f1c1fb5ccde6e02e8b92b1d01fec983b745359413`
- 传输大小：38,112,128 bytes
- 规范内容摘要：`ece4bbafdafa64ae5db2af655cb5dc47e9f17b1d456cafc79c53e2b3c77a127e`
- 展开大小：96,074,598 bytes
- 平台：Windows amd64、Linux amd64/arm64、macOS amd64/arm64
- 宿主最低版本：1.2.0（本仓 `wails.json` 与 `version/dev-version.txt` 已对齐）

## 发布步骤（需在确认后执行）

1. 在 `GoNavi-Plugin-Market` 建立并推送 tag：

   ```sh
   git tag mongo-shell-v1.0.0
   git push origin mongo-shell-v1.0.0
   ```

2. 用该 tag 创建 GitHub Release，上传上面的 ZIP；Release 资产名保持
   `gonavi-plugin-mongo-shell-1.0.0.zip`。

3. 确认资产可下载并复核 SHA-256 后，把下面的条目追加到
   `public/catalog.json` 的 `items`，再构建/部署站点。**不要在 Release 可下载前写入死链。**

```json
{
  "id": "com.gonavi.mongo-shell",
  "name": "MongoShell",
  "description": "在独立窗口中连接已保存的 MongoDB，浏览数据库并用 goja 执行有界的 JavaScript Shell 脚本：find、aggregate、countDocuments、insert、update、delete 与 runCommand；只读连接会拒绝写操作。",
  "author": "GoNavi Project",
  "publisherId": "gonavi",
  "official": true,
  "version": "1.0.0",
  "category": "数据库",
  "minHostVersion": "1.2.0",
  "platforms": ["windows", "macos", "linux"],
  "updatedAt": "2026-10-27T00:00:00Z",
  "icon": "database",
  "installSpec": "github:Syngnat/GoNavi-Plugin-Market@mongo-shell-v1.0.0",
  "repoUrl": "https://github.com/Syngnat/GoNavi-Plugin-Market",
  "contentDigest": "ece4bbafdafa64ae5db2af655cb5dc47e9f17b1d456cafc79c53e2b3c77a127e",
  "license": "Apache-2.0",
  "artifact": {
    "url": "https://github.com/Syngnat/GoNavi-Plugin-Market/releases/download/mongo-shell-v1.0.0/gonavi-plugin-mongo-shell-1.0.0.zip",
    "sha256": "f94dfd0f0dae303712f1482f1c1fb5ccde6e02e8b92b1d01fec983b745359413",
    "sizeBytes": 38112128,
    "platform": "all",
    "version": "1.0.0"
  },
  "sourceId": "plugin-market-site",
  "features": [
    "连接/库/集合右键直接打开 MongoShell，无需进入插件管理详情。",
    "独立窗口，goja JavaScript Shell，支持 db/collection 查询、聚合与写入。",
    "只读连接拒绝写入；行数上限 200/2000，命令超时 30 秒。",
    "按平台分发原生 backend；窗口从宿主加载统一主题与设计令牌。"
  ],
  "permissions": [
    {"name": "读取连接凭据", "key": "host.connections.credentials", "scope": "仅插件动作启动窗口", "granted": true},
    {"name": "写诊断日志", "key": "host.log", "scope": "诊断", "granted": true},
    {"name": "提供 AI 技能", "key": "ai.skills", "scope": "技能", "granted": true}
  ]
}
```

## 注意

- 历史 1.0.0/1.0.1 使用插件内的 `cmd/pack` 保留右键声明；1.0.2 起使用市场仓根
  `cmd/buildtool`，统一工具仅替换后端平台条目并保留全部原始清单贡献。
- 不提交 `plugin-dist/`、ZIP 或运行态数据。
- 发布前需在真实宿主 1.2.0 上验证：安装启用 → 右键 MongoDB 连接/库/集合 → 打开窗口 → connect/run。
