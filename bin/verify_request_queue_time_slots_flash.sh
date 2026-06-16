#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify_request_queue_time_slots_flash.sh

Environment variables:
  BASE_URL                  Backend base URL. Default: http://localhost:3000
  ADMIN_USERNAME            Admin username. Default: admin
  ADMIN_PASSWORD            Admin password. Default: 12345678
  ADMIN_UID                 Dashboard header user id. Defaults to login response data.id
  MODEL                     Relay model to test. Default: deepseek-v4-flash
  COOKIE_JAR                Cookie jar path. Default: temporary file
  WORKDIR                   Artifact directory. Default: temporary directory
  PREFIX                    Test data prefix. Default: qslot<unix-timestamp>
  PG_CONTAINER              PostgreSQL container name. Default: new-api-dev-pg
  REDIS_CONTAINER           Redis container name. Default: new-api-dev-redis
  DB_NAME                   Database name. Default: new-api
  DB_USER                   Database name. Default: root
  RATE_LIMIT_COUNT          Temporary ModelRequestRateLimitCount. Default: 2
  RATE_LIMIT_SUCCESS_COUNT  Temporary ModelRequestRateLimitSuccessCount. Default: 1000000
  RATE_LIMIT_DURATION_MIN   Temporary ModelRequestRateLimitDurationMinutes. Default: 1
  QUEUE_TIMEOUT_SECONDS     Temporary queue timeout. Default: 90
  QUEUE_MAX_SIZE            Temporary queue max size. Default: 50
  MAX_TOKENS                Relay max_tokens. Default: 32

What it verifies:
  1. Queue config time_slots are persisted and read back through the dashboard API.
  2. A currently matching enabled slot queues real deepseek-v4-flash requests over RPM.
  3. Queued real requests complete and produce consume logs with queue_wait_ms.
  4. queue_status=queued and queue_status=unqueued log filters distinguish queued and non-queued logs.
  5. A non-matching slot does not queue over-RPM requests.
  6. A currently matching disabled slot does not queue over-RPM requests.

Artifacts are written under WORKDIR and printed at the end.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

BASE_URL=${BASE_URL:-http://localhost:3000}
ADMIN_USERNAME=${ADMIN_USERNAME:-admin}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-12345678}
ADMIN_UID=${ADMIN_UID:-}
MODEL=${MODEL:-deepseek-v4-flash}
PG_CONTAINER=${PG_CONTAINER:-new-api-dev-pg}
REDIS_CONTAINER=${REDIS_CONTAINER:-new-api-dev-redis}
DB_NAME=${DB_NAME:-new-api}
DB_USER=${DB_USER:-root}
RATE_LIMIT_COUNT=${RATE_LIMIT_COUNT:-2}
RATE_LIMIT_SUCCESS_COUNT=${RATE_LIMIT_SUCCESS_COUNT:-1000000}
RATE_LIMIT_DURATION_MIN=${RATE_LIMIT_DURATION_MIN:-1}
QUEUE_TIMEOUT_SECONDS=${QUEUE_TIMEOUT_SECONDS:-90}
QUEUE_MAX_SIZE=${QUEUE_MAX_SIZE:-50}
MAX_TOKENS=${MAX_TOKENS:-32}
PREFIX=${PREFIX:-qslot$(date +%s)}
WORKDIR=${WORKDIR:-$(mktemp -d /tmp/request-queue-slots.XXXXXX)}
COOKIE_JAR=${COOKIE_JAR:-$(mktemp /tmp/request-queue-slots-cookies.XXXXXX)}

declare -a OPTION_KEYS=(
  "ModelRequestRateLimitEnabled"
  "ModelRequestRateLimitCount"
  "ModelRequestRateLimitSuccessCount"
  "ModelRequestRateLimitDurationMinutes"
  "QueueEnabled"
  "QueueDefaultTimeout"
  "QueueMaxTimeout"
  "QueueGlobalMaxSize"
)

SNAPSHOT_OPTIONS_FILE="$WORKDIR/options-before.json"
SNAPSHOT_QUEUE_FILE="$WORKDIR/queue-config-before.json"
SNAPSHOT_QUEUE_STATUS_FILE="$WORKDIR/queue-config-before.status"
RESTORE_REQUIRED=0
SCRIPT_SUCCESS=0

log() {
  printf '[queue-slot-test] %s\n' "$*"
}

