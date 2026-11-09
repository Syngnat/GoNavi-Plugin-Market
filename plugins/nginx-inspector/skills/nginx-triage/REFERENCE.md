# Nginx Inspector command reference

All commands are invoked through the plugin `com.gonavi.nginx-inspector`.
Every failure carries a stable `code` in the error data (for example
`SSH_AUTH_FAILED`, `HOSTKEY_CHANGED`, `PATH_NOT_ALLOWED`, `NGINX_TEST_FAILED`);
translate codes instead of echoing raw messages.

| Command | Effect | Key arguments | Returns |
| --- | --- | --- | --- |
| `connections.list` | read | — | masked profiles (no secrets) |
| `connections.save` | **write** | `name, host, port, username, authType(password\|key), password?, keyPath?, keyPassphrase?, id?, fingerprint?` | saved masked profile; omit `fingerprint` to keep TOFU, send it only to pin deliberately |
| `connections.delete` | **write** | `id` | deleted id |
| `connections.test` | read | `id` | `ok, fingerprint, latencyMs` |
| `host.info` | read | `connectionId, refresh?` | `found, binary, version, prefix, confPath, errorLogPath, httpLogPath, pidPath, systemd, docker, container, kernel, hostname, discoveredAt` |
| `config.tree` | read | `connectionId` | `files[]` (parsed directives, main config first), `servers[]`, `upstreams[]`, `includedCount`, `truncated` |
| `config.get` | read | `connectionId, path` | `content` (≤512 KiB), `sizeBytes`, `truncated`, `lineCount` |
| `config.validate` | read | `connectionId` | `ok, messages[{level,message,file,line}]` from `nginx -t` |
| `service.status` | read | `connectionId` | `state(active\|inactive\|failed\|unknown), unit, pid, uptime, systemd, docker, pidFile, hostname` |
| `service.reload` | **write** | `connectionId, confirm:true, method?` | `status(applied\|unknown), method, unit, fallbackUsed, startedAt, errorLogPath, errorLogChecked, errorLogMatches, output`; refuses when `nginx -t` fails; permission failures return `RELOAD_BLOCKED` with `reason:permission` |
| `logs.tail` | read | `connectionId, kind(error\|access)?, path?, lines?(≤1000), filter?(RE2), level?` | `kind, path, source, note, lines[], matched, scanned, truncated, sizeBytes, totalLines, windowStartLine, level, filter` |
| `logs.search` | read | `connectionId, pattern(POSIX ERE ≤256), kind?, path?, bytes?(≤50 MiB), maxMatches?(≤500)` | `matches[{windowLine,line,text}], more, windowStartOffset, windowBytes, fileSizeBytes, source, note` |
| `access.stats` | read | `connectionId, path?, bytes?(≤100 MiB)` | `statuses, classes, methods, topIPs, topPaths(with avgMs), analyzedBytes, requestedBytes, requestTime, upstreamTime, gzipUnsupported, timeFrom/To` |
| `certs.list` | read | `connectionId` | `certificates[{path, file, readable, reason, parseError, parseErrorText, skipped, commonName, domains, issuer, notBefore, notAfter, daysLeft, expired, expiringSoon}]`, counts `expiringSoon/expired/unreadable/parseErrors/skipped` |
| `upstreams.check` | read | `connectionId` | `upstreams[{name, servers[{address, alive, latencyMs, disabled, backup, skipReason}]}], alive, down, skipped, backup` |

## Bounds to respect

- `logs.tail`: ≤1000 lines, output capped ~2 MiB per call; `truncated:true` means the
  returned prefix is not the true tail. `source` explains where the path came from
  (`config|binary-default|explicit`), `note` explains exceptions such as
  `access_log off` or syslog, and `totalLines`/`windowStartLine` locate the window.
- `logs.search`: regex is POSIX ERE evaluated remotely; `\d` is invalid, use `[0-9]`.
  `windowLine` is the line number inside the searched window (the legacy `line`
  alias carries the same value), `windowStartOffset` is that window's byte offset.
- `access.stats`: window ≤100 MB; parsing is the plugin's own combined-format
  reader. `analyzedBytes` is what was really parsed, `requestedBytes` is the
  requested window; `windowBytes` carries the same value as `analyzedBytes`.
  `gzipUnsupported:true` means the log is gzip-compressed and was not parsed.
- `config.get`/`config.tree`: file reads are capped (512 KiB per file, 64 files,
  include depth 8); always surface `truncated`.
- Any invocation may be canceled by the host deadline; treat lost responses as
  **unknown outcome**, especially for `service.reload`.

## Safety notes

- `connections.save`, `connections.delete` and `service.reload` are the only write
  commands; every Agent tool call still requires user approval
  (`side_effect_unknown`). Never call them proactively.
- First connection records the server's SSH host key fingerprint (TOFU). A later
  mismatch surfaces as `HOSTKEY_CHANGED` and must be resolved by a human. Omit
  `fingerprint` on `connections.save` to keep the recorded value; sending one pins
  the key explicitly.
- File paths are restricted to the nginx prefix, log and certificate directories;
  anything else fails with `PATH_NOT_ALLOWED`.
- The plugin cannot read host credentials and cannot edit remote files by design.
