# 華龍算力 1.2.2 发布清单

## 变更

- 修复账号面板的并发刷新竞态：窗口打开时 `account.status` 与 `account.keys`
  会并发续期同一个 refresh token，服务端轮换后必有一方被拒，失败方调用
  `clearRecord()` 把另一方刚写入的新凭据删除，表现为「余额能显示、API Key
  列表报尚未登录」，随后 `account.json` 丢失。
- 续期改为串行执行：`ensureFresh` 持锁后重新读取磁盘，若另一命令已完成轮换
  且新令牌仍新鲜，直接采用其记录，不再重复消耗旧 refresh token。
- 401 清理改为条件删除：新增 `clearRecordIf(token)`，仅当磁盘记录仍是失败时
  使用的那份才清除；`logout` 保持无条件清理。
- 新增并发回归测试：两个并发 `status`、并发 `status` + `keys`、过期 401 不
  删除已轮换记录。移除互斥后测试可复现原故障。

## 产物

- ZIP：`plugin-dist/gonavi-plugin-hualong-ai-1.2.2.zip`
- SHA-256：`b1970eb6d4dd8dfd317fef060fdf7988c1f44cc780d7cd13430b81e1a20c790b`
- 传输大小：15,875,274 bytes
- 规范内容摘要：`44ee8700dfb3e282426a0c13c8c2db1a801e10ca96b2adf10f7922bda94e896a`
- 展开大小：36,254,323 bytes
- 平台：Windows amd64、Linux amd64/arm64、macOS amd64/arm64
- 宿主最低版本：1.1.5（不变）

## 发布步骤

1. 打 tag 并推送：

   ```sh
   git tag hualong-ai-provider-v1.2.2
   git push origin hualong-ai-provider-v1.2.2
   ```

2. 用该 tag 建 GitHub Release，上传上面的 ZIP；资产名保持
   `gonavi-plugin-hualong-ai-1.2.2.zip`。

3. 下载 Release 资产复核 SHA-256，再更新 `public/catalog.json` 的对应条目。

4. 服务端目录：

   ```sh
   deploy/publish-official-plugin.sh --plugin hualong-ai-provider \
       --tag hualong-ai-provider-v1.2.2 --reviewer <admin-github-id>
   ```

   发布后核对：`GET /api/v1/plugins?q=hualong` 返回 1.2.2 且 `official: true`，
   工件直链 `/api/v1/artifacts/<sha256>` 下载后 SHA-256 与 Release 一致。

## 注意

- 打包必须使用市场仓根 `cmd/buildtool`。清单声明了 `contributes.aiProviders[].icon`
  时包内必须带该文件，否则宿主 `InspectPackage` 会以「declared entry is not a regular
  package file」拒绝安装。
- 不提交 `plugin-dist/`、ZIP 或运行态数据。
- 宿主侧模型区（名称与上下文档位）依赖 models.dev 公共目录，属宿主能力，与本插件版本无关。

## 历史版本

### 1.2.0

- `account.listModels` 在原有 `id` 与 `keyIds` 之外，一并返回网关给的模型名称与上下文窗口。
- 网关字段兼容：`name` / `display_name` 与 `context_length` / `context_window` / `max_context_length`；
  名称超 120 字节、窗口非正或超 2000 万一律视为没给。
- 同一模型被多把 Key 收录时：名称取首个非空、窗口取最大值，`keyIds` 语义不变。
- 宿主缺失元数据时用 models.dev 公共目录补齐，插件不内建模型表。