fail() {
  printf '[queue-slot-test] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

json_payload() {
  local key=$1
  local value=$2
  python3 - <<'PY' "$key" "$value"
import json
import sys
print(json.dumps({"key": sys.argv[1], "value": sys.argv[2]}, ensure_ascii=False))
PY
}

run_sql() {
  docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -Atqc "$1"
}

api() {
  local method=$1
  local path=$2
  local data=${3-}

  if [[ -n "$data" ]]; then
    curl --noproxy '*' -sS -b "$COOKIE_JAR" -H "New-Api-User: $ADMIN_UID" -H 'Content-Type: application/json' -X "$method" "$BASE_URL$path" --data-binary "$data"
    return
  fi

  curl --noproxy '*' -sS -b "$COOKIE_JAR" -H "New-Api-User: $ADMIN_UID" -X "$method" "$BASE_URL$path"
}

api_to_file() {
  local method=$1
  local path=$2
  local out_file=$3
  local status

  status=$(curl --noproxy '*' -sS -o "$out_file" -w '%{http_code}' -b "$COOKIE_JAR" -H "New-Api-User: $ADMIN_UID" -X "$method" "$BASE_URL$path")
  printf '%s' "$status"
}

put_option() {
  local key=$1
  local value=$2
  api PUT /api/option/ "$(json_payload "$key" "$value")" >/dev/null
}

assert_api_success_file() {
  local file=$1
  python3 - <<'PY' "$file"
import json
import sys
with open(sys.argv[1]) as f:
    payload = json.load(f)
if not payload.get("success"):
    raise SystemExit(payload.get("message") or "api returned success=false")
PY
}

restore_options() {
  [[ -f "$SNAPSHOT_OPTIONS_FILE" ]] || return 0
  python3 - <<'PY' "$SNAPSHOT_OPTIONS_FILE" "${OPTION_KEYS[*]}" | while IFS=$'\t' read -r key value; do
import json
import sys

path = sys.argv[1]
keys = set(sys.argv[2].split())
with open(path) as f:
    items = json.load(f)["data"]
lookup = {item["key"]: item["value"] for item in items}
for key in keys:
    print(f"{key}\t{lookup.get(key, '')}")
PY
    [[ -n "$key" ]] || continue
    put_option "$key" "$value"
  done
}

restore_queue_config() {
  [[ -f "$SNAPSHOT_QUEUE_STATUS_FILE" ]] || return 0
  local status
  status=$(cat "$SNAPSHOT_QUEUE_STATUS_FILE")
  if [[ "$status" == "200" ]]; then
    python3 - <<'PY' "$SNAPSHOT_QUEUE_FILE" | while IFS= read -r payload; do
import json
import sys
with open(sys.argv[1]) as f:
    data = json.load(f)["data"]
payload = {
    "enabled": data["enabled"],
    "max_queue_size": data["max_queue_size"],
    "queue_timeout": data["queue_timeout"],
    "long_context_tiers": data.get("long_context_tiers") or [],
    "time_slots": data.get("time_slots") or [],
}
print(json.dumps(payload, ensure_ascii=False))
PY
      api PUT "/api/queue/config/$MODEL" "$payload" >/dev/null
    done
    return 0
  fi
  api DELETE "/api/queue/config/$MODEL" >/dev/null || true
}

delete_test_user_cache() {
  local user_ids
  user_ids=$(run_sql "select id from users where username like '${PREFIX}_user_%' order by id;" | tr '\n' ' ')
  [[ -n "$user_ids" ]] || return 0

  local redis_keys=()
  local uid
  for uid in $user_ids; do
    redis_keys+=("user:$uid" "rateLimit:$uid" "rateLimit:MRRLS:$uid")
  done
  docker exec "$REDIS_CONTAINER" redis-cli DEL "${redis_keys[@]}" >/dev/null || true
}

delete_test_rows() {
  delete_test_user_cache
  docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -qc "delete from tokens where key like '${PREFIX}%'; delete from users where username like '${PREFIX}%'; delete from companies where code like '${PREFIX}%';" >/dev/null || true
}

cleanup() {
  local exit_code=$?
  if [[ "$RESTORE_REQUIRED" -eq 1 ]]; then
    log "restoring queue settings and test data"
    restore_options || true
    restore_queue_config || true
    delete_test_rows || true
  fi

  if [[ "$SCRIPT_SUCCESS" -eq 1 ]]; then
    log "artifacts kept in $WORKDIR"
  else
    log "artifacts kept in $WORKDIR for debugging"
  fi

  exit "$exit_code"
}

trap cleanup EXIT

login_admin() {
  log "logging in as $ADMIN_USERNAME"
  local status
  status=$(curl --noproxy '*' -sS -o "$WORKDIR/login.json" -w '%{http_code}' -c "$COOKIE_JAR" -H 'Content-Type: application/json' -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" "$BASE_URL/api/user/login")
  [[ "$status" == "200" ]] || fail "admin login failed with HTTP $status"

  python3 - <<'PY' "$WORKDIR/login.json"
import json
import sys
with open(sys.argv[1]) as f:
    data = json.load(f)
if not data.get("success"):
    raise SystemExit(data.get("message") or "admin login returned success=false")
PY

  if [[ -z "$ADMIN_UID" ]]; then
    ADMIN_UID=$(python3 - <<'PY' "$WORKDIR/login.json"
import json
import sys
with open(sys.argv[1]) as f:
    data = json.load(f)
print(data["data"]["id"])
PY
)
  fi
}

assert_model_available() {
  api GET /api/user/models > "$WORKDIR/models.json"
  python3 - <<'PY' "$WORKDIR/models.json" "$MODEL"
import json
import sys
with open(sys.argv[1]) as f:
    payload = json.load(f)
models = payload.get("data") or []
if sys.argv[2] not in models:
    raise SystemExit(f"model {sys.argv[2]} not found in /api/user/models")
PY
}

snapshot_state() {
  log "snapshotting current options and queue config"
  api GET /api/option/ > "$SNAPSHOT_OPTIONS_FILE"
  api_to_file GET "/api/queue/config/$MODEL" "$SNAPSHOT_QUEUE_FILE" > "$SNAPSHOT_QUEUE_STATUS_FILE"
  RESTORE_REQUIRED=1
}

apply_test_options() {
  log "applying temporary queue settings for $MODEL"
  put_option "ModelRequestRateLimitEnabled" "true"
  put_option "ModelRequestRateLimitCount" "$RATE_LIMIT_COUNT"
  put_option "ModelRequestRateLimitSuccessCount" "$RATE_LIMIT_SUCCESS_COUNT"
  put_option "ModelRequestRateLimitDurationMinutes" "$RATE_LIMIT_DURATION_MIN"
  put_option "QueueEnabled" "true"
  put_option "QueueDefaultTimeout" "$QUEUE_TIMEOUT_SECONDS"
  put_option "QueueMaxTimeout" "$QUEUE_TIMEOUT_SECONDS"
  put_option "QueueGlobalMaxSize" "$QUEUE_MAX_SIZE"
}

slot_config_payload() {
 local mode=$1
  local include_tiers=${2:-false}
  python3 - <<'PY' "$mode" "$include_tiers" "$QUEUE_MAX_SIZE" "$QUEUE_TIMEOUT_SECONDS"
import datetime as dt
import json
import sys

mode = sys.argv[1]
include_tiers = sys.argv[2] == "true"
queue_max_size = int(sys.argv[3])
queue_timeout = int(sys.argv[4])
now = dt.datetime.now()
go_weekday = (now.weekday() + 1) % 7

slot_enabled = True
weekdays = [go_weekday]
if mode == "unmatched":
    weekdays = [(go_weekday + 1) % 7]
elif mode == "disabled":
    slot_enabled = False
elif mode != "matching":
    raise SystemExit(f"unknown slot mode: {mode}")

long_context_tiers = []
if include_tiers:
    long_context_tiers = [
        {
            "threshold_tokens": 1,
            "max_running": 1,
            "lease_turns": 2,
            "lease_idle_timeout_seconds": 10,
        }
    ]

payload = {
    "enabled": True,
    "max_queue_size": queue_max_size,
    "queue_timeout": queue_timeout,
    "long_context_tiers": long_context_tiers,
    "time_slots": [
        {
            "start_time": "00:00",
            "end_time": "23:59",
            "weekdays": weekdays,
            "enabled": slot_enabled,
            "max_queue_size": queue_max_size,
            "queue_timeout": queue_timeout,
            "long_context_tiers": long_context_tiers,
        }
    ],
}
print(json.dumps(payload, ensure_ascii=False))
PY
}

apply_slot_config() {
  local mode=$1
  local expected_enabled=$2
  local include_tiers=${3:-false}
  local payload_file="$WORKDIR/queue-config-${mode}.payload.json"
  local response_file="$WORKDIR/queue-config-${mode}.response.json"
  slot_config_payload "$mode" "$include_tiers" > "$payload_file"
  api PUT "/api/queue/config/$MODEL" "$(cat "$payload_file")" > "$response_file"
  assert_api_success_file "$response_file"
  api GET "/api/queue/config/$MODEL" > "$WORKDIR/queue-config-${mode}.readback.json"
  assert_api_success_file "$WORKDIR/queue-config-${mode}.readback.json"
  python3 - <<'PY' "$WORKDIR/queue-config-${mode}.readback.json" "$expected_enabled" "$include_tiers" "$QUEUE_MAX_SIZE" "$QUEUE_TIMEOUT_SECONDS"
import json
import sys

with open(sys.argv[1]) as f:
    data = json.load(f)["data"]
expected_enabled = sys.argv[2] == "true"
include_tiers = sys.argv[3] == "true"
expected_size = int(sys.argv[4])
expected_timeout = int(sys.argv[5])
slots = data.get("time_slots") or []
if len(slots) != 1:
    raise SystemExit(f"expected one time slot, got {len(slots)}")
slot = slots[0]
if slot.get("enabled") is not expected_enabled:
    raise SystemExit(f"expected slot enabled={expected_enabled}, got {slot.get('enabled')}")
if slot.get("max_queue_size") != expected_size:
    raise SystemExit(f"unexpected slot max_queue_size: {slot.get('max_queue_size')}")
if slot.get("queue_timeout") != expected_timeout:
    raise SystemExit(f"unexpected slot queue_timeout: {slot.get('queue_timeout')}")
tiers = slot.get("long_context_tiers") or []
if include_tiers and (len(tiers) != 1 or tiers[0].get("threshold_tokens") != 1):
    raise SystemExit("slot long_context_tiers were not persisted")
if not include_tiers and tiers:
    raise SystemExit("slot long_context_tiers should be empty for runtime queue-log scenario")
PY
}

insert_company() {
  local name=$1
  run_sql "insert into companies(name, code, description, status, sort_order, queue_priority, created_at, updated_at) values ('${PREFIX}_${name}_company','${PREFIX}_${name}_c','queue slot test ${name}',1,0,5,extract(epoch from now())::bigint,extract(epoch from now())::bigint) returning id;"
}

insert_user() {
  local name=$1
  local company_id=$2
  run_sql "insert into users(username,password,display_name,role,status,\"group\",company_id,quota,created_at,last_login_at) values ('${PREFIX}_user_${name}','x','${PREFIX}_user_${name}',1,1,'default',${company_id},100000000,extract(epoch from now())::bigint,0) returning id;"
}

insert_token() {
  local user_id=$1
  local token_key=$2
  local token_name=$3
  run_sql "insert into tokens(user_id,key,status,name,created_time,accessed_time,expired_time,remain_quota,unlimited_quota,model_limits_enabled,model_limits,allow_ips,used_quota,\"group\",queue_priority,queue_timeout,cross_group_retry) values (${user_id},'${token_key}',1,'${token_name}',extract(epoch from now())::bigint,extract(epoch from now())::bigint,-1,0,true,false,'','',0,'default',5,${QUEUE_TIMEOUT_SECONDS},false);"
}

provision_bundle() {
  local name=$1
  local company_id user_id token_key
  company_id=$(insert_company "$name")
  user_id=$(insert_user "$name" "$company_id")
  token_key="${PREFIX}_${name}_token"
  insert_token "$user_id" "$token_key" "$token_key" >/dev/null
  docker exec "$REDIS_CONTAINER" redis-cli DEL "user:$user_id" "rateLimit:$user_id" "rateLimit:MRRLS:$user_id" >/dev/null || true
  printf '%s %s\n' "$user_id" "$token_key"
}

request_body() {
  python3 - <<'PY' "$MODEL" "$MAX_TOKENS"
import json
import sys
print(json.dumps({
    "model": sys.argv[1],
    "messages": [{"role": "user", "content": "reply with ok"}],
    "max_tokens": int(sys.argv[2]),
    "temperature": 0,
}, ensure_ascii=False))
PY
}

relay_request() {
  local token=$1
  local body_file=$2
  local meta_file=$3
  local body=$4
  curl --noproxy '*' -sS -o "$body_file" -w '%{http_code} %{time_total}\n' -H "Authorization: Bearer sk-${token}" -H 'Content-Type: application/json' "$BASE_URL/v1/chat/completions" -d "$body" > "$meta_file"
}

assert_relay_status() {
  local meta_file=$1
  local expected=$2
  local status
  status=$(awk '{print $1}' "$meta_file")
  [[ "$status" == "$expected" ]] || fail "expected HTTP $expected for $meta_file, got $status"
}

queue_status() {
  api GET "/api/queue/status/$MODEL"
}

capture_queue_hit() {
  local out_file=$1
  python3 - <<'PY' "$out_file" "$BASE_URL" "$COOKIE_JAR" "$ADMIN_UID" "$MODEL"
import json
import subprocess
import sys
import time

out_file, base_url, cookie_jar, admin_uid, model = sys.argv[1:6]
for _ in range(1200):
    result = subprocess.run([
        "curl", "--noproxy", "*", "-sS", "-b", cookie_jar,
        "-H", f"New-Api-User: {admin_uid}",
        f"{base_url}/api/queue/status/{model}",
    ], check=True, capture_output=True, text=True)
    with open(out_file, "w") as f:
        f.write(result.stdout)
    payload = json.loads(result.stdout)["data"]
    if payload.get("queued", 0) > 0:
        raise SystemExit(0)
    time.sleep(0.05)
raise SystemExit(1)
PY
}

assert_queue_empty() {
  local out_file=$1
  queue_status > "$out_file"
  python3 - <<'PY' "$out_file"
import json
import sys
with open(sys.argv[1]) as f:
    payload = json.load(f)["data"]
if payload.get("queued", 0) != 0:
    raise SystemExit(f"expected queued=0, got {payload.get('queued')}")
PY
}

wait_for_log_filter() {
  local token_name=$1
  local queue_status_value=$2
  local out_file=$3
  python3 - <<'PY' "$BASE_URL" "$COOKIE_JAR" "$ADMIN_UID" "$MODEL" "$token_name" "$queue_status_value" "$out_file"
import json
import subprocess
import sys
import time
import urllib.parse

base_url, cookie_jar, admin_uid, model, token_name, queue_status_value, out_file = sys.argv[1:8]
query = urllib.parse.urlencode({
    "p": 1,
    "page_size": 20,
    "type": 2,
    "model_name": model,
    "token_name": token_name,
    "queue_status": queue_status_value,
})
url = f"{base_url}/api/log/?{query}"
for _ in range(180):
    result = subprocess.run([
        "curl", "--noproxy", "*", "-sS", "-b", cookie_jar,
        "-H", f"New-Api-User: {admin_uid}",
        url,
    ], check=True, capture_output=True, text=True)
    with open(out_file, "w") as f:
        f.write(result.stdout)
    payload = json.loads(result.stdout)
    if not payload.get("success"):
        raise SystemExit(payload.get("message") or "log api returned success=false")
    items = ((payload.get("data") or {}).get("items") or [])
    if queue_status_value == "queued":
        for item in items:
            other = item.get("other") or ""
            if "queue_wait_ms" in other:
                raise SystemExit(0)
    elif items:
        raise SystemExit(0)
    time.sleep(1)
raise SystemExit(f"timed out waiting for {queue_status_value} log for {token_name}")
PY
}

assert_no_queued_log() {
  local token_name=$1
  local out_file=$2
  api GET "/api/log/?p=1&page_size=20&type=2&model_name=$MODEL&token_name=$token_name&queue_status=queued" > "$out_file"
  python3 - <<'PY' "$out_file"
import json
import sys
with open(sys.argv[1]) as f:
    payload = json.load(f)
if not payload.get("success"):
    raise SystemExit(payload.get("message") or "log api returned success=false")
items = ((payload.get("data") or {}).get("items") or [])
if items:
    raise SystemExit(f"expected no queued logs, got {len(items)}")
PY
}

run_matching_slot_case() {
  log "running matching enabled time-slot queue scenario"
  local scenario_dir="$WORKDIR/matching"
  mkdir -p "$scenario_dir"
  apply_slot_config matching true true
  apply_slot_config matching true false

  local user_id token body queued_pid
  read -r user_id token < <(provision_bundle matching)
  body=$(request_body)

  relay_request "$token" "$scenario_dir/blocker1.body" "$scenario_dir/blocker1.meta" "$body"
  relay_request "$token" "$scenario_dir/blocker2.body" "$scenario_dir/blocker2.meta" "$body"
  assert_relay_status "$scenario_dir/blocker1.meta" 200
  assert_relay_status "$scenario_dir/blocker2.meta" 200

  (
    relay_request "$token" "$scenario_dir/queued.body" "$scenario_dir/queued.meta" "$body"
  ) &
  queued_pid=$!

  capture_queue_hit "$scenario_dir/snapshot.json" || fail "did not capture queued snapshot for matching slot"
  wait "$queued_pid"
  assert_relay_status "$scenario_dir/queued.meta" 200

  wait_for_log_filter "$token" queued "$scenario_dir/logs-queued.json"
  wait_for_log_filter "$token" unqueued "$scenario_dir/logs-unqueued.json"

  python3 - <<'PY' "$scenario_dir" "$token"
import json
import os
import sys

scenario_dir, token = sys.argv[1:3]
with open(os.path.join(scenario_dir, "snapshot.json")) as f:
    snapshot = json.load(f)["data"]
with open(os.path.join(scenario_dir, "queued.meta")) as f:
    status, total = f.read().strip().split()
summary = {
    "case": "matching",
    "token": token,
    "queued_snapshot": snapshot["queued"],
    "queued_status": int(status),
    "queued_time_total_sec": float(total),
}
with open(os.path.join(scenario_dir, "summary.json"), "w") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False))
PY
}

