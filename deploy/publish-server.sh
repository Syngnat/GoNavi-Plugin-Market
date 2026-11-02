#!/bin/bash
# Publish the plugin-market service binary to plugin.gonavi.org.
#
# Builds a static linux/amd64 binary from a committed ref in a throwaway
# worktree, so uncommitted local edits never reach production. Uploads one
# binary, verifies its SHA-256 remotely, backs up the running binary, installs
# the new one atomically, applies the bundled migrations with the operator
# credential, restarts the service and waits for a healthy read API. Any
# failure restores the previous binary and restarts again.
#
# Usage:
#   deploy/publish-server.sh [--ref origin/main] [--host BeroHost-6h16-DE]
#                            [--port 22] [--remote-root /srv/gonavi-plugin-market]
#                            [--keep 5] [--dry-run] [--skip-verify]
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
    -h|--help) sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
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
TAG="server-${SHA}-$(date -u +%Y%m%dT%H%M%SZ)"
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/market-server-build.XXXXXX")
BINARY="${TMPDIR:-/tmp}/gonavi-market-server-${TAG}"

cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$STAGE" >/dev/null 2>&1 || true
  rm -rf "$STAGE"
  rm -f "$BINARY"
}
trap cleanup EXIT

log "build $REF ($SHA) in throwaway worktree"
git -C "$REPO_ROOT" worktree add --detach "$STAGE" "$REF" >/dev/null
# The production host is linux/amd64; a static build keeps the binary free of
# host libc assumptions. Same flags as the manual deployments before this script.
( cd "$STAGE" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o "$BINARY" ./cmd/gonavi-market-server )

[ -s "$BINARY" ] || { echo "build produced no binary" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  SHA256=$(sha256sum "$BINARY" | awk '{print $1}')
else
  SHA256=$(shasum -a 256 "$BINARY" | awk '{print $1}')
fi
SIZE=$(wc -c < "$BINARY" | tr -d ' ')
log "artifact $BINARY ($SIZE bytes) sha256 $SHA256"

if [ "$DRY_RUN" = "1" ]; then
  log "dry run: stopping before upload (tag would be $TAG)"
  exit 0
fi

log "upload to $HOST"
scp "${SCP_ARGS[@]}" "$BINARY" "$HOST:/root/gonavi-market-server-staging"

log "remote verify, install and restart"
# Values travel as environment assignments, so the script below stays literal
# and needs no nested quoting; every path is an absolute literal on the host.
ssh "${SSH_ARGS[@]}" "$HOST" "SHA256=$SHA256 TAG=$TAG REMOTE_ROOT=$REMOTE_ROOT KEEP=$KEEP bash -s" <<'REMOTE'
set -euo pipefail
SERVER_DIR=$REMOTE_ROOT/server
BIN=$SERVER_DIR/gonavi-market-server
STAGING=/root/gonavi-market-server-staging
ENV_FILE=/etc/gonavi-market/env
HEALTH_URL=http://127.0.0.1:8090/api/v1/plugins
HEALTH_TIMEOUT=30

echo "$SHA256  $STAGING" | sha256sum -c - >/dev/null

exec 9>/tmp/gonavi-market-publish.lock
flock 9

bak=$SERVER_DIR/gonavi-market-server.bak-$TAG
cp -p "$BIN" "$bak"
install -m 755 -o root -g root "$STAGING" "$BIN.new"
mv -f "$BIN.new" "$BIN"
rm -f "$STAGING"

# Migrations are embedded in the binary, so apply them with the operator
# credential before the new process starts. A failed migration leaves the old
# binary on disk and the old process untouched in memory.
set -a; . "$ENV_FILE"; set +a
if ! "$BIN" -migrate >/tmp/gonavi-market-migrate.log 2>&1; then
  echo "migration failed; restoring $bak" >&2
  cat /tmp/gonavi-market-migrate.log >&2 || true
  mv -f "$bak" "$BIN"
  exit 1
fi

systemctl restart gonavi-market
healthy=0
for _ in $(seq 1 "$HEALTH_TIMEOUT"); do
  if curl -fsS -o /dev/null "$HEALTH_URL" 2>/dev/null; then healthy=1; break; fi
  sleep 1
done
if [ "$healthy" != "1" ]; then
  echo "health check failed; rolling back to $bak" >&2
  mv -f "$bak" "$BIN"
  systemctl restart gonavi-market
  for _ in $(seq 1 "$HEALTH_TIMEOUT"); do
    curl -fsS -o /dev/null "$HEALTH_URL" 2>/dev/null && break
    sleep 1
  done
  journalctl -u gonavi-market -n 30 --no-pager >&2 || true
  exit 1
fi

# Keep a bounded set of backups so a bad publish can be rolled back by hand.
ls -1dt "$SERVER_DIR"/gonavi-market-server.bak-* 2>/dev/null | tail -n +$((KEEP + 1)) | xargs -r rm -f

echo "revision: $(strings "$BIN" | grep -m1 -o 'vcs.revision=[0-9a-f]*' || echo unknown)"
systemctl is-active gonavi-market
REMOTE

rm -f "$BINARY"

if [ "$SKIP_VERIFY" = "1" ]; then
  log "published $TAG (host verification skipped)"
  exit 0
fi

log "verify public read API"
LIVE=$(curl -fsS https://plugin.gonavi.org/api/v1/plugins >/dev/null && echo ok || echo failed)
[ "$LIVE" = "ok" ] || { echo "public read API is not serving" >&2; exit 1; }
log "published $TAG"
