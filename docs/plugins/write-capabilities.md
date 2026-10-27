# 写与 DDL 能力边界（设计稿）

> 状态：设计稿，未实现、未注册任何写能力。当前插件只能通过 `query.read` 与
> `metadata.*` 读取数据；`metadata.createStatement` 只返回已有对象的 DDL，不执行 DDL。
> 本文件定义未来 `schema.preview` / `schema.apply` 的宿主边界，供实现与评审使用。

## 1. 为什么不直接开放 DBQuery

`host.query.read` 的语义是只读。把写语句塞进 `DBQuery` 会绕过：

- 只读连接与生产库确认；
- SQL 审计和数据源级策略；
- 多语句、事务和结果大小限制；
- 宿主对破坏性语句的二次确认。

因此写与 DDL 必须是独立权限、独立能力，不能复用查询入口。

## 2. 提议的权限

| 权限 | 能力 | 说明 |
| --- | --- | --- |
| `host.schema.write` | `schema.preview`, `schema.apply` | 仅对已保存连接执行经过宿主确认的 DDL/DML 计划。 |

`host.schema.write` 与 `host.query.read` 分开授予；只读连接即使声明该权限也必须在宿主侧拒绝。
插件不能自行弹确认框代替宿主确认，也不能把确认 token 传给其他插件或窗口。

## 3. 两步协议

### 3.1 `schema.preview`

输入：

```json
{
  "connectionId": "saved-connection-id",
  "database": "app",
  "statements": ["ALTER TABLE ..."],
  "reason": "short human-readable reason"
}
```

宿主返回：

```json
{
  "planId": "random-one-use-id",
  "normalizedSql": ["ALTER TABLE ..."],
  "statementKinds": ["alter_table"],
  "risk": "ddl",
  "requiresConfirmation": true,
  "expiresAt": "2026-10-27T10:00:00Z"
}
```

宿主必须重新解析语句，而不是信任插件给出的 `statementKinds` 或 `risk`。`planId` 绑定连接、数据库、
规范化 SQL 摘要、权限授予和过期时间，一次性使用。

### 3.2 `schema.apply`

输入：

```json
{
  "planId": "random-one-use-id",
  "confirmationToken": "host-issued-token"
}
```

`confirmationToken` 只能由宿主确认 UI 在用户明确点击后签发，绑定 `planId`，不得由页面/插件生成。
宿主执行前再次检查只读连接、生产库风险、连接配置与权限；执行后返回受影响对象、行数和审计 id。
若计划已过期、已被使用、连接配置变化或语句被篡改，返回 `stale` / `invalid_request`，不执行任何语句。

## 4. 允许与拒绝

首版只允许：

- `CREATE` / `ALTER` / `DROP` 表、索引、视图、触发器（按方言白名单）；
- 单条语句或显式事务内的有限多条语句；
- 插件声明、宿主确认后的一次性执行。

首版拒绝：

- `DROP DATABASE`、`TRUNCATE` 全库、无 `WHERE` 的批量 `UPDATE` / `DELETE`；
- 动态 SQL、存储过程/函数体、跨库 DDL；
- 文件导入导出、`LOAD DATA`、`COPY`、`xp_cmdshell` 等宿主未建模的副作用；
- 任何无法可靠解析或无法确定目标对象的语句。

## 5. 审计与幂等

- 每次 preview/apply 均写 SQL 审计，记录插件 id、连接 id、数据库、规范化 SQL 摘要、结果和耗时；
- 日志不得记录凭据、完整连接串或结果行内容；
- `planId` 一次性消费；执行失败不自动重试，避免重复 DDL；
- 结果受宿主统一大小限制，超出时截断并标记 `truncated`。

## 6. 实现顺序

1. 在 `internal/app` 增加独立的 `host.schema.write` 权限与 `schema.preview`；
2. 增加宿主确认 UI 与 token 存储（短期、单次、绑定摘要）；
3. 增加 `schema.apply`，先只支持 SQL 方言白名单中的单条 DDL；
4. 用测试覆盖只读连接拒绝、token 重放、语句篡改、过期、生产库确认和审计；
5. 更新能力清单、权限参考与 SDK 类型后，才允许插件声明该权限。

在以上完成前，插件市场中的插件不应声明 `host.schema.write`，宿主也不应注册 `schema.apply`。
