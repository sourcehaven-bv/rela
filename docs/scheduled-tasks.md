<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Scheduled Tasks

Rela includes a built-in task scheduler that runs Lua scripts on recurring schedules.
This lets you automate recurring work — reports, validation checks, data cleanup —
without depending on external cron or task scheduling infrastructure.

## Quick Start

### 1. Create a script

Create a Lua script in your project's `scripts/` directory:

```lua
-- scripts/daily-check.lua
local orphans = rela.list_entities("*", "status=draft")
if #orphans > 0 then
    rela.output("Found " .. #orphans .. " draft entities")
    for _, e in ipairs(orphans) do
        rela.output("  - " .. e.id .. ": " .. (e.properties.title or "(no title)"))
    end
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

### Schedule Values

| Value        | Meaning                                                    |
| ------------ | ---------------------------------------------------------- |
| `day`        | Once per day — runs after local midnight                   |
| `monday`     | Once per week on Mondays (after midnight local time)       |
| `friday`     | Once per week on Fridays                                   |
| `week`       | Alias for `monday`                                         |
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

The scheduler tracks the last successful run time for each task in
`.rela/scheduler-state.json`. On startup, it checks whether any tasks missed their
scheduled window while the scheduler was not running.

**Example**: You have a daily task. The scheduler was stopped on Monday evening and
restarted on Wednesday morning. On startup, the scheduler detects that Tuesday's run
was missed and executes the task immediately before entering the normal schedule loop.

This applies to all schedule types:

- **Day tasks**: missed if the day changed since the last run
- **Week tasks**: missed if the ISO week changed since the last run
- **Interval tasks**: missed if more than the interval has elapsed

### First Run

When a task has no recorded history (new task or fresh project), it executes immediately
on startup.

### State File

The state file `.rela/scheduler-state.json` is gitignored. Each developer or deployment
maintains its own scheduler state. If you delete this file, all tasks will execute on
the next startup.

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
2. Waits for any currently-running task to finish
3. Exits cleanly

### Logging

All task activity is logged to stderr with structured fields:

```text
level=INFO msg="scheduled task" name=daily-check every=day script=daily-check.lua
level=INFO msg="first run, executing immediately" name=daily-check
level=INFO msg="task started" name=daily-check script=daily-check.lua
level=INFO msg="task completed" name=daily-check duration=45.2ms
level=INFO msg="scheduler started" tasks=1
```

Failed tasks are logged at ERROR level with the error message. The scheduler continues
running — a failed task does not stop other tasks from executing.

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
local orphans = {}
for _, e in ipairs(entities) do
    local rels = rela.get_relations(e.id)
    if #rels == 0 then
        table.insert(orphans, e)
    end
end
if #orphans > 0 then
    rela.output("Orphaned entities: " .. #orphans)
    for _, e in ipairs(orphans) do
        rela.output("  " .. e.type .. "/" .. e.id .. ": " .. (e.properties.title or ""))
    end
end
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
local summary = {}
for _, t in ipairs(types) do
    local all = rela.list_entities(t)
    summary[t] = #all
end
rela.output(summary)
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
local unlinked = 0
for _, req in ipairs(reqs) do
    local traces = rela.trace_from(req.id, "implements")
    if #traces == 0 then
        rela.output("WARNING: " .. req.id .. " has no implementations")
        unlinked = unlinked + 1
    end
end
if unlinked > 0 then
    rela.output(unlinked .. " requirements without implementations")
end
```
