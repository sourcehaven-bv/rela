---
audience: intermediate
id: GUIDE-scheduled-tasks
order: 11
status: published
summary: Run Lua scripts on recurring schedules
title: Scheduled Tasks
type: guide
---

Rela includes a built-in task scheduler that runs Lua scripts on recurring schedules.
This lets you automate recurring work — reports, validation checks, data cleanup —
without depending on external cron or task scheduling infrastructure.

## Quick Start

### 1. Create a script

Create a Lua script in your project's `scripts/` directory:

```lua
-- scripts/daily-check.lua
local orphans = rela.list_entities("*", "status=draft")
local lines = {}
for _, e in ipairs(orphans) do
    table.insert(lines, "- " .. e.type .. "/" .. e.id .. ": " .. (e.properties.title or "(no title)"))
end
if #lines > 0 then
    local body = "## Draft Entities\n\n" .. table.concat(lines, "\n")
    rela.update_entity("REPORT-daily-check", {
        title = "Daily check: " .. #orphans .. " draft entities",
        status = "open",
    }, body)
end
```

### 2. Define a schedule

Create `schedules.yaml` in your project root:

```yaml
tasks:
  - name: daily-check
    script: daily-check.lua
    every: day
```

### 3. Start the scheduler

```bash
rela scheduler
```

The scheduler runs in the foreground, executing tasks as they become due.
Stop it with Ctrl+C or SIGTERM.

## Configuration

Schedules are defined in `schedules.yaml` in the project root. Each task has a name,
a script path (relative to `scripts/`), and a schedule.

```yaml
tasks:
  - name: daily-report
    script: reports/daily.lua
    every: day

  - name: weekly-review
    script: checks/weekly.lua
    every: friday

  - name: quick-check
    script: checks/orphans.lua
    every: 30m
```

A task may instead name a declarative mail template and expand it once per
recipient. `for_each` is available only on calendar schedules (`day`, `week`, or
a weekday), is bounded to 1,000 entities by default, and may set `limit` up to
10,000:

```yaml
tasks:
  - name: daily-digest
    template: overdue_digest
    every: day
    for_each:
      entity_type: person
      where: ["active = true"]
      limit: 1000
```

`script` and `template` are mutually exclusive. Template tasks require
`for_each`; `run_as` and `for_each` are mutually exclusive because every child
runs as its selected entity's current ACL principal. Expansion posts independent
bounded-retry child jobs and does not wait for their delivery.

### Identity and what a task can read (`run_as`)

A scheduled task runs under an **identity**, and that identity decides what
its script may read. By default every task runs as `system:scheduler`, a
fixed identity that does not depend on which OS account started the
scheduler. `run_as` gives a task its own identity instead:

```yaml
tasks:
  - name: weekly-digest
    script: reports/digest.lua
    every: monday
    run_as: system:digest      # an identity, not a permission
```

**`run_as` grants nothing by itself.** Privileges come from `acl.yaml`, the
same place every other principal's do:

```yaml
# acl.yaml
roles:
  reporting:
    read: [ticket, project]
assignments:
  system:digest: reporting
```

With that pairing, `rela.get_entity` / `list_entities` / `search` /
`get_relations` and the trace bindings inside `digest.lua` return only what
the `reporting` role may see. Anything else is invisible to the script —
which is what bounds the data an AI-assisted or outbound-reporting job can
possibly include.

#### If your project has an `acl.yaml`, grant the scheduler

The default identity is subject to the same rule as any other: it reads
only what a role grants it. A project that has an `acl.yaml` but never
assigns `system:scheduler` a role has tasks that read **nothing** — and
because a gated read is indistinguishable from missing data, they fail
silently rather than erroring.

`rela migrate` adds the grant for you:

```yaml
# acl.yaml
roles:
  scheduler-system:
    read: ["*"]
assignments:
  system:scheduler: scheduler-system
```

Narrow `read:` to the types your jobs actually need, or give each job its
own `run_as` identity with a tighter role — the migration writes a
permissive default so existing jobs keep working, not because wide access
is recommended.

Projects with **no** `acl.yaml` need do nothing: with no policy there is no
access control, and scheduled tasks read the whole graph as they always
have.

Notes:

- **`system:` identities cannot be asserted over the API.** The grant above
  is reachable only by the scheduler process itself. rela reserves the whole
  `system:` namespace at the HTTP boundary: a proxy header, a
  `RELA_DATAENTRY_USER` value, or even a validly signed identity assertion
  naming `system:scheduler` is refused with a 403 and logged, so a wide
  `read: ["*"]` grant here cannot be borrowed by a web or MCP caller. `run_as`
  in this file is unaffected — it is operator-authored config, read
  in-process, and may name any `system:` identity you like.
