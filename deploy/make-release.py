# -*- coding: utf-8 -*-
"""Build the market site release tarball from dist/ and print tag + sha256."""
import hashlib
import subprocess
import sys
import tarfile
import time
from pathlib import Path

root = Path(r"D:\Work\CodeRepos\GoNavi-Plugin-Market")
sha = subprocess.check_output(["git", "-C", str(root), "rev-parse", "--short", "HEAD"]).decode().strip()
tag = f"{sha}-20261024T{time.strftime('%H%M%S', time.gmtime())}Z"
out = Path(rf"D:\Work\CodeRepos\plugin-market-{tag}.tar.gz")
with tarfile.open(out, "w:gz") as t:
    t.add(root / "dist", arcname=".")
digest = hashlib.sha256(out.read_bytes()).hexdigest()
print("TAG", tag)
print("FILE", out)
print("SHA256", digest)
