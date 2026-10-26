#!/bin/sh
# Source-side verification for the plugin market site (runs on Bero).
set -e
RESOLVE='--resolve plugin.gonavi.org:2053:127.0.0.1'
BASE='https://plugin.gonavi.org:2053'
ASSET=$(ls /srv/gonavi-plugin-market/current/assets | head -n1)
echo "asset=$ASSET"

curl -s $RESOLVE "$BASE/" -o /dev/null -w 'index (LE cert): %{http_code}\n'
curl -s $RESOLVE "$BASE/assets/$ASSET" -o /dev/null -w 'asset status: %{http_code}\n'
curl -s $RESOLVE "$BASE/assets/$ASSET" -D- -o /dev/null | grep -i '^cache-control' | sed 's/^/asset /'
curl -s $RESOLVE "$BASE/index.html" -D- -o /dev/null | grep -i '^cache-control' | sed 's/^/index /'
curl -s $RESOLVE "$BASE/plugins/com.gonavi.sql-insights" -o /dev/null -w 'spa fallback: %{http_code}\n'
curl -s $RESOLVE "$BASE/login" -o /dev/null -w 'spa login: %{http_code}\n'
curl -s $RESOLVE "$BASE/favicon.svg" -o /dev/null -w 'favicon: %{http_code}\n'
curl -s $RESOLVE -X POST "$BASE/assets/x" -o /dev/null -w 'post /assets (want 403): %{http_code}\n'
curl -s $RESOLVE "$BASE/no-such-page" -o /dev/null -w 'spa unknown route: %{http_code}\n'
curl -s $RESOLVE "$BASE/gonavi/fonts/v1/catalog.json" -o /dev/null -w 'font namespace isolation (want 404): %{http_code}\n'
curl -s --resolve plugin.gonavi.org:2053:127.0.0.1 "$BASE/" | grep -c 'GoNavi' | sed 's/^/index mentions GoNavi: /'
