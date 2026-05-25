#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify_request_queue_flash.sh

Environment variables:
  BASE_URL                  Backend base URL. Default: http://localhost:3000
  ADMIN_USERNAME            Admin username. Default: admin
  ADMIN_PASSWORD            Admin password. Default: 12345678
  ADMIN_UID                 Dashboard header user id. Defaults to login response data.id
  MODEL                     Relay model to test. Default: deepseek-v4-flash
  COOKIE_JAR                Cookie jar path. Default: temporary file
  WORKDIR                   Artifact directory. Default: temporary directory
  PREFIX                    Test data prefix. Default: qtest<unix-timestamp>
  PG_CONTAINER              PostgreSQL container name. Default: new-api-dev-pg
  REDIS_CONTAINER           Redis container name. Default: new-api-dev-redis
  DB_NAME                   Database name. Default: new-api
  DB_USER                   Database user. Default: root
  RATE_LIMIT_COUNT          Temporary ModelRequestRateLimitCount. Default: 2
  RATE_LIMIT_SUCCESS_COUNT  Temporary ModelRequestRateLimitSuccessCount. Default: 1000000
  RATE_LIMIT_DURATION_MIN   Temporary ModelRequestRateLimitDurationMinutes. Default: 1
  QUEUE_TIMEOUT_SECONDS     Temporary queue timeout. Default: 600
  QUEUE_MAX_SIZE            Temporary queue max size. Default: 100

What it verifies:
  1. Requests over the flash RPM threshold queue instead of returning 429.
  2. Token + Company priority resolves to buckets 10 / 6 / 1 as designed.
  3. Weighted scheduling serves high-priority flash requests faster on average.
  4. Low-priority flash requests still complete and are not starved.

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
QUEUE_TIMEOUT_SECONDS=${QUEUE_TIMEOUT_SECONDS:-600}
QUEUE_MAX_SIZE=${QUEUE_MAX_SIZE:-100}
PREFIX=${PREFIX:-qtest$(date +%s)}
WORKDIR=${WORKDIR:-$(mktemp -d /tmp/request-queue-flash.XXXXXX)}
COOKIE_JAR=${COOKIE_JAR:-$(mktemp /tmp/request-queue-cookies.XXXXXX)}

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
CREATED_SHARED_A_UID=""
CREATED_SHARED_B_UID=""
CREATED_COMBO_HIGH_UID=""
CREATED_COMBO_MID_UID=""
CREATED_COMBO_LOW_UID=""
SCRIPT_SUCCESS=0

log() {
  printf '[queue-test] %s\n' "$*"
}

fail() {
  printf '[queue-test] ERROR: %s\n' "$*" >&2
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
    redis_keys+=("user:$uid")
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

  local login_success
  login_success=$(python3 - <<'PY' "$WORKDIR/login.json"
import json
import sys
with open(sys.argv[1]) as f:
    data = json.load(f)
print('true' if data.get('success') else 'false')
PY
)
  [[ "$login_success" == "true" ]] || fail "admin login returned success=false"

  if [[ -z "$ADMIN_UID" ]]; then
    ADMIN_UID=$(python3 - <<'PY' "$WORKDIR/login.json"
import json
import sys
with open(sys.argv[1]) as f:
    data = json.load(f)
print(data['data']['id'])
PY
)
  fi
}

assert_model_available() {
  api GET /api/user/models > "$WORKDIR/models.json"
  if ! python3 - <<'PY' "$WORKDIR/models.json" "$MODEL"
import json
import sys
with open(sys.argv[1]) as f:
    payload = json.load(f)
models = payload.get('data') or []
if sys.argv[2] not in models:
    raise SystemExit(1)
PY
  then
    fail "model $MODEL not found in /api/user/models"
  fi
}

snapshot_state() {
  log "snapshotting current options and queue config"
  api GET /api/option/ > "$SNAPSHOT_OPTIONS_FILE"
  api_to_file GET "/api/queue/config/$MODEL" "$SNAPSHOT_QUEUE_FILE" > "$SNAPSHOT_QUEUE_STATUS_FILE"
  RESTORE_REQUIRED=1
}

