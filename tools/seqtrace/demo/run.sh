#!/usr/bin/env bash
# Trace a networked rela deployment and draw its sequence diagrams.
#
# Mimics a production deploy: rela-server on the postgres build, a tenant
# pinned to its own schema, identity from a reverse-proxy header, and the
# perf project's acl.yaml (graph-resolved roles, field redaction, worlds).
# A fixed set of requests is sent as three users; each becomes one diagram.
#
# Safe on any checkout: it copies the checkout to .ignored/seqtrace-demo/work
# and builds there, and runs a throwaway postgres: a local cluster when
# initdb and pg_ctl are on PATH, a docker container otherwise. It writes
# nothing outside .ignored/seqtrace-demo. See tools/seqtrace/README.md.
#
# Environment: SEQTRACE_PG_PORT (55439), SEQTRACE_HTTP_PORT (18766),
# SEQTRACE_SCALE (0.05, about 1000 entities), SEQTRACE_KEEP_DB=1 to leave
# postgres running, SEQTRACE_OPEN=0 to not open the result.
#
# SEQTRACE_REF traces a git ref instead of the working tree; the current
# tools/seqtrace and scenarios are used either way, so two runs compare like
# with like. SEQTRACE_LABEL writes to .ignored/seqtrace-demo/LABEL so two runs
# can sit side by side; `just seqtrace-compare` uses both.
set -euo pipefail

REPO=$(pwd)
REF=${SEQTRACE_REF:-}
LABEL=${SEQTRACE_LABEL:-}
[[ $LABEL =~ ^[a-z0-9-]*$ ]] || { echo "seqtrace-demo: SEQTRACE_LABEL must match [a-z0-9-]*" >&2; exit 1; }
OUT="$REPO/.ignored/seqtrace-demo${LABEL:+/$LABEL}"
WORK="$OUT/work"
SRC="$WORK/src"
PG_PORT=${SEQTRACE_PG_PORT:-55439}
HTTP_PORT=${SEQTRACE_HTTP_PORT:-18766}
SCALE=${SEQTRACE_SCALE:-0.05}
# One container per checkout, so runs from two worktrees do not collide.
CTR=rela-seqtrace-pg-$(printf '%s' "$REPO" | cksum | cut -d' ' -f1)
# A fresh password per run: the server listens on TCP, and trust auth would
# let any local user connect as superuser.
PG_PASSWORD=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
export PGPASSWORD=$PG_PASSWORD
SCHEMA=tenant_demo
BASE="http://127.0.0.1:$HTTP_PORT"
DSN="host=127.0.0.1 port=$PG_PORT user=postgres password=$PG_PASSWORD dbname=postgres sslmode=disable search_path=$SCHEMA,public"
SERVER_PID=""

step() { printf '\n==> %s\n' "$*"; }
die() { echo "seqtrace-demo: $*" >&2; exit 1; }

for tool in rsync curl go; do
    command -v "$tool" >/dev/null || die "$tool is required"
done
if command -v initdb >/dev/null && command -v pg_ctl >/dev/null; then
    PG_MODE=local
elif command -v docker >/dev/null && docker info >/dev/null 2>&1; then
    PG_MODE=docker
else
    die "needs postgres: initdb and pg_ctl on PATH, or a running docker"
fi
[[ -f "$REPO/go.mod" && -d "$REPO/tools/seqtrace" ]] || die "run from the repository root (just seqtrace-demo)"

stop_pg() {
    if [[ $PG_MODE == local ]]; then
        [[ -f "$WORK/pgdata/postmaster.pid" ]] && pg_ctl -D "$WORK/pgdata" -m fast stop >/dev/null 2>&1
    else
        docker rm -f "$CTR" >/dev/null 2>&1
    fi
    return 0
}

# psql runs one statement against the throwaway server and prints bare rows.
psql() {
    if [[ $PG_MODE == local ]]; then
        command psql -h 127.0.0.1 -p "$PG_PORT" -U postgres -v ON_ERROR_STOP=1 -tAc "$1"
    else
        docker exec -e PGPASSWORD "$CTR" psql -h 127.0.0.1 -U postgres -v ON_ERROR_STOP=1 -tAc "$1"
    fi
}

