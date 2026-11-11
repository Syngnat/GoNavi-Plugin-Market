#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Push only public/catalog.json to the live site (no rebuild, no full release).

The site fetches catalog.json at runtime, so listing a new official plugin is
just: edit public/catalog.json in a commit, then run this script.

This is also the repair path for the release pipeline. plugin-release.yml pushes
the catalog commit with GITHUB_TOKEN, and GitHub does not start workflows for
events triggered by that token, so deploy.yml never republishes the site. The
release workflow therefore calls this script itself; without it the static
fallback keeps advertising the previous version months after the API moved on.

Usage:
  python3 deploy/update-catalog.py [--host user@host] [--port 22]
                                   [--remote-root /srv/gonavi-plugin-market]
                                   [--dry-run] [--skip-verify]

Environment:
  SSH_KEY    identity file for ssh/scp (same contract as deploy/publish-site.sh)
  DEPLOY_HOST, DEPLOY_PORT   fall back for --host / --port

Exit codes: 0 updated or already current, 1 refused or verification failed.
"""
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DEFAULT_HOST = os.environ.get("DEPLOY_HOST") or "BeroHost-6h16-DE"
DEFAULT_PORT = os.environ.get("DEPLOY_PORT") or ""
REMOTE_ROOT = "/srv/gonavi-plugin-market"
VERIFY_URL = "https://plugin.gonavi.org/catalog.json"

host = DEFAULT_HOST
port = DEFAULT_PORT
remote_root = REMOTE_ROOT
dry_run = False
skip_verify = False

argv = sys.argv[1:]
while argv:
    flag = argv.pop(0)
    if flag == "--host" and argv:
        host = argv.pop(0)
    elif flag == "--port" and argv:
        port = argv.pop(0)
    elif flag == "--remote-root" and argv:
        remote_root = argv.pop(0)
    elif flag == "--dry-run":
        dry_run = True
    elif flag == "--skip-verify":
        skip_verify = True
    elif flag in ("-h", "--help"):
        print(__doc__)
        sys.exit(0)
    else:
        sys.exit(f"update-catalog: unknown argument {flag}")

if not remote_root.startswith("/"):
    sys.exit(f"update-catalog: --remote-root must be absolute, got {remote_root}")

catalog = ROOT / "public" / "catalog.json"
if not catalog.is_file():
    sys.exit(f"missing {catalog}")

data = json.loads(catalog.read_text(encoding="utf-8"))
if not data.get("items"):
    sys.exit("refuse: catalog has no items (would blank the market)")
for item in data["items"]:
    if not item.get("artifact", {}).get("sha256"):
        sys.exit(f"refuse: {item.get('id')} has no artifact.sha256")

digest = hashlib.sha256(catalog.read_bytes()).hexdigest()
versions = ", ".join(f"{i['id']}={i['version']}" for i in data["items"][:4])
print(f"catalog {digest[:12]} ({versions})")

if dry_run:
    print(f"dry run: would push to {host}:{remote_root}/current/catalog.json")
    sys.exit(0)

SSH_ARGS = ["-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=20"]
SCP_ARGS = ["-q"] + SSH_ARGS
ssh_key = os.environ.get("SSH_KEY", "").strip()
if ssh_key:
    SSH_ARGS += ["-i", ssh_key, "-o", "IdentitiesOnly=yes"]
    SCP_ARGS += ["-i", ssh_key, "-o", "IdentitiesOnly=yes"]
if port:
    SSH_ARGS += ["-p", str(port)]
    SCP_ARGS += ["-P", str(port)]

STAGING = f"/root/plugin-market-catalog-{digest[:12]}.json"
DEST = f"{remote_root}/current/catalog.json"

# The remote side re-checks the digest, then installs atomically under a lock:
# publishing a release that swaps `current` must never race a catalog write.
remote_script = f"""set -euo pipefail
echo '{digest}  {STAGING}' | sha256sum -c - >/dev/null
[ -d {remote_root}/current ] || {{ echo 'no {remote_root}/current on this host' >&2; exit 1; }}
exec 9>/tmp/gonavi-market-catalog.lock
flock 9
prev=$(sha256sum {DEST} 2>/dev/null | awk '{{print $1}}' || true)
install -m 644 {STAGING} {DEST}.new && mv -f {DEST}.new {DEST}
actual=$(sha256sum {DEST} | awk '{{print $1}}')
[ "$actual" = '{digest}' ] || {{ echo "installed catalog digest $actual != {digest}" >&2; exit 1; }}
rm -f {STAGING}
echo "catalog installed (${{prev:-none}} -> {digest[:12]})"
"""

with tempfile.NamedTemporaryFile("w", delete=False, suffix=".sh") as tmp:
    tmp.write(remote_script)
    script = tmp.name

try:
    subprocess.run(["scp"] + SCP_ARGS + [str(catalog), f"{host}:{STAGING}"], check=True)
    subprocess.run(["scp"] + SCP_ARGS + [script, f"{host}:/root/update-catalog-run.sh"], check=True)
    subprocess.run(
        ["ssh"] + SSH_ARGS + [host,
         "sed -i 's/\\r$//' /root/update-catalog-run.sh && bash /root/update-catalog-run.sh;"
         " rc=$?; rm -f /root/update-catalog-run.sh; exit $rc"],
        check=True,
    )
finally:
    Path(script).unlink(missing_ok=True)

print("CATALOG-UPDATED")

if skip_verify:
    print("origin verification skipped")
    sys.exit(0)

# The site serves this file straight from `current`; poll briefly so a
# transient proxy or reload delay is not reported as a bad publish.
for attempt in range(5):
    try:
        with urllib.request.urlopen(VERIFY_URL, timeout=10) as resp:
            live = hashlib.sha256(resp.read()).hexdigest()
        if live == digest:
            print(f"origin catalog matches {digest[:12]}")
            sys.exit(0)
    except Exception as err:  # noqa: BLE001 - retried below, reported at the end
        last_error = err
    else:
        last_error = f"origin digest {live[:12]} != {digest[:12]}"
    time.sleep(2)

print(f"origin catalog verification failed: {last_error}", file=sys.stderr)
sys.exit(1)