apply_test_settings() {
  log "applying temporary queue settings for $MODEL"
  put_option "ModelRequestRateLimitEnabled" "true"
  put_option "ModelRequestRateLimitCount" "$RATE_LIMIT_COUNT"
  put_option "ModelRequestRateLimitSuccessCount" "$RATE_LIMIT_SUCCESS_COUNT"
  put_option "ModelRequestRateLimitDurationMinutes" "$RATE_LIMIT_DURATION_MIN"
  put_option "QueueEnabled" "true"
  put_option "QueueDefaultTimeout" "$QUEUE_TIMEOUT_SECONDS"
  put_option "QueueMaxTimeout" "$QUEUE_TIMEOUT_SECONDS"
  put_option "QueueGlobalMaxSize" "$QUEUE_MAX_SIZE"
  api PUT "/api/queue/config/$MODEL" "{\"enabled\":true,\"max_queue_size\":$QUEUE_MAX_SIZE,\"queue_timeout\":$QUEUE_TIMEOUT_SECONDS}" >/dev/null
}

insert_company() {
  local name=$1
  local queue_priority=$2
  run_sql "insert into companies(name, code, description, status, sort_order, queue_priority, created_at, updated_at) values ('${PREFIX}_${name}','${PREFIX}_${name}','queue test ${name}',1,0,${queue_priority},extract(epoch from now())::bigint,extract(epoch from now())::bigint) returning id;"
}

insert_user() {
  local username=$1
  local company_id=$2
  run_sql "insert into users(username,password,display_name,role,status,\"group\",company_id,quota,created_at,last_login_at) values ('${PREFIX}_${username}','x','${PREFIX}_${username}',1,1,'default',${company_id},100000000,extract(epoch from now())::bigint,0) returning id;"
}

insert_token() {
  local user_id=$1
  local token_key=$2
  local token_name=$3
  local queue_priority=$4
  run_sql "insert into tokens(user_id,key,status,name,created_time,accessed_time,expired_time,remain_quota,unlimited_quota,model_limits_enabled,model_limits,allow_ips,used_quota,\"group\",queue_priority,queue_timeout,cross_group_retry) values (${user_id},'${token_key}',1,'${token_name}',extract(epoch from now())::bigint,extract(epoch from now())::bigint,-1,0,true,false,'','',0,'default',${queue_priority},${QUEUE_TIMEOUT_SECONDS},false);"
}

provision_user_bundle() {
  local bundle_name=$1
  local company_priority=$2
  local user_var=$3
  local company_var=$4
  local prefix_var=$5

  local company_id user_id token_prefix
  company_id=$(insert_company "${bundle_name}_company" "$company_priority")
  user_id=$(insert_user "${bundle_name}_user" "$company_id")
  token_prefix="${PREFIX}_${bundle_name}"

  printf -v "$company_var" '%s' "$company_id"
  printf -v "$user_var" '%s' "$user_id"
  printf -v "$prefix_var" '%s' "$token_prefix"
}

provision_shared_bundles() {
  log "creating shared-user bundles"
  local company_id user_id token_prefix

  provision_user_bundle shared_a 5 user_id company_id token_prefix
  CREATED_SHARED_A_UID=$user_id
  insert_token "$user_id" "${token_prefix}_block" "${token_prefix}_block" 5 >/dev/null
  insert_token "$user_id" "${token_prefix}_high" "${token_prefix}_high" 10 >/dev/null
  insert_token "$user_id" "${token_prefix}_mid" "${token_prefix}_mid" 8 >/dev/null
  insert_token "$user_id" "${token_prefix}_low" "${token_prefix}_low" 1 >/dev/null

  SHARED_A_BLOCK="${token_prefix}_block"
  SHARED_A_HIGH="${token_prefix}_high"
  SHARED_A_MID="${token_prefix}_mid"
  SHARED_A_LOW="${token_prefix}_low"

  provision_user_bundle shared_b 5 user_id company_id token_prefix
  CREATED_SHARED_B_UID=$user_id
  insert_token "$user_id" "${token_prefix}_block" "${token_prefix}_block" 5 >/dev/null
  insert_token "$user_id" "${token_prefix}_high1" "${token_prefix}_high1" 10 >/dev/null
  insert_token "$user_id" "${token_prefix}_high2" "${token_prefix}_high2" 10 >/dev/null
  insert_token "$user_id" "${token_prefix}_low1" "${token_prefix}_low1" 1 >/dev/null
  insert_token "$user_id" "${token_prefix}_low2" "${token_prefix}_low2" 1 >/dev/null

  SHARED_B_BLOCK="${token_prefix}_block"
  SHARED_B_HIGH1="${token_prefix}_high1"
  SHARED_B_HIGH2="${token_prefix}_high2"
  SHARED_B_LOW1="${token_prefix}_low1"
  SHARED_B_LOW2="${token_prefix}_low2"
}