- **An identity with no assignment reads nothing.** If `run_as` names a
  principal that `acl.yaml` never assigns a role, the task's reads come back
  empty. A typo produces a silently empty job, so check the identity against
  your assignments when a task stops finding data.
- **Both row- and field-level policy apply.** An entity your identity cannot
  read stays invisible, and `visible:` field policy redacts hidden property
  values on the entities it does return — a scheduled task sees the same
  redacted view a person with that identity sees in the UI. (Field redaction
  on this path landed in TKT-0XL8MF; before that, row access was enforced but
  every property of a readable entity came through.)
- Writes are unaffected: they go through the normal ACL, exactly as before.

### Schedule Values

| Value        | Meaning                                                    |
| ------------ | ---------------------------------------------------------- |
| `day`        | Once per day — runs after local midnight                   |
| `monday`     | Once per week on Mondays (after midnight local time)       |
| `friday`     | Once per week on Fridays                                   |
| `week`       | Alias for `monday` — fires on Mondays, not ISO-week change |
| `30m`        | Every 30 minutes                                           |
| `2h`         | Every 2 hours                                              |
| `1h30m`      | Every 90 minutes (any valid Go duration)                   |
| `15`         | Every 15 minutes (bare number = minutes)                   |

All seven weekday names are supported: `monday`, `tuesday`, `wednesday`, `thursday`,
`friday`, `saturday`, `sunday`.

**Day and weekday schedules** check whether the calendar boundary has been crossed since
the last run. They don't fire at a specific clock time — they fire on the first scheduler
tick after the target day begins. This means "every friday" runs as soon as possible after
Friday midnight, regardless of when you start the scheduler.

**Interval schedules** fire when enough time has elapsed since the last run. A `30m` task
that last ran at 9:05 will next run at or after 9:35.

### Task Names

Each task must have a unique name. The name is used to track execution state — if you
rename a task, it will be treated as a new task and execute immediately on next startup.

### Script Paths

Script paths are relative to the `scripts/` directory. Subdirectories are supported:

```yaml
tasks:
  - name: daily-report
    script: reports/daily.lua       # scripts/reports/daily.lua
  - name: cleanup
    script: maintenance/cleanup.lua # scripts/maintenance/cleanup.lua
```

## Execution Model

### Sequential Execution

Tasks execute **sequentially** in the order they appear in `schedules.yaml`. If you have
three tasks due at the same time, they run one after another — never in parallel. This
means:

- No race conditions between scripts modifying the same entities
- Predictable resource usage
- Simple mental model — each script sees the results of the previous one

### Workspace Sync

Before each task execution, the scheduler syncs the workspace from disk. This ensures
scripts always see the latest entities and relations, even if files were modified externally
(by another tool, a git pull, or the data entry app).

### Script Capabilities

Scheduled scripts have the same capabilities as `rela script`:

- **Entity CRUD**: `rela.create_entity()`, `rela.update_entity()`, `rela.delete_entity()`
- **Graph queries**: `rela.list_entities()`, `rela.get_relations()`, `rela.trace_from()`, `rela.trace_to()`
- **AI access**: `ai.chat()`, `ai.complete()` (requires `.rela/ai.yaml`)
- **Output**: `rela.output()` (logged to stderr)
- **File writing**: `rela.write_file()` (to the output directory)

See the [Lua Scripting guide](GUIDE-lua-scripting.md) for the full API reference.

## Missed Run Detection

