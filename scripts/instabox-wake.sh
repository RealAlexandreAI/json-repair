#!/usr/bin/env bash
# Wake/resume an Instabox machine; wait until state=running && runtime.ready.
# Required: INSTABOX_API_KEY, INSTABOX_MACHINE_ID
# Optional: INSTABOX_API_BASE (default https://api.instabox.sh)
#           INSTABOX_WAKE_TIMEOUT_SEC (default 180)
#           INSTABOX_POLL_INTERVAL_SEC (default 3)
set -euo pipefail

API_BASE="${INSTABOX_API_BASE:-https://api.instabox.sh}"
API_BASE="${API_BASE%/}"
KEY="${INSTABOX_API_KEY:-}"
MID="${INSTABOX_MACHINE_ID:-}"
TIMEOUT="${INSTABOX_WAKE_TIMEOUT_SEC:-180}"
INTERVAL="${INSTABOX_POLL_INTERVAL_SEC:-3}"

if [[ -z "$KEY" || -z "$MID" ]]; then
  echo "instabox-wake: need INSTABOX_API_KEY and INSTABOX_MACHINE_ID" >&2
  exit 2
fi

HOST="${API_BASE#https://}"
HOST="${HOST#http://}"
HOST="${HOST%%/*}"

need_resolve=0
RESOLVE_ARGS=()
if command -v getent >/dev/null 2>&1; then
  resolved="$(getent ahostsv4 "$HOST" 2>/dev/null | awk '{print $1; exit}' || true)"
else
  resolved=""
fi
case "$resolved" in
  198.18.*|198.19.*|10.*|172.1[6-9].*|172.2[0-9].*|172.3[0-1].*|192.168.*|127.*|"")
    need_resolve=1 ;;
esac

if [[ "$need_resolve" -eq 1 ]]; then
  ip="$(curl -fsS --max-time 10 -H 'accept: application/dns-json' \
    "https://cloudflare-dns.com/dns-query?name=${HOST}&type=A" \
    | jq -r '.Answer[]? | select(.type==1) | .data' | head -1 || true)"
  if [[ -z "$ip" ]]; then
    echo "instabox-wake: DoH failed to resolve $HOST" >&2
    exit 3
  fi
  RESOLVE_ARGS=(--resolve "${HOST}:443:${ip}")
  echo "instabox-wake: using DoH resolve $HOST -> $ip"
fi

api() {
  local method="$1" path="$2"; shift 2
  curl -fsS --max-time 60 "${RESOLVE_ARGS[@]}" \
    -X "$method" \
    -H "X-API-Key: ${KEY}" \
    -H "Accept: application/json" \
    -H "Content-Type: application/json" \
    "$@" \
    "${API_BASE}${path}"
}

get_machine() { api GET "/v1/machines/${MID}"; }

idem() {
  if command -v uuidgen >/dev/null 2>&1; then uuidgen
  else cat /proc/sys/kernel/random/uuid 2>/dev/null || date +%s%N
  fi
}

echo "instabox-wake: machine=${MID} base=${API_BASE}"
before="$(get_machine)"
st="$(echo "$before" | jq -r '.state')"
rd="$(echo "$before" | jq -r '.runtime.ready')"
echo "instabox-wake: before state=${st} ready=${rd}"

if [[ "$st" == "running" && "$rd" == "true" ]]; then
  # Record activity via wake (no-op resume path)
  api POST "/v1/machines/${MID}/wake" -d '{}' >/dev/null || true
  echo "instabox-wake: already ready"
  exit 0
fi

# Prefer wake (handles idle-suspend, no Idempotency-Key). Fall back to resume with key.
set +e
wake_out="$(api POST "/v1/machines/${MID}/wake" -d '{}' 2>/tmp/instabox-wake-wake.err)"
wrc=$?
set -e
if [[ $wrc -eq 0 ]]; then
  echo "instabox-wake: wake accepted; state=$(echo "$wake_out" | jq -r '.state // empty')"
else
  echo "instabox-wake: wake soft-failed; trying resume with Idempotency-Key"
  set +e
  api POST "/v1/machines/${MID}/resume" \
    -H "Idempotency-Key: wake-$(idem)" \
    -d '{}' >/tmp/instabox-wake-resume.json 2>/tmp/instabox-wake-resume.err
  rrc=$?
  set -e
  if [[ $rrc -ne 0 ]]; then
    echo "instabox-wake: resume failed: $(tr -d '\n' </tmp/instabox-wake-resume.err | head -c 300)" >&2
    exit 4
  fi
  echo "instabox-wake: resume accepted"
fi

deadline=$((SECONDS + TIMEOUT))
while (( SECONDS < deadline )); do
  m="$(get_machine)"
  st="$(echo "$m" | jq -r '.state')"
  ready="$(echo "$m" | jq -r '.runtime.ready')"
  echo "instabox-wake: poll state=${st} ready=${ready} preserved=$(echo "$m" | jq -r '.runtime.processesPreserved // empty')"
  if [[ "$st" == "running" && "$ready" == "true" ]]; then
    echo "instabox-wake: ready"
    exit 0
  fi
  if [[ "$st" == "error" ]]; then
    echo "instabox-wake: machine in error: $(echo "$m" | jq -c '.lastError // {}')" >&2
    exit 5
  fi
  sleep "$INTERVAL"
done
echo "instabox-wake: timeout after ${TIMEOUT}s" >&2
get_machine | jq '{state,desiredState,runtime,lastError}' >&2 || true
exit 1