provision_combo_bundles() {
  log "creating company-combination bundles"
  local company_id user_id token_prefix

  provision_user_bundle combo_high 10 user_id company_id token_prefix
  CREATED_COMBO_HIGH_UID=$user_id
  insert_token "$user_id" "${token_prefix}_token" "${token_prefix}_token" 10 >/dev/null
  COMBO_HIGH_TOKEN="${token_prefix}_token"

  provision_user_bundle combo_mid 3 user_id company_id token_prefix
  CREATED_COMBO_MID_UID=$user_id
  insert_token "$user_id" "${token_prefix}_token" "${token_prefix}_token" 8 >/dev/null
  COMBO_MID_TOKEN="${token_prefix}_token"

  provision_user_bundle combo_low 1 user_id company_id token_prefix
  CREATED_COMBO_LOW_UID=$user_id
  insert_token "$user_id" "${token_prefix}_token" "${token_prefix}_token" 1 >/dev/null
  COMBO_LOW_TOKEN="${token_prefix}_token"
}

request_body() {
  python3 - <<'PY' "$MODEL"
import json
import sys
print(json.dumps({
    "model": sys.argv[1],
    "messages": [{"role": "user", "content": "reply with ok"}],
    "max_tokens": 8,
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

smoke_test() {
  log "running smoke relay against $MODEL"
  local body
  body=$(request_body)
  relay_request "$SHARED_A_BLOCK" "$WORKDIR/smoke.body" "$WORKDIR/smoke.meta" "$body"
  local smoke_code
  smoke_code=$(awk '{print $1}' "$WORKDIR/smoke.meta")
  [[ "$smoke_code" == "200" ]] || fail "smoke relay failed with HTTP $smoke_code"
}

queue_status() {
  api GET "/api/queue/status/$MODEL"
}

capture_queue_hit() {
  local out_file=$1
  local expected_bucket_csv=$2

  python3 - <<'PY' "$out_file" "$expected_bucket_csv" "$BASE_URL" "$COOKIE_JAR" "$ADMIN_UID" "$MODEL"
import json
import os
import subprocess
import sys

out_file = sys.argv[1]
expected = [item for item in sys.argv[2].split(',') if item]
base_url, cookie_jar, admin_uid, model = sys.argv[3:7]

for _ in range(2000):
    result = subprocess.run([
        "curl", "--noproxy", "*", "-sS", "-b", cookie_jar,
        "-H", f"New-Api-User: {admin_uid}",
        f"{base_url}/api/queue/status/{model}",
    ], check=True, capture_output=True, text=True)
    with open(out_file, "w") as f:
        f.write(result.stdout)
    payload = json.loads(result.stdout)["data"]
    if payload.get("queued", 0) <= 0:
        continue
    buckets = payload.get("buckets", {})
    if all(buckets.get(bucket, 0) > 0 for bucket in expected):
        raise SystemExit(0)

raise SystemExit(1)
PY
}

run_queue_entry_case() {
  log "running queue-entry flash scenario"
  local scenario_dir="$WORKDIR/queue-entry"
  local body
  body=$(request_body)
  rm -rf "$scenario_dir"
  mkdir -p "$scenario_dir"

  relay_request "$SHARED_A_BLOCK" "$scenario_dir/blocker1.body" "$scenario_dir/blocker1.meta" "$body"
  relay_request "$SHARED_A_BLOCK" "$scenario_dir/blocker2.body" "$scenario_dir/blocker2.meta" "$body"

  (
    local start end
    start=$(date +%s)
    relay_request "$SHARED_A_HIGH" "$scenario_dir/high.body" "$scenario_dir/high.meta" "$body"
    end=$(date +%s)
    printf 'high %s %s\n' "$start" "$end" > "$scenario_dir/high.time"
  ) &
  local high_pid=$!

  (
    local start end
    start=$(date +%s)
    relay_request "$SHARED_A_MID" "$scenario_dir/mid.body" "$scenario_dir/mid.meta" "$body"
    end=$(date +%s)
    printf 'mid %s %s\n' "$start" "$end" > "$scenario_dir/mid.time"
  ) &
  local mid_pid=$!

  (
    local start end
    start=$(date +%s)
    relay_request "$SHARED_A_LOW" "$scenario_dir/low.body" "$scenario_dir/low.meta" "$body"
    end=$(date +%s)
    printf 'low %s %s\n' "$start" "$end" > "$scenario_dir/low.time"
  ) &
  local low_pid=$!

  capture_queue_hit "$scenario_dir/snapshot.json" "10,8,1" || fail "did not capture queued snapshot for queue-entry scenario"

  wait "$high_pid"
  wait "$mid_pid"
  wait "$low_pid"

  python3 - <<'PY' "$scenario_dir"
import json
import os
import sys

scenario_dir = sys.argv[1]
with open(os.path.join(scenario_dir, "snapshot.json")) as f:
    snapshot = json.load(f)["data"]
buckets = snapshot["buckets"]
if snapshot["queued"] < 1:
    raise SystemExit("queue-entry snapshot queued=0")
for bucket in ("10", "8", "1"):
    if buckets.get(bucket, 0) < 1:
        raise SystemExit(f"queue-entry missing bucket {bucket}")
for name in ("high", "mid", "low"):
    with open(os.path.join(scenario_dir, f"{name}.meta")) as f:
        status = f.read().strip().split()[0]
    if status != "200":
        raise SystemExit(f"queue-entry {name} returned HTTP {status}")

summary = {
    "queued": snapshot["queued"],
    "buckets": {key: buckets[key] for key in ("10", "8", "1")},
}
with open(os.path.join(scenario_dir, "summary.json"), "w") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False))
PY
}

run_weighted_case() {
  log "running weighted-scheduling flash scenario"
  local scenario_dir="$WORKDIR/weighted"
  local body
  body=$(request_body)
  rm -rf "$scenario_dir"
  mkdir -p "$scenario_dir"

  relay_request "$SHARED_B_BLOCK" "$scenario_dir/blocker1.body" "$scenario_dir/blocker1.meta" "$body"
  relay_request "$SHARED_B_BLOCK" "$scenario_dir/blocker2.body" "$scenario_dir/blocker2.meta" "$body"

  for name in high1 high2 low1 low2; do
    local token
    case "$name" in
      high1) token=$SHARED_B_HIGH1 ;;
      high2) token=$SHARED_B_HIGH2 ;;
      low1) token=$SHARED_B_LOW1 ;;
      low2) token=$SHARED_B_LOW2 ;;
    esac
    (
      local start end
      start=$(date +%s)
      relay_request "$token" "$scenario_dir/${name}.body" "$scenario_dir/${name}.meta" "$body"
      end=$(date +%s)
      printf '%s %s %s\n' "$name" "$start" "$end" > "$scenario_dir/${name}.time"
    ) &
    pids+=("$!")
  done

  capture_queue_hit "$scenario_dir/snapshot.json" "10,1" || fail "did not capture queued snapshot for weighted scenario"

  local pid
  for pid in "${pids[@]}"; do
    wait "$pid"
  done

  python3 - <<'PY' "$scenario_dir"
