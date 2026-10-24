# -*- coding: utf-8 -*-
"""Package a GoNavi plugin directory into its release zip (manifest + sources + bin)."""
import hashlib
import os
import shutil
import sys
import zipfile
from pathlib import Path

def package(repo_dir: str, zip_name: str, exclude_dirs=(".git",)) -> str:
    root = Path(repo_dir).resolve()
    out = root / zip_name
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        for current, dirs, files in os.walk(root):
            dirs[:] = [d for d in dirs if d not in exclude_dirs]
            for name in files:
                path = Path(current) / name
                arc = path.relative_to(root).as_posix()
                if arc == zip_name:
                    continue
                z.write(path, arc)
    digest = hashlib.sha256(out.read_bytes()).hexdigest()
    with zipfile.ZipFile(out) as z:
        names = z.namelist()
    assert "gonavi.plugin.json" in names, names
    bins = [n for n in names if n.startswith("bin/")]
    print(f"packaged {out.name}: {len(names)} entries, bins={bins}")
    print(f"sha256 {digest}  {out.name}")
    return digest

if __name__ == "__main__":
    package(sys.argv[1], sys.argv[2])