cleanup() {
    if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    if [[ "${SEQTRACE_KEEP_DB:-0}" != 1 ]]; then
        stop_pg
    else
        echo "postgres left running on port $PG_PORT; PGPASSWORD=$PG_PASSWORD"
    fi
}
trap cleanup EXIT

mkdir -p "$WORK"
if [[ -n $REF ]]; then
    step "Exporting $REF to $SRC"
    git -C "$REPO" rev-parse --verify --quiet "$REF^{commit}" >/dev/null || die "unknown git ref: $REF"
    rm -rf "$SRC"
    mkdir -p "$SRC"
    git -C "$REPO" archive --format=tar "$REF" | tar -x -C "$SRC"
    rsync -a --delete "$REPO/tools/seqtrace/" "$SRC/tools/seqtrace/"
else
    step "Copying the checkout to $SRC"
    rsync -a --delete \
        --exclude '/.git' --exclude '/.ignored' --exclude '/build' --exclude 'node_modules' \
        "$REPO/" "$SRC/"
fi
# The server refuses to start without the embedded SPA. The diagrams are about
# the API, so a stub replaces a frontend build when the checkout has none.
if [[ ! -f "$SRC/internal/dataentry/static/v2/index.html" ]]; then
    mkdir -p "$SRC/internal/dataentry/static/v2"
    echo '<!doctype html><title>rela (seqtrace stub)</title>' >"$SRC/internal/dataentry/static/v2/index.html"
fi

step "Building the instrumented server (postgres build)"
cd "$SRC"
go run ./tools/seqtrace/cmd/seqtrace overlay -out "$WORK/overlay" -tags postgres ./internal/... ./cmd/...
go build -tags postgres -o "$WORK/rela" ./cmd/rela
go build -tags postgres -overlay "$WORK/overlay/overlay.json" -o "$WORK/rela-server" ./cmd/rela-server

step "Starting a throwaway postgres ($PG_MODE) on port $PG_PORT"
stop_pg
if [[ $PG_MODE == local ]]; then
    rm -rf "$WORK/pgdata"
    (umask 077 && printf '%s\n' "$PG_PASSWORD" >"$WORK/pgpass")
    initdb -D "$WORK/pgdata" -U postgres --auth=scram-sha-256 --pwfile="$WORK/pgpass" >"$WORK/initdb.log"
    rm -f "$WORK/pgpass"
    pg_ctl -D "$WORK/pgdata" -l "$WORK/postgres.log" -w \
        -o "-p $PG_PORT -c listen_addresses=127.0.0.1 -c unix_socket_directories=''" start >/dev/null
else
    docker run -d --name "$CTR" -e POSTGRES_PASSWORD="$PG_PASSWORD" \
        -p "127.0.0.1:$PG_PORT:5432" postgres:17 >/dev/null
fi
# Poll over TCP: during initdb the docker image runs a socket-only server, so
# a TCP answer means the real server is up.
for _ in $(seq 1 60); do
    psql 'select 1' >/dev/null 2>&1 && break
    sleep 1
done
psql 'select 1' >/dev/null || die "postgres did not start"
psql "create schema $SCHEMA" >/dev/null

step "Seeding the perf project at scale $SCALE"
rm -rf "$WORK/project"
cp -R "$SRC/prototypes/perf/project" "$WORK/project"
RELA_DATABASE_URL="$DSN" "$WORK/rela" --project "$WORK/project" dev seed --scale "$SCALE"

pick() { psql "select id from $SCHEMA.entities where $1 order by id limit 1"; }
TASK=$(pick "type = 'task' and properties->>'status' = 'todo'")
RISK=$(pick "type = 'risk'")
PERSON=$(pick "type = 'person' and properties->>'email' <> 'alice@perf.example'")
WORD=$(psql "select split_part(properties->>'title', ' ', 1) from $SCHEMA.entities where id = '$TASK'" | tr -cd '[:alnum:]')
[[ -n "$TASK" && -n "$RISK" && -n "$PERSON" ]] || die "seeded data is missing tasks, risks or people"

