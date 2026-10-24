# -*- coding: utf-8 -*-
"""Push only public/catalog.json to the live site (no rebuild, no full release).

The site fetches catalog.json at runtime, so listing a new official plugin is
just: edit public/catalog.json in a commit, then run this script.

Usage:  python deploy/update-catalog.py
"""
import hashlib
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
REMOTE = "Bero-6H16-DE"
DEST = "/srv/gonavi-plugin-market/current/catalog.json"
STAGING = "/root/plugin-market-catalog-staging.json"

catalog = ROOT / "public" / "catalog.json"
if not catalog.is_file():
    sys.exit(f"missing {catalog}")

import json
data = json.loads(catalog.read_text(encoding="utf-8"))
if not data.get("items"):
    sys.exit("refuse: catalog has no items (would blank the market)")
for item in data["items"]:
    if not item.get("artifact", {}).get("sha256"):
        sys.exit(f"refuse: {item.get('id')} has no artifact.sha256")

digest = hashlib.sha256(catalog.read_bytes()).hexdigest()
with tempfile.NamedTemporaryFile(delete=False, suffix=".sh") as tmp:
    tmp.write(f"""sha256sum -c <<EOF
{digest}  {STAGING}
EOF
install -m 644 {STAGING} {DEST}.new && mv -f {DEST}.new {DEST}
curl -fsS https://plugin.gonavi.org/catalog.json | sha256sum
""".encode())
    script = tmp.name
subprocess.run(["scp", "-o", "BatchMode=yes", str(catalog), f"{REMOTE}:{STAGING}"], check=True)
subprocess.run(["scp", "-o", "BatchMode=yes", script, f"{REMOTE}:/root/update-catalog-run.sh"], check=True)
subprocess.run(["ssh", "-o", "BatchMode=yes", REMOTE, "sed -i 's/\\r$//' /root/update-catalog-run.sh && bash /root/update-catalog-run.sh && rm -f /root/update-catalog-run.sh " + STAGING], check=True)
Path(script).unlink(missing_ok=True)
print("CATALOG-UPDATED")
