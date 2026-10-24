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
    # Through a file, not a pipe: bsdtar stops reading at the end-of-archive
    # marker, so git archive gets SIGPIPE and pipefail aborts the script.
    git -C "$REPO" archive --format=tar -o "$WORK/src.tar" "$REF"
    tar -x -f "$WORK/src.tar" -C "$SRC"
    rm -f "$WORK/src.tar"
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
if [[ ${SEQTRACE_SUITE:-} == routes ]]; then
    # The perf project has no command, action or transform. The routes suite
    # needs one of each, so their permission gates run in the trace.
    cat >>"$WORK/project/data-entry.yaml" <<'YAML'
commands:
  seqtrace-echo:
    label: "Echo"
    script: "cat >/dev/null; echo done"
    context: global
    permission: publish-policy
actions:
  seqtrace-touch:
    label: "Touch"
    permission: publish-policy
    set:
      priority: low
YAML
    cat >>"$WORK/project/schema.yaml" <<'YAML'
transforms:
  txt:
    command: ["cp", "{in}", "{out}"]
    produces: text/plain
YAML
fi
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
# USER - sends no identity header.
req() {
    local name=$1 user=$2 method=$3 path=$4 body=${5:-}
    n=$((n + 1))
    echo "$name" >>"$WORK/names.txt"
    local args=(-s -o "$WORK/responses/$(printf %02d $n).json" -w '%{http_code}' -X "$method" -H "Origin: $BASE")
    # REQ_MAX_TIME ends a streaming response (SSE) after that many seconds.
    [[ -n ${REQ_MAX_TIME:-} ]] && args+=(--max-time "$REQ_MAX_TIME")
    [[ $user != - ]] && args+=(-H "X-Forwarded-User: $user")
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
req "create task as reader, denied" "$CAROL" POST "/api/v1/tasks" \
    '{"id":"TSK-SEQTRACE-DENIED","properties":{"title":"Denied task","status":"todo","priority":"low"}}'
req "create task as editor" "$BOB" POST "/api/v1/tasks" \
    '{"id":"TSK-SEQTRACE","properties":{"title":"Traced task","status":"todo","priority":"low"}}'
req "delete task as editor" "$BOB" DELETE "/api/v1/tasks/TSK-SEQTRACE"

# SEQTRACE_SUITE=routes adds one request per route family, so a trace shows
# the identity and access checks on each: reads as the reader, writes the
# reader may not make, requests without an identity header, and the routes
# outside /api/. The labels say what should happen ("denied"); the trace
# shows what did. It runs against the throwaway demo database.
if [[ ${SEQTRACE_SUITE:-} == routes ]]; then
    for p in _me _schema _config _sidebar _nav_status _nav_items _dashboard _conflicts _commands _transforms \
        _piles _settings _palette _theme/export _git/status _copies _leases _openapi.json _feeds/ _documents/ \
        _templates/ _views/ _apps/ _next_action "_comments/tasks/$TASK" "_history/tasks/$TASK"; do
        req "route GET /api/v1/$p as reader" "$CAROL" GET "/api/v1/$p"
    done
    req "route GET /api/help/task as reader" "$CAROL" GET "/api/help/task"
    req "route GET /api/v1/_configure as reader" "$CAROL" GET "/api/v1/_configure"
    req "route PUT /api/v1/_settings as reader, denied" "$CAROL" PUT "/api/v1/_settings" '{}'
    req "route POST /api/v1/_git/sync as reader, denied" "$CAROL" POST "/api/v1/_git/sync" '{}'
    req "route POST /api/command/x as reader, denied" "$CAROL" POST "/api/command/x" '{}'
    req "route DELETE task as reader, denied" "$CAROL" DELETE "/api/v1/tasks/$TASK"
    req "route GET task without identity" - GET "/api/v1/tasks/$TASK"
    req "route GET /api/v1/_me without identity" - GET "/api/v1/_me"
    req "route PATCH task without identity" - PATCH "/api/v1/tasks/$TASK" '{"properties":{"priority":"low"}}'
    # Entity sub-resources and writes, as the reader and, where the reader is
    # refused, as a role that may.
    rel="/api/v1/tasks/$TASK/relations/assigned-to"
    req "route GET task relations as reader" "$CAROL" GET "/api/v1/tasks/$TASK/relations"
    req "route POST relation as reader, denied" "$CAROL" POST "$rel" "{\"id\":\"$PERSON\"}"
    req "route POST relation as editor" "$BOB" POST "$rel" "{\"id\":\"$PERSON\"}"
    req "route PATCH relation as reader, denied" "$CAROL" PATCH "$rel/$PERSON" '{"meta":{}}'
    req "route DELETE relation as reader, denied" "$CAROL" DELETE "$rel/$PERSON"
    req "route DELETE relation as editor" "$BOB" DELETE "$rel/$PERSON"
    req "route POST clone task as reader, denied" "$CAROL" POST "/api/v1/tasks/$TASK/_actions/clone" '{}'
    req "route POST restore task as reader, denied" "$CAROL" POST "/api/v1/tasks/$TASK/restore" '{}'
    req "route POST dry-run create as reader" "$CAROL" POST "/api/v1/tasks?dry_run=true" \
        '{"properties":{"title":"Dry run","status":"todo","priority":"low"}}'
    req "route GET export task as reader" "$CAROL" GET "/api/v1/tasks/$TASK/_export?transform=txt"
    req "route GET export risk as reader, hidden" "$CAROL" GET "/api/v1/risks/$RISK/_export?transform=txt"
    req "route GET export task list as reader" "$CAROL" GET "/api/v1/tasks/_export?transform=txt"
    req "route POST lease as reader, denied" "$CAROL" POST "/api/v1/tasks/$TASK/_lease" '{}'
    req "route POST lease as editor" "$BOB" POST "/api/v1/tasks/$TASK/_lease" '{}'
    req "route GET attachment as reader" "$CAROL" GET "/api/v1/tasks/$TASK/_attachments/file/x.txt"
    req "route PUT attachment as reader, denied" "$CAROL" PUT "/api/v1/tasks/$TASK/_attachments/file" '{}'
    req "route DELETE attachment as reader, denied" "$CAROL" DELETE "/api/v1/tasks/$TASK/_attachments/file/x.txt"
    req "route GET gantt as reader" "$CAROL" GET "/api/v1/_gantts/delivery"
    req "route GET analyze as reader" "$CAROL" GET "/api/v1/_analyze"
    req "route POST conflict resolve as reader, denied" "$CAROL" POST "/api/v1/_conflicts/resolve" '{}'
    req "route POST action as reader, denied" "$CAROL" POST "/api/v1/_action/seqtrace-touch" "{\"entity_id\":\"$TASK\"}"
    req "route POST action as manager" "$ALICE" POST "/api/v1/_action/seqtrace-touch" "{\"entity_id\":\"$TASK\"}"
    req "route POST entity action as reader, denied" "$CAROL" POST "/api/v1/tasks/$TASK/_actions/seqtrace-touch" '{}'
    req "route POST entity action as manager" "$ALICE" POST "/api/v1/tasks/$TASK/_actions/seqtrace-touch" '{}'
    req "route POST command as reader, denied" "$CAROL" POST "/api/command/seqtrace-echo" '{}'
    req "route POST command as manager" "$ALICE" POST "/api/command/seqtrace-echo" '{}'
    req "route POST command cancel as reader" "$CAROL" POST "/api/command-cancel/x" '{}'
    req "route GET command file as reader" "$CAROL" GET "/api/command-file/x"
    REQ_MAX_TIME=1 req "route GET /api/v1/_events as reader" "$CAROL" GET "/api/v1/_events"
    # Project-wide appearance: the reader should be refused.
    req "route PUT /api/v1/_palette as reader, denied" "$CAROL" PUT "/api/v1/_palette" '{}'
    req "route PUT /api/v1/_theme/logo as reader, denied" "$CAROL" PUT "/api/v1/_theme/logo" '{}'
    req "route DELETE /api/v1/_theme/logo as reader, denied" "$CAROL" DELETE "/api/v1/_theme/logo"
    req "route POST /api/v1/_theme/import as reader, denied" "$CAROL" POST "/api/v1/_theme/import" '{}'
    req "route GET / (SPA shell)" - GET "/"
    req "route GET /_custom/custom.css" - GET "/_custom/custom.css"
    req "route GET /static/missing" - GET "/static/missing"
fi

sleep 1 # the runtime flushes every 250 ms
kill -0 "$SERVER_PID" 2>/dev/null || { cat "$WORK/server.log" >&2; die "server exited during the scenarios"; }
kill "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""

step "Drawing diagrams"
rm -rf "$OUT/diagrams"
# Draw the declared guards even inside the handler's package, so archplan sees
# every request that passes one.
keep=$(tools/seqtrace/guards.sh)
go run ./tools/seqtrace/cmd/seqtrace diagram -in "$WORK/trace.jsonl" -out "$OUT/diagrams" \
    -names "$WORK/names.txt" -min 0 ${keep:+-keep "$keep"}
echo "Responses are in $WORK/responses, the server log in $WORK/server.log."
if [[ "${SEQTRACE_OPEN:-1}" != 0 ]] && command -v open >/dev/null; then
    open "$OUT/diagrams/index.html"
fi