The scheduler records the last successful run time for each task in its run
state (see [Run State](#run-state)). On startup, it checks whether any tasks
missed their scheduled window while the scheduler was not running.

**Example**: You have a daily task. The scheduler was stopped on Monday evening and
restarted on Wednesday morning. On startup, the scheduler detects that Tuesday's run
was missed and executes the task immediately before entering the normal schedule loop.

This applies to all schedule types:

- **Day tasks**: missed if the day changed since the last run
- **Weekday tasks**: missed if the target weekday has occurred since the last run
- **Interval tasks**: missed if more than the interval has elapsed

### First Run

When a task has no recorded history (new task or fresh project), it executes immediately
on startup.

### Run State

Each tick, the scheduler queues every due task as a **run** on the
background-job queue and moves on. It never waits for a run to finish. A worker
executes the run and records its outcome in the run state, so a slow or lost
job cannot block other tasks.

A run moves through these statuses:

| Status      | Meaning                                                  |
| ----------- | -------------------------------------------------------- |
| `queued`    | Created and handed to the queue; no worker has taken it. |
| `running`   | A worker claimed it.                                     |
| `succeeded` | It finished without error.                               |
| `failed`    | It finished with an error; the retry ladder advances.    |
| `abandoned` | Its lease expired before it finished; counted as failed. |

A task has at most one active (`queued` or `running`) run. A task that is due
while its previous run is still active is skipped for that tick, which also
stops a slow task from piling up work.

Every active run carries a **lease**: 30 minutes while queued and 20 minutes
once running. A `for_each` run extends its lease each time a subject starts.
If the lease expires, for example because the process running the job died, the
next tick marks the run `abandoned` and the task retries on the ladder.

Where the run state lives depends on the build:

- **Filesystem, SQLite and desktop builds**: `.rela/scheduler-run-state.json`,
  which is gitignored. If you delete it, all tasks run on the next startup.
- **PostgreSQL build**: the `scheduler_tasks`, `scheduler_runs` and
  `scheduler_run_children` tables in the tenant's schema. Every node sees the
  same runs, so several nodes can run the scheduler safely.

An older `.rela/scheduler-state.json` is imported on first start and then
deleted. Runs that ended more than 14 days ago are pruned, or twice the longest
task period ago if that is longer.

On the filesystem and desktop builds, the job queue is in memory. If the process
exits while a run is queued or running, that job is gone, and the task waits
for the lease to expire before it retries. On the PostgreSQL build the queue is
durable, so a job left by a crashed node is picked up by another.

## Deployment

### Running as a Service

The scheduler is designed to run as a long-lived process. Common deployment options:

**systemd (Linux):**

```ini
[Unit]
Description=Rela Scheduler
After=network.target

[Service]
Type=simple
WorkingDirectory=/path/to/project
ExecStart=/usr/local/bin/rela scheduler
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

**launchd (macOS):**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.rela.scheduler</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/rela</string>
        <string>scheduler</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/project</string>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

### Graceful Shutdown

The scheduler responds to SIGINT (Ctrl+C) and SIGTERM. On receiving a signal, it:

1. Stops checking for new due tasks
2. Stops the job queue, which gives running jobs time to finish
3. Exits cleanly

A run cut off by shutdown is recorded as `failed` and retried on the ladder. A
run still queued at shutdown is marked `abandoned` once its lease expires.

### Logging

All task activity is logged to stderr with structured fields:

```text
level=INFO msg="scheduled task" task=daily-check every=day script=daily-check.lua
level=INFO msg="scheduler started" tasks=1 node=web-1:4182
level=INFO msg="run queued" task=daily-check run_id=5f0c... reason="first run"
level=INFO msg="run started" task=daily-check run_id=5f0c... node=web-1:4182 attempt=1 queue_wait=12ms
level=INFO msg="run finished" task=daily-check run_id=5f0c... status=succeeded duration=45ms
```

Every line about a run carries its `run_id`, so you can follow one run from
queue to outcome, across nodes. `queue_wait` is how long the run waited for a
worker; a growing value means the workers cannot keep up.

Failed runs are logged with the error message, at WARN for the first few
consecutive failures and escalating to ERROR once retries are clearly not
helping. The scheduler continues running: a failed task does not stop other
tasks from executing.

These lines point at a problem:

| Message                                                | Level | Meaning                                                                                     |
| ------------------------------------------------------ | ----- | ------------------------------------------------------------------------------------------- |
| `run abandoned`                                        | ERROR | The run's lease expired before it finished; the worker probably died. The task retries.     |
| `task due but its previous run is still active, skipping` | INFO  | The previous run has not finished. Frequent lines mean the task is slower than its interval. |
| `duplicate delivery skipped, run is no longer queued`  | WARN  | The queue delivered a job twice; the second copy did nothing.                               |
| `late result discarded, run had already ended`         | WARN  | A run failed after it was already marked abandoned; the abandonment already counted it.     |
| `late success recorded, run had already been abandoned` | WARN  | A run succeeded after it was marked abandoned. The task counts as run, so it does not repeat. |
| `child skipped, subject already settled or its run has ended` | WARN | A `for_each` subject was delivered again, or its run was replaced by a retry. It did not run twice. |
| `could not create run` / `could not record run outcome` | ERROR | The run state could not be written, usually a database problem.                             |

## Failure Handling and Retries

When a task fails, it is retried on a fixed backoff ladder:

```text
5m → 10m → 20m → 40m → 80m → every 2h
```

Each consecutive failure moves one rung down; once the ladder reaches 2 hours it
stays there, retrying every 2 hours until the task succeeds.

**While a task is failing, the ladder replaces its schedule.** The task fires
only on retry steps, never on its normal cadence. This means the ladder is the
same for every schedule, but its effect differs:

- A **daily** task that fails at 09:00 retries at 09:05, 09:15, 09:35, 10:15,
  11:35, then every 2 hours — recovering from an intermittent failure without
  waiting a full day.
- A **`5m`** task that fails **slows down** to the same ladder instead of
  hammering every 5 minutes while it is broken.

A successful run is the only thing that resets the ladder. Elapsed scheduled
slots do not: for a short-interval task a slot passes faster than the ladder
climbs, so resetting on slots would prevent it from ever backing off.

Note that a retry *is* the run for that period, not an extra one. A daily task
that fails at 09:00 and succeeds on the 11:35 retry has run for that day, and
will not run again until the next day — so a recovered run can land some hours
after its nominal slot.

Retry state is kept in the run state alongside the last-run timestamps, so a
task mid-backoff keeps its position across a scheduler restart.

```text
level=WARN msg="run finished" task=daily-check run_id=... status=failed duration=4ms \
  failures=1 retry_at=2026-08-13T23:12:04+02:00 error="..."
level=INFO msg="run queued" task=daily-check run_id=... reason=retry
level=ERROR msg="run finished" task=daily-check run_id=... status=failed duration=4ms \
  failures=4 retry_at=2026-08-14T00:15:00+02:00 error="..."
```

If the scheduler finds a retry time further out than the 2-hour maximum — a
symptom of a clock jump or a hand-edited state file — it logs a WARN and retries
immediately rather than leaving the task stuck indefinitely.

## Examples

### Daily Orphan Report

```yaml
# schedules.yaml
tasks:
  - name: orphan-check
    script: checks/orphans.lua
    every: day
```

```lua
-- scripts/checks/orphans.lua
local entities = rela.list_entities()
local lines = {}
for _, e in ipairs(entities) do
    -- Note the table: get_relations takes an options table, not a bare id.
    -- rela.get_relations(e.id) ignores the argument and returns EVERY
    -- relation, so `#rels == 0` would never fire and the check would
    -- silently pass forever.
    local rels = rela.get_relations({ from = e.id })
    if #rels == 0 then
        table.insert(lines, "- **" .. e.id .. "**: " .. (e.properties.title or "(no title)"))
    end
end

local body = "## Orphan Report\n\n"
if #lines > 0 then
    body = body .. "Found " .. #lines .. " unlinked entities:\n\n" .. table.concat(lines, "\n")
else
    body = body .. "No orphaned entities found."
end

rela.update_entity("REPORT-orphans", {
    title = "Orphan report: " .. #lines .. " unlinked",
    status = #lines > 0 and "open" or "closed",
}, body)
```

### Periodic Status Summary

```yaml
tasks:
  - name: status-summary
    script: reports/status.lua
    every: 4h
```

```lua
-- scripts/reports/status.lua
local types = {"requirement", "decision", "ticket"}
local lines = {}
for _, t in ipairs(types) do
    local all = rela.list_entities(t)
    table.insert(lines, "- **" .. t .. "**: " .. #all .. " entities")
end

rela.update_entity("REPORT-status", {
    title = "Status summary",
    date = os.date("%Y-%m-%d"),
}, "## Status Summary\n\n" .. table.concat(lines, "\n"))
```

### Weekly Traceability Check

```yaml
tasks:
  - name: trace-check
    script: checks/traceability.lua
    every: week
```

```lua
-- scripts/checks/traceability.lua
local reqs = rela.list_entities("requirement")
local gaps = {}
for _, req in ipairs(reqs) do
    local traces = rela.trace_from(req.id, "implements")
    if #traces == 0 then
        table.insert(gaps, "- **" .. req.id .. "**: " .. (req.properties.title or ""))
    end
end

local body = "## Traceability Gaps\n\n"
if #gaps > 0 then
    body = body .. #gaps .. " requirements without implementations:\n\n" .. table.concat(gaps, "\n")
else
    body = body .. "All requirements have implementations."
end

rela.update_entity("REPORT-traceability", {
    title = "Traceability: " .. #gaps .. " gaps",
    status = #gaps > 0 and "open" or "closed",
}, body)
```

## Audit log

Every write a scheduled task performs (via `rela.create_entity`,
`rela.update_entity`, etc.) is recorded in
`.rela/audit/YYYY-MM-DD.jsonl` with `principal.tool: "scheduler"` and
`triggered_by: "schedule:<task-name>"`. This makes it easy to filter
the audit log for scheduler-driven changes:

```bash
cat .rela/audit/*.jsonl | jq 'select(.triggered_by == "schedule:traceability-report")'
```

`principal.user` is the task's identity: `system:scheduler` by default, or
its `run_as` value. Earlier versions recorded the OS account that started
the scheduler, so records written before the upgrade carry that instead —
worth knowing when reading back across the change.

See [audit-log.md](audit-log.md) for the full record schema.
