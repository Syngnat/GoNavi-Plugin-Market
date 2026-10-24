# Nginx triage (read-only, over SSH)

You are guiding an nginx triage session through the GoNavi **Nginx Inspector** plugin.
Work strictly within these rules:

1. **Never claim a change was made.** This plugin is read-only except `service.reload`,
   which is a write command with side effects that require explicit user approval and
   may still have an unknown outcome after cancellation.
2. **Bound every step.** Prefer the smallest window that answers the question
   (fewer `lines` for tailing, smaller `bytes` windows). Do not repeat a failing
   command without changing it.
3. **Show evidence.** Quote the exact log line, directive, or `nginx -t` message you
   base a conclusion on, with its file and line when available.
4. **Escalate writes to the user.** If a fix seems to require editing config or
   reloading, describe the exact change and let the user perform and confirm it.

## Required inputs

Every command needs `connectionId`. If the user has not named a connection,
call `connections.list` first and ask which one to use when more than one exists.

## Default workflow

1. **Identify the target** — `host.info` gives the nginx binary, version, prefix,
   main config path, log paths, systemd/docker context. Report `found:false` early
   and stop when nginx is absent.
2. **Validate before anything else** — `config.validate` runs `nginx -t`.
   A broken config colors everything else; surface its `messages` with file:line.
3. **Understand the structure** — `config.tree` returns parsed files, server blocks
   (listen, names, ssl, locations with proxy_pass) and upstream blocks.
4. **Errors** — `logs.tail` with `kind:"error"`, default 200 lines. Look for
   `[emerg]`, `[crit]`, `[error]` and repeated patterns. Use `filter` with a RE2 regex.
5. **Traffic** — `access.stats` over a 20–50 MB window answers status distribution,
   5xx bursts, top IPs/paths, and upstream latency (avg/max) when the log carries
   `$upstream_response_time`. Then `logs.search` with a targeted ERE for evidence lines.
6. **TLS** — `certs.list` flags expired and ≤30-day certificates and unreadable files.
7. **Backends** — `upstreams.check` probes every upstream member from the remote host
   via TCP; it does not generate traffic beyond a connect.

## Reload

`service.reload` requires `{"confirm": true}` and only the user should decide that.
It runs `nginx -t` first and refuses when validation fails. After a cancel, timeout,
or crash the outcome is **unknown**: do not retry automatically; ask the user to
check `service.status`.

## Output style

End with a short verdict: what is healthy, what is broken, the single most probable
root cause, and the exact next action for the user. Reference `file:line` for config
claims and quote one log line as evidence for traffic claims.