run_not_queued_slot_case() {
  local mode=$1
  local expected_enabled=$2
  log "running $mode time-slot non-queue scenario"
  local scenario_dir="$WORKDIR/$mode"
  mkdir -p "$scenario_dir"
  apply_slot_config "$mode" "$expected_enabled" false

  local user_id token body
  read -r user_id token < <(provision_bundle "$mode")
  body=$(request_body)

  relay_request "$token" "$scenario_dir/blocker1.body" "$scenario_dir/blocker1.meta" "$body"
  relay_request "$token" "$scenario_dir/blocker2.body" "$scenario_dir/blocker2.meta" "$body"
  assert_relay_status "$scenario_dir/blocker1.meta" 200
  assert_relay_status "$scenario_dir/blocker2.meta" 200

  relay_request "$token" "$scenario_dir/rejected.body" "$scenario_dir/rejected.meta" "$body"
  assert_relay_status "$scenario_dir/rejected.meta" 429
  assert_queue_empty "$scenario_dir/status-after-reject.json"
  assert_no_queued_log "$token" "$scenario_dir/logs-queued.json"

  python3 - <<'PY' "$scenario_dir" "$mode" "$token"
import json
import os
import sys

scenario_dir, mode, token = sys.argv[1:4]
with open(os.path.join(scenario_dir, "rejected.meta")) as f:
    status, total = f.read().strip().split()
summary = {
    "case": mode,
    "token": token,
    "rejected_status": int(status),
    "rejected_time_total_sec": float(total),
}
with open(os.path.join(scenario_dir, "summary.json"), "w") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False))
PY
}

emit_final_summary() {
  python3 - <<'PY' "$WORKDIR"
import json
import os
import sys

workdir = sys.argv[1]
summary = {
    "matching": json.load(open(os.path.join(workdir, "matching", "summary.json"))),
    "unmatched": json.load(open(os.path.join(workdir, "unmatched", "summary.json"))),
    "disabled": json.load(open(os.path.join(workdir, "disabled", "summary.json"))),
}
with open(os.path.join(workdir, "summary.json"), "w") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False, indent=2))
PY
}

main() {
  require_cmd curl
  require_cmd docker
  require_cmd python3

  mkdir -p "$WORKDIR"
  login_admin
  assert_model_available
  snapshot_state
  apply_test_options
  delete_test_rows
  run_matching_slot_case
  run_not_queued_slot_case unmatched true
  run_not_queued_slot_case disabled false
  log "all time-slot flash scenarios passed"
  emit_final_summary
  SCRIPT_SUCCESS=1
}

main "$@"
