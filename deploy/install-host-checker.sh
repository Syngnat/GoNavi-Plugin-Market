#!/bin/bash
# Install or refresh the host CLI used to re-check uploaded plugin packages.
#
# The marketplace shares its manifest rules with the host SDK, but only the real
# binary proves the released host agrees. The service runs `gonavi plugin check`
# against every upload, so it needs a gonavi build for this machine's platform.
#
# The binary is downloaded from the official release named by --version (default:
# the newest release), its SHA-256 verified against the release's checksum file,
# installed atomically, and its path written to the service environment as
# GONAVI_MARKET_HOST_BINARY. A failed download or checksum leaves the currently
# installed binary and environment untouched.
#
# Usage:
#   deploy/install-host-checker.sh [--version 1.2.1] [--host BeroHost-6h16-DE]
#                                  [--port 22] [--remote-root /srv/gonavi-plugin-market]
#                                  [--env-file /etc/gonavi-market/env] [--dry-run]
#
# Run it on the server (or over SSH via --host). Requires curl, tar, sha256sum.
set -euo pipefail

HOST=""
PORT=""
VERSION=""
REMOTE_ROOT=/srv/gonavi-plugin-market
ENV_FILE=/etc/gonavi-market/env
DRY_RUN=0
REPO=Syngnat/GoNavi

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --host) HOST="$2"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --remote-root) REMOTE_ROOT="$2"; shift 2 ;;
    --env-file) ENV_FILE="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }

if [ "$DRY_RUN" = "1" ]; then
  log "dry run: would install the host checker for version ${VERSION:-<latest>} into $REMOTE_ROOT and write $ENV_FILE"
  exit 0
fi

# Everything below runs on the target host. Values that came from flags travel as
# environment assignments so the remote script stays literal.
if [ -n "$HOST" ]; then
  SSH_ARGS=(-o BatchMode=yes)
  SCP_ARGS=(-q)
  if [ -n "$PORT" ]; then
    SSH_ARGS+=(-p "$PORT"); SCP_ARGS+=(-P "$PORT")
  fi
  scp "${SCP_ARGS[@]}" "$0" "$HOST:/tmp/install-host-checker.sh"
  ssh "${SSH_ARGS[@]}" "$HOST" \
    "VERSION='$VERSION' REMOTE_ROOT='$REMOTE_ROOT' ENV_FILE='$ENV_FILE' bash /tmp/install-host-checker.sh" \
    && ssh "${SSH_ARGS[@]}" "$HOST" "rm -f /tmp/install-host-checker.sh"
  exit $?
fi

TOOL_DIR=$REMOTE_ROOT/host-tools
TARGET=$TOOL_DIR/gonavi

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64)  ASSET_OS=linux;  ASSET_ARCH=amd64 ;;
  Linux/aarch64) ASSET_OS=linux;  ASSET_ARCH=arm64 ;;
  *) echo "unsupported platform: $(uname -s)/$(uname -m); install a gonavi build manually and set GONAVI_MARKET_HOST_BINARY" >&2; exit 1 ;;
esac

if [ -z "$VERSION" ]; then
  log "resolving the newest release"
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$VERSION" ] || { echo "could not resolve the newest release" >&2; exit 1; }
fi
VERSION=${VERSION#v}
log "installing host checker $VERSION for $ASSET_OS/$ASSET_ARCH"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
NAME=gonavi-cli_${VERSION}_${ASSET_OS}_${ASSET_ARCH}
BASE="https://github.com/$REPO/releases/download/v$VERSION"

curl -fsSL -o "$WORK/$NAME.tar.gz" "$BASE/$NAME.tar.gz"
curl -fsSL -o "$WORK/checksums.txt" "$BASE/gonavi-cli_${VERSION}_checksums.txt" || true

if [ -s "$WORK/checksums.txt" ]; then
  WANT=$(awk -v n="$NAME.tar.gz" '$2 == n || $2 == "*" n { print $1 }' "$WORK/checksums.txt" | head -n 1)
  if [ -z "$WANT" ]; then
    echo "checksum for $NAME.tar.gz is missing from the release" >&2
    exit 1
  fi
  GOT=$(sha256sum "$WORK/$NAME.tar.gz" | awk '{ print $1 }')
  [ "$WANT" = "$GOT" ] || { echo "checksum mismatch for $NAME.tar.gz" >&2; exit 1; }
  log "checksum verified"
else
  echo "release does not publish gonavi-cli checksums; refusing to install unverified bytes" >&2
  exit 1
fi

tar -xzf "$WORK/$NAME.tar.gz" -C "$WORK"
BIN=$(find "$WORK" -maxdepth 2 -type f -name 'gonavi*' ! -name '*.tar.gz' | head -n 1)
[ -n "$BIN" ] || { echo "archive does not contain a gonavi binary" >&2; exit 1; }

# Prove the binary runs here before it replaces a working one.
if ! "$BIN" -version >/dev/null 2>&1; then
  echo "downloaded binary does not run on this host" >&2
  exit 1
fi

install -d -m 755 "$TOOL_DIR"
install -m 755 -o root -g root "$BIN" "$TARGET.new"
mv -f "$TARGET.new" "$TARGET"
log "installed $TARGET ($("$TARGET" -version 2>/dev/null || echo unknown))"

# Only the service reads this file, so keep it readable by root alone.
if grep -q '^GONAVI_MARKET_HOST_BINARY=' "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^GONAVI_MARKET_HOST_BINARY=.*|GONAVI_MARKET_HOST_BINARY=$TARGET|" "$ENV_FILE"
else
  printf 'GONAVI_MARKET_HOST_BINARY=%s\n' "$TARGET" >> "$ENV_FILE"
fi
log "recorded GONAVI_MARKET_HOST_BINARY in $ENV_FILE"

if systemctl is-active --quiet gonavi-market 2>/dev/null; then
  systemctl restart gonavi-market
  for _ in $(seq 1 30); do
    curl -fsS -o /dev/null http://127.0.0.1:8090/api/v1/plugins 2>/dev/null && break
    sleep 1
  done
  log "service restarted with the host checker enabled"
else
  log "gonavi-market is not running; the checker takes effect on next start"
fi
