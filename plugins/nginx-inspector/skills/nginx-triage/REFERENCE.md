# Nginx Inspector command reference

All commands are invoked through the plugin `com.gonavi.nginx-inspector`.
Every failure carries a stable `code` in the error data (for example
`SSH_AUTH_FAILED`, `HOSTKEY_CHANGED`, `NGINX_TEST_FAILED`); translate codes
instead of echoing raw messages.

| Command | Effect | Key arguments | Returns |
| --- | --- | --- | --- |
| `connections.list` | read | — | masked profiles (no secrets) |
| `connections.save` | write | `name, host, port, username, authType(password\|key), password?, keyPath?, keyPassphrase?, id?` | saved masked profile |
| `connections.delete` | write | `id` | deleted id |
| `connections.test` | read | `id` | `ok, fingerprint, latencyMs` |
| `host.info` | read | `connectionId, refresh?` | `found, binary, version, prefix, confPath, errorLogPath, httpLogPath, pidPath, systemd, docker, kernel, hostname` |
| `config.tree` | read | `connectionId` | `files[]` (parsed directives), `servers[]`, `upstreams[]`, `includedCount`, `truncated` |
| `config.get` | read | `connectionId, path` | `content` (≤512 KiB), `sizeBytes`, `truncated`, `lineCount` |
| `config.validate` | read | `connectionId` | `ok, messages[{level,message,file,line}]` from `nginx -t` |
| `service.status` | read | `connectionId` | `state(active\|inactive\|unknown), pid, uptime, systemd, docker` |
| `service.reload` | **write** | `connectionId, confirm:true, method?` | `method(systemd\|signal), output`; refuses when `nginx -t` fails |
| `logs.tail` | read | `connectionId, kind(error\|access)?, path?, lines?(≤1000), filter?(RE2)` | `lines[], matched, scanned, sizeBytes` |
| `logs.search` | read | `connectionId, pattern(POSIX ERE ≤256), kind?, path?, bytes?(≤50 MiB), maxMatches?(≤500)` | `matches[{line,text}], more` |
| `access.stats` | read | `connectionId, path?, bytes?(≤100 MiB)` | `statuses, classes, methods, topIPs, topPaths(with avgMs), bytesTotal, timeFrom/To, avgUpstreamMs, maxUpstreamMs` |
| `certs.list` | read | `connectionId` | `certificates[{path, readable, domains, issuer, notAfter, daysLeft, expired, expiringSoon}]`, counts |
| `upstreams.check` | read | `connectionId` | per-member `alive, latencyMs` probed from the remote host |

## Bounds to respect

- `logs.tail`: ≤1000 lines, output capped ~2 MiB per call.
- `logs.search`: regex is POSIX ERE evaluated remotely; `\d` is invalid, use `[0-9]`.
- `access.stats`: window ≤100 MB; parsing is the plugin's own combined-format reader.
- Any invocation may be canceled by the host deadline; treat lost responses as
  **unknown outcome**, especially for `service.reload`.

## Safety notes

- `connections.save`/`delete` and `service.reload` are the only write commands;
  every Agent tool call still requires user approval (`side_effect_unknown`).
- First connection records the server's SSH host key fingerprint (TOFU). A later
  mismatch surfaces as `HOSTKEY_CHANGED` and must be resolved by a human.
- The plugin cannot read host credentials and cannot edit remote files by design.
