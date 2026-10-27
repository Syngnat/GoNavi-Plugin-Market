# Host capabilities

GoNavi registers host capabilities explicitly. The page bridge (`PluginPageCall`) and the native plugin backend (`host.capability`) share one registry; the channel decides whether a method is reachable. A plugin still needs the exact permission shown below, and the host enforces read-only connections, argument/result bounds and a per-call timeout.

`app.info.capabilitiesVersion` identifies the catalogue generation (currently `2`). `app.capabilities` (no permission) returns the capabilities the running host actually supports, whether the calling plugin holds each permission, the channels and the mutating/confirmation flags. Plugins should use it to degrade gracefully instead of assuming a host version.

## Initial catalogue (capability API v2, first slice)

| Capability | Permission | Channels | Mutating |
|---|---|---|---|
| `app.info` | — | page, window, backend | no |
| `app.capabilities` | — | page, window, backend | no |
| `connections.list` | `host.connections.read` | page, window, backend | no |
| `connections.capabilities` | `host.connections.read` | page, window, backend | no |
| `connections.health` | `host.connections.read` | page, window, backend | no |
| `connections.test` | `host.connections.read` | page, window, backend | no |
| `metadata.databases` | `host.metadata.read` | page, window, backend | no |
| `metadata.tables` | `host.metadata.read` | page, window, backend | no |
| `metadata.columns` | `host.metadata.read` | page, window, backend | no |
| `metadata.views` | `host.metadata.read` | page, window, backend | no |
| `metadata.objects` | `host.metadata.read` | page, window, backend | no |
| `metadata.allColumns` | `host.metadata.read` | page, window, backend | no |
| `metadata.indexes` | `host.metadata.read` | page, window, backend | no |
| `metadata.foreignKeys` | `host.metadata.read` | page, window, backend | no |
| `metadata.triggers` | `host.metadata.read` | page, window, backend | no |
| `metadata.createStatement` | `host.metadata.read` | page, window, backend | no |
| `metadata.tableInfo` | `host.metadata.read` | page, window, backend | no |
| `metadata.serverVersion` | `host.metadata.read` | page, window, backend | no |
| `metadata.tableExists` | `host.metadata.read` | page, window, backend | no |
| `metadata.databaseForeignKeys` | `host.metadata.read` | page, window, backend | no |
| `metadata.charsets` | `host.metadata.read` | page, window, backend | no |
| `metadata.collations` | `host.metadata.read` | page, window, backend | no |
| `sync.jobs` | `host.metadata.read` | page, window, backend | no |
| `sync.job` | `host.metadata.read` | page, window, backend | no |
| `nacos.services` | `host.metadata.read` | page, window, backend | no |
| `nacos.service` | `host.metadata.read` | page, window, backend | no |
| `nacos.instances` | `host.metadata.read` | page, window, backend | no |
| `nacos.instance` | `host.metadata.read` | page, window, backend | no |
| `jvm.capabilities` | `host.query.read` | page, window, backend | no |
| `jvm.resources` | `host.query.read` | page, window, backend | no |
| `jvm.value` | `host.query.read` | page, window, backend | no |
| `jvm.audit` | `host.query.read` | page, window, backend | no |
| `mongo.discoverMembers` | `host.metadata.read` | page, window, backend | no |
| `metadata.savedQueries` | `host.metadata.read` | page, window, backend | no |
| `sessions.list` | `host.query.read` | page, window, backend | no |
| `sessions.databases` | `host.query.read` | page, window, backend | no |
| `sessions.longTransactions` | `host.query.read` | page, window, backend | no |
| `sessions.lockWaits` | `host.query.read` | page, window, backend | no |
| `sessions.capabilities` | `host.query.read` | page, window, backend | no |
| `redis.scan` | `host.query.read` | page, window, backend | no |
| `redis.value` | `host.query.read` | page, window, backend | no |
| `redis.serverInfo` | `host.query.read` | page, window, backend | no |
| `redis.databases` | `host.query.read` | page, window, backend | no |
| `query.explain` | `host.query.read` | page, window, backend | no |
| `query.read` | `host.query.read` | page, window, backend | no |
| `ui.notify` | — | page | no |

Connections are always resolved by saved connection id on the host. A page or backend never receives usernames, passwords, SSH/proxy credentials, DSNs or arbitrary host paths. Metadata and read queries return bounded results; data that exceeds the result budget must be split by the caller.

## Backend call

`host.capability` params:

```json
{"method":"metadata.tables","arguments":{"connectionId":"conn-1","database":"app"}}
```

The host returns a normal JSON result envelope:

```json
{"ok":true,"data":[{"name":"users","type":"table"}]}
{"ok":false,"error":{"code":"permission_denied"}}
```

Stable failure codes include `invalid_arguments`, `unknown_method`, `permission_denied`, `not_running`, `result_too_large` and `internal`. The SDK exposes `plugin.Host.CallCapability(ctx, method, arguments)`; it does not expose arbitrary Wails bindings.

## Page call

The page bridge uses the same capability names with `window.gonavi.call(method, args)`. Page calls are limited to 64 KiB of arguments, 256 KiB of results and 30 seconds per call in this first slice. The host validates the declared page, lifecycle, permission, arguments and result size in that order. See [pages](<pages.md>).

## Evolution

The catalogue is versioned with the host and grows in reviewable slices. `app.capabilities` is the compatibility surface: a plugin must not assume a capability exists merely because a newer document lists it. Future slices add write, DDL, export/import, sync, backup, Redis, MongoDB, AI, JVM, driver, user/session, Nacos/XxlJob, plugin/market, file and UI capabilities with their own permissions and audit rules.
