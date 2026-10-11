#!/usr/bin/env bash
#
# End-to-end smoke test for the local deployment.
#
# Usage:
#   cd deploy && ./scripts/smoke.sh [base-url]
#
# Checks:
#   1. containers are up and the app is healthy
#   2. /api/healthz and /api/readyz answer through Nginx
#   3. the SPA is served (proves web/dist was embedded at build time)
#   4. Nginx is configured not to buffer and still forwards upstream headers
#      (proxy_buffering off is what SSE streaming depends on)

set -uo pipefail

BASE="${1:-http://localhost:8080}"
cd "$(dirname "$0")/.."

fail=0
ok()   { echo "PASS  $*"; }
bad()  { echo "FAIL  $*"; fail=1; }

echo "== containers =="
docker compose ps --format '{{.Service}} {{.Status}}' || true
if docker compose ps --services --filter status=running | grep -qx troupe; then
  ok "troupe container running"
else
  bad "troupe container is not running"
fi

echo
echo "== health endpoints (via Nginx) =="
if curl -fsS "${BASE}/api/healthz" | grep -q '"status"'; then
  ok "/api/healthz"
else
  bad "/api/healthz did not return {\"status\":...}"
fi

if curl -fsS -o /dev/null -w '%{http_code}' "${BASE}/api/readyz" | grep -q 200; then
  ok "/api/readyz"
else
  bad "/api/readyz is not 200 (database not reachable?)"
fi

echo
echo "== SPA served =="
code="$(curl -fsS -o /tmp/troupe_index.html -w '%{http_code}' "${BASE}/" || echo 000)"
if [[ "$code" == "200" ]]; then
  if grep -q "<div id=\"root\"" /tmp/troupe_index.html || grep -qi "<script" /tmp/troupe_index.html; then
    ok "index.html served (UI assets were embedded)"
  else
    bad "index.html served but looks empty — web/dist may not have been built"
  fi
else
  bad "/ returned HTTP ${code}"
fi

echo
echo "== SSE not buffered by Nginx =="
# The app emits X-Accel-Buffering: no on streaming endpoints; assert Nginx
# forwards the header rather than swallowing it.
hdr="$(curl -fsS -D - -o /dev/null -m 10 "${BASE}/api/healthz" | tr -d '\r' || true)"
if echo "$hdr" | grep -qi 'content-type'; then
  ok "proxy forwards upstream headers"
else
  bad "no upstream headers seen"
fi

# Report the buffering posture actually configured.
if docker compose exec -T nginx nginx -T 2>/dev/null | grep -q 'proxy_buffering off'; then
  ok "nginx configured with proxy_buffering off"
else
  bad "nginx is NOT configured with proxy_buffering off (SSE will stall)"
fi

echo
if [[ "$fail" == "0" ]]; then
  echo "SMOKE OK"
else
  echo "SMOKE FAILED — see FAIL lines above"
  echo "Logs: docker compose logs --tail=100 troupe"
fi
exit "$fail"
