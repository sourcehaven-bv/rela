<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Audit Log

rela writes one append-only JSONL record per entity / relation
create / update / delete to `.rela/audit/YYYY-MM-DD.jsonl`. The log
exists to answer **what changed, when, and on whose behalf** —
forensic, not authoritative.

## Where records land

- **Path:** `.rela/audit/YYYY-MM-DD.jsonl` (UTC date in the filename).
- **Rotation:** daily, on the first record written after UTC midnight.
- **Modes:** dir `0o700`, files `0o600` (owner-only).
- **Symlink-refused:** if `.rela/audit/` is a symlink, the backend
  enters a 60-second cool-down and logs `audit.write_failed`. Entity
  writes still succeed; audit retries when the cool-down expires.
- **`.rela/audit/` is gitignored** by convention; the audit log is
  per-machine state, not part of the repository.

## Record shape

Every record is one line of JSON:

```json
{
  "time": "2026-05-17T08:30:00Z",
  "op": "create-entity",
  "subject": {
    "kind": "entity",
    "type": "ticket",
    "id": "TKT-001"
  },
  "principal": {
    "user": "alice",
    "tool": "cli"
  },
  "triggered_by": "",
  "summary": "created"
}
```

### Fields

| Field         | Meaning                                                             |
|---------------|---------------------------------------------------------------------|
| `time`        | UTC timestamp                                                        |
| `op`          | `create-entity`, `update-entity`, `delete-entity`, `rename-entity`, `create-relation`, `update-relation`, `delete-relation` |
| `subject`     | The thing acted on (see "Subject shape" below)                       |
| `before` / `after` | For `rename-entity` only — the identity diff                   |
| `principal.user` | The OS user (from `$USER`) that initiated the operation           |
| `principal.tool` | `cli`, `mcp`, `data-entry`, `scheduler`, or `desktop`             |
| `triggered_by` | Optional. Engine-initiated writes carry `automation:<name>`, `schedule:<task-name>`, or `cascade:delete-entity:<id>` |
| `summary`     | One-line human-readable summary; for updates names *which* properties changed but never their values (secret-leak defense) |

### Subject shape

- **Entity ops:** `{"kind":"entity", "type":..., "id":...}`
- **Relation ops:** `{"kind":"relation", "relation_type":..., "from_id":..., "to_id":...}`
- **Rename:** `subject` is absent from the JSON; `before` and `after`
  carry the entity identity diff.

## Principal — who's "behind" a write

`principal.user` is best-effort: rela captures `$USER` at process
startup and stamps it on every audit record. If `$USER` is unset,
`user` becomes `"unknown"`.

`principal.tool` identifies the entry point:

| Tool          | When                                                     |
|---------------|----------------------------------------------------------|
| `cli`         | Any `rela` subcommand (except `rela mcp` and the data-entry server) |
| `mcp`         | An MCP client calling tools over stdio                   |
| `data-entry`  | A request to the data-entry HTTP server                  |
| `scheduler`   | The background scheduler running a Lua task              |
| `desktop`     | The rela-desktop Wails app                               |

### `data-entry` user attribution

The data-entry server stamps `Principal.User` from one of these
sources, in order:

1. `$RELA_DATAENTRY_USER` (process-wide env override — local-dev
   escape hatch).
2. The HTTP header named by `--principal-header` on `rela-server`
   (typically `X-Forwarded-User`, set by an SSO reverse proxy like
   oauth2-proxy / Vouch / traefik forward-auth).
3. `"unknown"` when neither is set or both resolve to an empty
   value.

Recording the server process owner (e.g. `www-data`) for every
edit by every human web user would be actively misleading, so a
direct, unproxied deployment that hasn't configured either source
records `"unknown"` — honest about the gap.

**Trust boundary**: enabling `--principal-header` only makes sense
behind a reverse proxy that *strips the same header from inbound
requests* and *sets it from an authenticated source*. A direct
client can otherwise spoof the header at will. See
[`docs/security.md`](../security.md) for deployment guidance.

Header values are trimmed, length-capped at 256 runes, and have
control characters replaced with a space — defense-in-depth against
header-injection corrupting the JSONL stream.

### `mcp` user is the host process owner

MCP records the OS user that launched `rela mcp ...`, not the LLM
agent making calls. MCP's wire protocol has no notion of "user", and
the operator who started the server is the right grain for forensics.

## `triggered_by` — engine-initiated writes

When a write is caused by an automation cascade or scheduler task,
`triggered_by` distinguishes it from direct user actions:

- `automation:<name>` — a scripted automation action.
- `automation` — a non-scripted automation action (relation create,
  cascaded entity create). Generic label by design — the engine
  doesn't currently thread the originating automation name through
  these paths.
- `schedule:<task-name>` — a scheduler-driven Lua task.
- `cascade:delete-entity:<id>` — a relation deleted as a side effect
  of `delete-entity` with `cascade=true`.

## Known gaps

### Crash window

There's a window between a successful store write and the audit
record append. A process crash in between leaves the mutation on disk
with no audit row. The trade-off: closing the window by writing the
audit record *before* the store mutation introduces false-positive
rows (audit entries for writes that never landed) — a worse failure
mode for the forensic use case. Audit is forensic, not authoritative;
the store is the source of truth.

If the gap is unacceptable for your operational model, consider:

- Mounting `.rela/audit/` on a filesystem with `sync` enabled (or set
  `O_SYNC` via a fork — not built in).
- Forwarding records to an external append-only sink in a future
  phase.

### Per-automation attribution for cascaded relations / entities

When an `on: created` automation fires `create_relation` or
`create_entity` actions, the resulting records carry `triggered_by:
"automation"` (generic) rather than `automation:<originating-name>`.
The automation engine's Result type doesn't carry the per-action
originating-name today. Scripted actions (`lua: |` blocks) do carry
the specific name as `automation:<name>`.

### Retention

There is no automatic log rotation / pruning. Operators are
responsible for managing the directory:

```bash
find .rela/audit -mtime +90 -delete
```

Daily file naming makes retention granularity trivial.

## Reading the log

`jq` is the easiest tool for reading JSONL streams. Common queries:

```bash
# Who changed entity TKT-001 today?
jq 'select(.subject.id == "TKT-001")' .rela/audit/$(date -u +%Y-%m-%d).jsonl

# All scheduler-driven writes in the last week
cat .rela/audit/*.jsonl | jq 'select(.principal.tool == "scheduler")'

# All automation cascades for entity TKT-007
cat .rela/audit/*.jsonl | jq 'select(.triggered_by | startswith("automation")
  and (.subject.id == "TKT-007" or .before.id == "TKT-007"))'

# Count writes per user in May 2026
cat .rela/audit/2026-05-*.jsonl | jq -r '.principal.user' | sort | uniq -c | sort -rn
```

## Security considerations

- The log records entity IDs and property *names* (never values).
  Property names are not secrets in rela's model — entities are
  markdown files in the project tree, names are visible to anyone
  with repo read access.
- Audit failures (disk full, permission errors, symlinked dir) never
  block a legitimate entity write. The backend logs
  `audit.write_failed` via slog, enters a 60-second cool-down, and
  retries on the next record.
- `.rela/audit/` should remain on the same filesystem as the project
  — moving it elsewhere isn't supported and the path isn't
  configurable.
