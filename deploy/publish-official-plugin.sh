#!/bin/bash
# Publish one official plugin release into the market database.
#
# The periodic GitHub release sync only tracks plugins that carry a sourceUrl and
# reads each repo's *latest* release, so a multi-plugin repo (all official plugins
# share GoNavi-Plugin-Market) can never sync by itself: the latest release belongs
# to whichever plugin was tagged last. Official plugins are therefore imported one
# release at a time through the submission pipeline with -publish-official.
#
# Usage:
#   deploy/publish-official-plugin.sh --plugin mongo-shell --tag mongo-shell-v1.0.4 \
#       [--reviewer 92659908] [--host BeroHost-6h16-DE] [--port 37167] \
#       [--namespace com.gonavi] [--dry-run]
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PLUGIN=""
TAG=""
REVIEWER=""
HOST=BeroHost-6h16-DE
PORT=""
NAMESPACE=com.gonavi
SERVER_BIN=/srv/gonavi-plugin-market/server/gonavi-market-server
ENV_FILE=/etc/gonavi-market/env
DRY_RUN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --plugin) PLUGIN=$2; shift 2 ;;
    --tag) TAG=$2; shift 2 ;;
    --reviewer) REVIEWER=$2; shift 2 ;;
    --host) HOST=$2; shift 2 ;;
    --port) PORT=$2; shift 2 ;;
    --namespace) NAMESPACE=$2; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

log() { printf '%s\n' "==> $*"; }

[ -n "$PLUGIN" ] && [ -n "$TAG" ] || { echo "--plugin and --tag are required" >&2; exit 2; }
MANIFEST="$REPO_ROOT/plugins/$PLUGIN/gonavi.plugin.json"
[ -f "$MANIFEST" ] || { echo "no manifest at $MANIFEST" >&2; exit 1; }
PLUGIN_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["id"])' "$MANIFEST")
VERSION=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$MANIFEST")

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

WORK=$(mktemp -d "${TMPDIR:-/tmp}/official-publish.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

log "download $TAG asset from GitHub"
( cd "$WORK" && gh release download "$TAG" --repo Syngnat/GoNavi-Plugin-Market --pattern '*.zip' --clobber >/dev/null )
ARCHIVE=$(ls "$WORK"/*.zip | head -1)
SIZE=$(wc -c < "$ARCHIVE" | tr -d ' ')
log "archive $(basename "$ARCHIVE") ($SIZE bytes) for $PLUGIN_ID $VERSION"

if [ "$DRY_RUN" = "1" ]; then
  log "dry run: stopping before upload"
  exit 0
fi

REMOTE_PATH="/root/$(basename "$ARCHIVE")"
log "upload to $HOST"
scp "${SCP_ARGS[@]}" "$ARCHIVE" "$HOST:$REMOTE_PATH"

REVIEW_ARGS=""
if [ -n "$REVIEWER" ]; then
  REVIEW_ARGS="-official-reviewer $REVIEWER"
fi

log "import through the submission pipeline"
ssh "${SSH_ARGS[@]}" "$HOST" "set -euo pipefail
  set -a; . $ENV_FILE; set +a
  $SERVER_BIN -publish-official $REMOTE_PATH -official-namespace $NAMESPACE $REVIEW_ARGS
  rm -f $REMOTE_PATH
"

log "verify public API"
LIVE=$(curl -fsS https://plugin.gonavi.org/api/v1/plugins | python3 -c '
import json, sys
data = json.load(sys.stdin)
items = data if isinstance(data, list) else (data.get("items") or [])
target = sys.argv[1]
for item in items:
    if item.get("id") == target:
        print(item.get("version", ""))
        break
' "$PLUGIN_ID")
log "live $PLUGIN_ID = $LIVE (manifest $VERSION)"
[ "$LIVE" = "$VERSION" ] || { echo "published version does not match the manifest" >&2; exit 1; }
log "done"
