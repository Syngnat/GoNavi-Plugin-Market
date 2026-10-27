# 写与 DDL 能力边界

> 状态：已实现受控版本。插件必须声明 `host.schema.write`，先调用 `schema.preview` 生成一次性计划，
> 再调用 `schema.apply` 触发宿主原生确认对话框。当前**只执行 DDL**，不开放 DML、事务或脚本。

## 1. 为什么不直接开放 DBQuery

`host.query.read` 的语义是只读。把写语句塞进 `DBQuery` 会绕过只读连接、SQL 审计、结果预算和宿主确认。
因此写与 DDL 使用独立权限、独立能力，不能复用查询入口。

## 2. 权限

| 权限 | 能力 | 说明 |
| --- | --- | --- |
| `host.schema.write` | `schema.preview`, `schema.apply` | 仅对已保存连接执行经宿主确认的 DDL 计划。 |

`host.schema.write` 与 `host.query.read` 分开授予。只读连接、`RestrictStructureEdit` 或
`RestrictScriptExecution` 会在 preview 与 apply 两处被拒绝，即使插件声明了权限。

## 3. 两步协议

### 3.1 `schema.preview`

输入：

```json
{
  "connectionId": "saved-connection-id",
  "database": "app",
  "statements": ["ALTER TABLE users ADD COLUMN nickname VARCHAR(64)"]
}
```

宿主返回：

```json
{
  "planId": "opaque-one-use-id",
  "database": "app",
  "statements": ["ALTER TABLE users ADD COLUMN nickname VARCHAR(64)"],
  "expiresAt": "2026-10-27T10:00:00Z",
  "requiresHostConfirmation": true
}
```

宿主自行拆分/分类语句，不信任插件提供的类型。计划绑定连接、数据库、规范化语句和过期时间，一次性使用；
默认 5 分钟过期，最多 16 个待确认计划，单次最多 8 条语句，每条最大 32 KiB。

### 3.2 `schema.apply`

输入：

```json
{ "planId": "opaque-one-use-id" }
```

`planId` 在读取时立即消费；过期、已使用或不存在都返回 `plan_expired`。随后宿主重新解析保存连接并检查只读/保护策略，
再弹出宿主原生确认对话框（Cancel / Apply）。用户取消、对话框不可用或调用处于 headless/web runtime 时失败关闭：
返回 `confirmation_denied` / `confirmation_unavailable`，不执行任何语句。

确认后逐条执行，每条语句单独写 SQL 审计。返回：

```json
{
  "applied": 1,
  "failed": 0,
  "statements": [
    { "statement": "ALTER TABLE ...", "success": true }
  ]
}
```

DDL 在部分数据库会隐式提交，失败不会自动回滚或重试；结果中的 `applied` / `failed` 必须展示给用户。

## 4. 允许与拒绝

当前只允许首关键字与目标同时匹配：

- `CREATE TABLE` / `CREATE INDEX` / `CREATE VIEW`
- `ALTER TABLE` / `ALTER INDEX` / `ALTER VIEW`
- `DROP TABLE` / `DROP INDEX` / `DROP VIEW`

当前拒绝：

- 所有 DML（`INSERT` / `UPDATE` / `DELETE`）、`SELECT` 和事务控制；
- `TRUNCATE`、`DROP DATABASE`、`CREATE/ALTER DATABASE`、schema 级 DDL；
- `CREATE UNIQUE INDEX`（第二 token 不是 `INDEX`）、存储过程/函数/触发器、动态 SQL；
- 多语句输入（每条 `statements` 必须恰好一条语句）、无法分类或无法确定目标的语句。

这是一份刻意保守的 v1 白名单；扩展前必须先补测试、权限说明和 UI 风险提示。

## 5. 审计与幂等

- 每条 apply 语句写 SQL 审计，来源为 `plugin_schema_write`；
- 日志不记录凭据、完整连接串或结果行内容；
- `planId` 一次性消费，执行失败不自动重试；
- 结果受能力层 64 KiB 上限约束；大结果应分批/分语句。

## 6. 已知限制

- 没有跨语句事务保证，部分失败可能留下已执行的 DDL；
- 原生确认框只展示语句数量与数据库名，完整 SQL 由插件在预览 UI 中展示；
- 尚未提供 SDK 专用类型；插件按能力 envelope 自行建模；
- 生产库没有额外的二次审批层，宿主原生确认是唯一强制门。

插件在真实连接上使用前，应先在测试库验证方言、权限和回滚策略。