import json
import os
import sys

scenario_dir = sys.argv[1]
rows = []
for name in ("high1", "high2", "low1", "low2"):
    with open(os.path.join(scenario_dir, f"{name}.meta")) as f:
        status, total = f.read().strip().split()
    if status != "200":
        raise SystemExit(f"weighted {name} returned HTTP {status}")
    rows.append({"name": name, "time_total_sec": float(total)})

high = [row["time_total_sec"] for row in rows if row["name"].startswith("high")]
low = [row["time_total_sec"] for row in rows if row["name"].startswith("low")]
avg_high = sum(high) / len(high)
avg_low = sum(low) / len(low)
if not avg_high < avg_low:
    raise SystemExit(f"weighted average did not favor high priority: high={avg_high} low={avg_low}")

summary = {
    "avg_high_sec": avg_high,
    "avg_low_sec": avg_low,
    "max_high_sec": max(high),
    "max_low_sec": max(low),
}
with open(os.path.join(scenario_dir, "summary.json"), "w") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False))
PY
}

run_combo_case() {
  local case_name=$1
  local token=$2
  local expected_bucket=$3
  log "running combination-priority case $case_name"

  local scenario_dir="$WORKDIR/$case_name"
  local body
  body=$(request_body)
  rm -rf "$scenario_dir"
  mkdir -p "$scenario_dir"

  relay_request "$token" "$scenario_dir/blocker1.body" "$scenario_dir/blocker1.meta" "$body"
  relay_request "$token" "$scenario_dir/blocker2.body" "$scenario_dir/blocker2.meta" "$body"

  (
    local start end
    start=$(date +%s)
    relay_request "$token" "$scenario_dir/queued.body" "$scenario_dir/queued.meta" "$body"
    end=$(date +%s)
    printf '%s %s %s\n' "$case_name" "$start" "$end" > "$scenario_dir/queued.time"
  ) &
  local queued_pid=$!

  capture_queue_hit "$scenario_dir/snapshot.json" "$expected_bucket" || fail "did not capture queued snapshot for $case_name"
  wait "$queued_pid"

  python3 - <<'PY' "$scenario_dir" "$expected_bucket" "$case_name"
import json
import os
import sys

scenario_dir, expected_bucket, case_name = sys.argv[1:4]
with open(os.path.join(scenario_dir, "snapshot.json")) as f:
    snapshot = json.load(f)["data"]
if snapshot["queued"] < 1:
    raise SystemExit(f"{case_name} snapshot queued=0")
if snapshot["buckets"].get(expected_bucket, 0) < 1:
    raise SystemExit(f"{case_name} missing expected bucket {expected_bucket}")
with open(os.path.join(scenario_dir, "queued.meta")) as f:
    status, total = f.read().strip().split()
if status != "200":
    raise SystemExit(f"{case_name} queued request returned HTTP {status}")
summary = {
    "case": case_name,
    "expected_bucket": int(expected_bucket),
    "observed_bucket": int(expected_bucket),
    "time_total_sec": float(total),
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
    "queue_entry": json.load(open(os.path.join(workdir, "queue-entry", "summary.json"))),
    "weighted": json.load(open(os.path.join(workdir, "weighted", "summary.json"))),
    "combo_high": json.load(open(os.path.join(workdir, "combo-high", "summary.json"))),
    "combo_mid": json.load(open(os.path.join(workdir, "combo-mid", "summary.json"))),
    "combo_low": json.load(open(os.path.join(workdir, "combo-low", "summary.json"))),
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
  apply_test_settings
  provision_shared_bundles
  provision_combo_bundles
  delete_test_user_cache
  smoke_test
  run_queue_entry_case
  declare -a pids=()
  run_weighted_case
  run_combo_case combo-high "$COMBO_HIGH_TOKEN" 10
  run_combo_case combo-mid "$COMBO_MID_TOKEN" 6
  run_combo_case combo-low "$COMBO_LOW_TOKEN" 1
  log "all flash scenarios passed"
  emit_final_summary
  SCRIPT_SUCCESS=1
}

main "$@"