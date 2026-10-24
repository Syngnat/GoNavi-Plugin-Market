#!/bin/bash
# Provision the GoNavi plugin market service on Bero (first run).
# Idempotent-ish: refuses to reinitialize an existing deployment.
set -euo pipefail

ROOT=/srv/gonavi-plugin-market
SERVER_DIR=$ROOT/server
ARTIFACTS=$ROOT/artifacts
ENV_FILE=/etc/gonavi-market/env
LOG_DIR=/var/log/gonavi-market
PG_CONTAINER=gonavi-market-pg
PG_PORT=25433
DB_NAME=gonavi_market
OWNER_ROLE=gonavi_market_owner
APP_ROLE=gonavi_market_app

if [ -f "$ENV_FILE" ]; then
  echo "refuse: $ENV_FILE already exists; deployment seems initialized" >&2
  exit 1
fi
if ss -tln | grep -q "127.0.0.1:$PG_PORT "; then
  echo "refuse: 127.0.0.1:$PG_PORT already in use" >&2
  exit 1
fi
if ss -tln | grep -q '127.0.0.1:8090 '; then
  echo "refuse: 127.0.0.1:8090 already in use" >&2
  exit 1
fi

PW_OWNER=$(openssl rand -hex 20)
PW_APP=$(openssl rand -hex 20)

echo "[1/8] postgres container"
docker run -d --name "$PG_CONTAINER" --restart unless-stopped \
  -p 127.0.0.1:$PG_PORT:5432 \
  -e POSTGRES_USER=$OWNER_ROLE -e POSTGRES_PASSWORD="$PW_OWNER" -e POSTGRES_DB=$DB_NAME \
  -v gonavi-market-pgdata:/var/lib/postgresql/data \
  postgres:16-alpine >/dev/null

echo "[2/8] wait for postgres"
for i in $(seq 1 60); do
  if docker exec "$PG_CONTAINER" pg_isready -U "$OWNER_ROLE" -d "$DB_NAME" >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$PG_CONTAINER" pg_isready -U "$OWNER_ROLE" -d "$DB_NAME" >/dev/null

echo "[3/8] users and directories"
getent group gonavi-market >/dev/null || groupadd --system gonavi-market
id -u gonavi-market >/dev/null 2>&1 || useradd --system --gid gonavi-market --home-dir /nonexistent --shell /usr/sbin/nologin gonavi-market
mkdir -p "$SERVER_DIR" "$ARTIFACTS" "$LOG_DIR" /etc/gonavi-market
chown root:root "$SERVER_DIR"
chmod 755 "$SERVER_DIR"
chown -R gonavi-market:gonavi-market "$ARTIFACTS" "$LOG_DIR"
chmod 700 "$ARTIFACTS"

echo "[4/8] apply migrations (operator credential)"
GONAVI_MARKET_DATABASE_URL="postgres://${OWNER_ROLE}:${PW_OWNER}@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable" \
  "$SERVER_DIR/gonavi-market-server" -migrate

echo "[5/8] runtime role and grants"
docker exec -i "$PG_CONTAINER" psql -U "$OWNER_ROLE" -d "$DB_NAME" >/dev/null <<SQL
CREATE ROLE ${APP_ROLE} LOGIN PASSWORD '${PW_APP}';
GRANT CONNECT ON DATABASE ${DB_NAME} TO ${APP_ROLE};
GRANT USAGE ON SCHEMA public TO ${APP_ROLE};
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ${APP_ROLE};
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO ${APP_ROLE};
ALTER DEFAULT PRIVILEGES FOR ROLE ${OWNER_ROLE} IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ${APP_ROLE};
ALTER DEFAULT PRIVILEGES FOR ROLE ${OWNER_ROLE} IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO ${APP_ROLE};
SQL

echo "[6/8] environment file"
cat > "$ENV_FILE" <<EOF
GONAVI_MARKET_DATABASE_URL=postgres://${APP_ROLE}:${PW_APP}@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable
GONAVI_MARKET_PUBLIC_URL=https://plugin.gonavi.org
GONAVI_MARKET_LISTEN=127.0.0.1:8090
GONAVI_MARKET_ARTIFACT_ROOT=${ARTIFACTS}
GONAVI_LOG_DIR=${LOG_DIR}
# Enable login by filling both GitHub OAuth values and restarting the service:
# GONAVI_MARKET_GITHUB_CLIENT_ID=
# GONAVI_MARKET_GITHUB_CLIENT_SECRET=
EOF
chown root:root "$ENV_FILE"
chmod 600 "$ENV_FILE"

echo "[7/8] systemd unit"
cat > /etc/systemd/system/gonavi-market.service <<EOF
[Unit]
Description=GoNavi plugin market service
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=gonavi-market
Group=gonavi-market
EnvironmentFile=$ENV_FILE
ExecStart=$SERVER_DIR/gonavi-market-server
Restart=on-failure
RestartSec=3
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=$ARTIFACTS $LOG_DIR
ProtectHome=yes
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now gonavi-market >/dev/null

echo "[8/8] local smoke test"
sleep 2
for i in $(seq 1 20); do
  if curl -fsS http://127.0.0.1:8090/api/v1/plugins >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -fsS http://127.0.0.1:8090/api/v1/plugins | head -c 200; echo

cat > /root/gonavi-market-credentials.txt <<EOF
# GoNavi market service credentials (root only)
# PostgreSQL operator (migrations / bootstrap):
postgres://${OWNER_ROLE}:${PW_OWNER}@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable
# Service runtime (in ${ENV_FILE}):
# role ${APP_ROLE} password: ${PW_APP}
# Container: ${PG_CONTAINER} (data volume gonavi-market-pgdata)
EOF
chown root:root /root/gonavi-market-credentials.txt
chmod 600 /root/gonavi-market-credentials.txt

echo "PROVISION-OK"
