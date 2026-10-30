#!/bin/bash
# Publish the plugin-market website to plugin.gonavi.org.
#
# Builds from a committed ref in a throwaway worktree, so uncommitted local
# edits never reach production. Uploads one tarball, verifies it remotely,
# unpacks into releases/<tag>/ and swaps the `current` symlink atomically.
#
# Usage:
#   deploy/publish-site.sh [--ref origin/main] [--host BeroHost-6h16-DE]
#                          [--port 22] [--remote-root /srv/gonavi-plugin-market]
#                          [--keep 5] [--dry-run] [--skip-verify]
#
# Host may be an ssh alias or user@host (CI passes the latter). Port defaults to
# the alias' own port when --port is omitted; SSH_KEY selects an identity file.
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
HOST=BeroHost-6h16-DE
REF=origin/main
KEEP=5
DRY_RUN=0
SKIP_VERIFY=0
PORT=""
REMOTE_ROOT=/srv/gonavi-plugin-market

while [ $# -gt 0 ]; do
  case "$1" in
    --ref) REF=$2; shift 2 ;;
    --host) HOST=$2; shift 2 ;;
    --keep) KEEP=$2; shift 2 ;;
    --port) PORT=$2; shift 2 ;;
    --remote-root) REMOTE_ROOT=$2; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-verify) SKIP_VERIFY=1; shift ;;
    -h|--help) sed -n '2,12p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

log() { printf '%s\n' "==> $*"; }

SSH_ARGS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=20)
SCP_ARGS=(-q -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=20)
if [ -n "${SSH_KEY:-}" ]; then
  SSH_ARGS+=(-i "$SSH_KEY" -o IdentitiesOnly=yes)
  SCP_ARGS+=(-i "$SSH_KEY" -o IdentitiesOnly=yes)
fi
if [ -n "$PORT" ]; then
  SSH_ARGS+=(-p "$PORT")
  SCP_ARGS+=(-P "$PORT")
fi

cd "$REPO_ROOT"
git rev-parse --verify "$REF" >/dev/null 2>&1 || { echo "ref not found: $REF" >&2; exit 1; }
SHA=$(git rev-parse --short "$REF")
TAG="site-${SHA}-$(date -u +%Y%m%dT%H%M%SZ)"
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/market-build.XXXXXX")
TARBALL="${TMPDIR:-/tmp}/plugin-market-site-${TAG}.tar.gz"

cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$STAGE" >/dev/null 2>&1 || true
  rm -rf "$STAGE"
}
trap cleanup EXIT

log "build $REF ($SHA) in throwaway worktree"
git -C "$REPO_ROOT" worktree add --detach "$STAGE" "$REF" >/dev/null

# Reuse the local dependency tree when present; a cold checkout pays for npm ci.
if [ -d "$REPO_ROOT/node_modules" ]; then
  if cp -Rc "$REPO_ROOT/node_modules" "$STAGE/node_modules" 2>/dev/null; then :; else
    cp -R "$REPO_ROOT/node_modules" "$STAGE/node_modules"
  fi
else
  ( cd "$STAGE" && npm ci --prefer-offline >/dev/null )
fi

( cd "$STAGE" && npm run build >/dev/null )

[ -f "$STAGE/dist/catalog.json" ] || { echo "build produced no dist/catalog.json" >&2; exit 1; }
VERSIONS=$(python3 - "$STAGE/dist/catalog.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
items = data.get("items") or []
print(", ".join(f"{i['id']}={i['version']}" for i in items[:4]))
PY
)
log "catalog plugins: $VERSIONS"

tar -czf "$TARBALL" -C "$STAGE/dist" .
if command -v sha256sum >/dev/null 2>&1; then
  SHA256=$(sha256sum "$TARBALL" | awk '{print $1}')
else
  SHA256=$(shasum -a 256 "$TARBALL" | awk '{print $1}')
fi
SIZE=$(wc -c < "$TARBALL" | tr -d ' ')
log "artifact $TARBALL ($SIZE bytes) sha256 $SHA256"

if [ "$DRY_RUN" = "1" ]; then
  log "dry run: stopping before upload (tag would be $TAG)"
  exit 0
fi

log "upload to $HOST"
scp "${SCP_ARGS[@]}" "$TARBALL" "$HOST:/root/plugin-market-staging.tar.gz"

log "remote verify, unpack and switch current -> $TAG"
# 并发发布会同时改 current：串行化切换，且不让较旧的构建覆盖较新的。
ssh "${SSH_ARGS[@]}" "$HOST" "set -euo pipefail
  echo '$SHA256  /root/plugin-market-staging.tar.gz' | sha256sum -c - >/dev/null
  exec 9>/tmp/gonavi-market-publish.lock
  flock 9
  mkdir -p $REMOTE_ROOT/releases/$TAG
  tar -xzf /root/plugin-market-staging.tar.gz -C $REMOTE_ROOT/releases/$TAG
  CUR=\$(readlink -f $REMOTE_ROOT/current 2>/dev/null || true)
  if [ -n \"\$CUR\" ] && [ -d \"\$CUR\" ] && [ \"\$CUR\" -nt \"$REMOTE_ROOT/releases/$TAG\" ]; then
    echo \"skip switch: current is newer than $TAG\"
    rm -rf \"$REMOTE_ROOT/releases/$TAG\"
  else
    ln -sfn $REMOTE_ROOT/releases/$TAG $REMOTE_ROOT/current.tmp
    mv -Tf $REMOTE_ROOT/current.tmp $REMOTE_ROOT/current
  fi
  rm -f /root/plugin-market-staging.tar.gz
  ls -l $REMOTE_ROOT/current
  # keep the newest N releases so a bad publish can be rolled back by hand
  ls -1dt $REMOTE_ROOT/releases/*/ | tail -n +\$(( $KEEP + 1 )) | xargs -r rm -rf
"

rm -f "$TARBALL"

if [ "$SKIP_VERIFY" = "1" ]; then
  log "published $TAG (origin verification skipped)"
  exit 0
fi

log "verify origin"
ssh "${SSH_ARGS[@]}" "$HOST" 'bash -s' < "$REPO_ROOT/deploy/verify-origin.sh"

log "verify public catalog"
LIVE=$(curl -fsS https://plugin.gonavi.org/catalog.json | python3 -c 'import json,sys
d = json.load(sys.stdin)
print(" ".join(i["id"] + "=" + i["version"] for i in d.get("items", [])[:4]))')
log "live: $LIVE"
log "published $TAG"