step "Starting the traced server on $BASE"
# The server logs "starting server" before it binds, so a busy port would
# send the scenarios, with forged identity headers, to another process.
if curl -s -o /dev/null "$BASE/"; then
    die "port $HTTP_PORT is in use; set SEQTRACE_HTTP_PORT"
fi
rm -f "$WORK/trace.jsonl" "$WORK/names.txt"
RELA_DATABASE_URL="$DSN" \
    SEQTRACE_OUT="$WORK/trace.jsonl" \
    SEQTRACE_ROOT='internal/dataentry\.requestStats\.' \
    "$WORK/rela-server" -project "$WORK/project" -port "$HTTP_PORT" \
    -principal-header X-Forwarded-User >"$WORK/server.log" 2>&1 &
SERVER_PID=$!
# Wait for the log line rather than polling an endpoint, so no request other
# than the scenarios below is traced.
for _ in $(seq 1 600); do
    grep -q "starting server" "$WORK/server.log" && break
    kill -0 "$SERVER_PID" 2>/dev/null || { cat "$WORK/server.log" >&2; die "server exited"; }
    sleep 0.5
done
grep -q "starting server" "$WORK/server.log" || die "server did not start; see $WORK/server.log"

step "Sending scenarios"
mkdir -p "$WORK/responses"
n=0
# req NAME USER METHOD PATH [BODY]: one request, recorded as one diagram.
req() {
    local name=$1 user=$2 method=$3 path=$4 body=${5:-}
    n=$((n + 1))
    echo "$name" >>"$WORK/names.txt"
    local args=(-s -o "$WORK/responses/$(printf %02d $n).json" -w '%{http_code}' -X "$method"
        -H "X-Forwarded-User: $user" -H "Origin: $BASE")
    [[ -n "$body" ]] && args+=(-H 'Content-Type: application/json' -d "$body")
    local code
    code=$(curl "${args[@]}" "$BASE$path") || true
    # Names are matched to traced requests by position: a request that never
    # reached the server would shift every later name.
    [[ $code != 000 ]] || { cat "$WORK/server.log" >&2; die "no response for: $name"; }
    printf '  %-45s %s\n' "$name" "$code"
}
ALICE=alice@perf.example # manager
BOB=bob@perf.example     # editor
CAROL=carol@perf.example # reader

req "read task as manager" "$ALICE" GET "/api/v1/tasks/$TASK"
req "read person as editor, salary redacted" "$BOB" GET "/api/v1/persons/$PERSON"
req "read risk as reader, hidden" "$CAROL" GET "/api/v1/risks/$RISK"
req "list tasks as reader" "$CAROL" GET "/api/v1/tasks?limit=20"
req "search as editor" "$BOB" GET "/api/v1/_search?q=$WORD"
req "update task priority as editor" "$BOB" PATCH "/api/v1/tasks/$TASK" '{"properties":{"priority":"critical"}}'
req "start task (state transition) as editor" "$BOB" PATCH "/api/v1/tasks/$TASK" '{"properties":{"status":"doing"}}'
req "update task as reader, denied" "$CAROL" PATCH "/api/v1/tasks/$TASK" '{"properties":{"priority":"low"}}'
req "create task as editor" "$BOB" POST "/api/v1/tasks" \
    '{"id":"TSK-SEQTRACE","properties":{"title":"Traced task","status":"todo","priority":"low"}}'
req "delete task as editor" "$BOB" DELETE "/api/v1/tasks/TSK-SEQTRACE"

sleep 1 # the runtime flushes every 250 ms
kill -0 "$SERVER_PID" 2>/dev/null || { cat "$WORK/server.log" >&2; die "server exited during the scenarios"; }
kill "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""

step "Drawing diagrams"
rm -rf "$OUT/diagrams"
go run ./tools/seqtrace/cmd/seqtrace diagram -in "$WORK/trace.jsonl" -out "$OUT/diagrams" \
    -names "$WORK/names.txt" -min 0
echo "Responses are in $WORK/responses, the server log in $WORK/server.log."
if [[ "${SEQTRACE_OPEN:-1}" != 0 ]] && command -v open >/dev/null; then
    open "$OUT/diagrams/index.html"
fi
