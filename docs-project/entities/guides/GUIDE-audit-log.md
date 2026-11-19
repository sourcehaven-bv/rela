---
audience: intermediate
id: GUIDE-audit-log
order: 12
status: published
summary: Forensic log of every entity / relation write, as JSONL files or a hash-chained database table
title: Audit Log
type: guide
---

rela writes one append-only record per entity / relation create /
update / delete. The log exists to answer **what changed, when, and on
whose behalf**.

Where the records go depends on the build:

- The default (filesystem) build writes JSONL files to
  `.rela/audit/YYYY-MM-DD.jsonl`.
- The SQLite and PostgreSQL builds write to the `audit_log` table in
  the project's database, as a hash chain. See "Database audit log"
  below.

Both write the same record shape.

## Where records land

- **Path:** `.rela/audit/YYYY-MM-DD.jsonl` (UTC date in the filename).
- **Rotation:** daily, on the first record written after UTC midnight.
- **Modes:** dir `0o700`, files `0o600` (owner-only).
- **Symlink-refused:** if `.rela/audit/` is a symlink, the backend
  enters a 60-second cool-down and logs `audit.write_failed`. Entity
  writes still succeed; audit retries when the cool-down expires.
- **`.rela/audit/` is gitignored** by convention; the audit log is
  per-machine state, not part of the repository.

## Database audit log

On the SQLite and PostgreSQL builds the records go to the database
instead of `.rela/audit/`. On PostgreSQL the tables live in the store's
schema, so each tenant has its own log.

| Table | Contents |
|-------|----------|
| `audit_log` | One row per record: `seq`, append time `at` (microseconds since the epoch), `op`, `principal_user`, the record as JSON in `doc`, `prev_hash` and `hash` |
| `audit_subjects` | The entity IDs each row names (subject, before, after, relation endpoints), for lookup by entity |
| `audit_anchor` | The last pruned row's `seq` and `hash`, after a prune |

### Hash chain

`seq` has no gaps. Each row's hash covers the previous row's hash and
every other column of the row:

```text
hash = sha256_hex(prev_hash "\n" seq "\n" at "\n"
                  len(op) ":" op len(principal_user) ":" principal_user doc)
```

Numbers are written in decimal; the lengths are byte counts.

The first row links to the empty string, or to the anchor after a
prune. Appends take a lock on the chain (a transaction-scoped advisory
lock per schema on PostgreSQL; SQLite has one writer), so several
`rela-server` processes on one database still form one chain.

A prune sets the anchor and appends an `audit-prune` row that names it.
The anchor is only valid when the newest `audit-prune` row names it.

`rela audit verify` walks the chain and reports the first row that
does not check out: a missing row, a changed row, a broken link, or an
anchor that no prune set. It prints the head (`seq` and `hash`) when the
chain verifies. `--expect-head` also checks that the chain still passes
through a head you recorded earlier:

```bash
rela audit verify
rela -o json audit verify   # ok, rows, anchor_seq, head_seq, head_hash
rela audit verify --expect-head 1042:9f86d08...
```

### What the chain does and does not protect

- It detects an edit, deletion or insertion of a row, including deleting
  the oldest rows and setting the anchor by hand, by anyone who does not
  also recompute every later hash.
- It does **not** stop someone with write access to the database from
  rewriting the whole chain from some row onward, or from deleting the
  newest rows. To detect that, record the head that `rela audit verify`
  prints somewhere the database user cannot write, for example a
  scheduled job that sends it to a separate system. Later, run
  `rela audit verify --expect-head SEQ:HASH` with the recorded head; it
  fails when the chain no longer contains that row with that hash. A
  prune past the recorded `seq` makes the check impossible, so record a
  new head after each prune.
- `rela audit prune` verifies the rows it would delete and refuses when
  one does not check out, so a prune cannot erase evidence of an edit.
- Database roles are the first line of defence, but only when
  `rela-server` connects as a role that does **not** own the tables. The
  owner, normally the role that ran `rela db migrate`, can always delete
  or rewrite rows. Give the server role `SELECT, INSERT` on `audit_log`
  and `audit_subjects` and `SELECT` on `audit_anchor`. Run
  `rela audit prune` as a separate role that also has `DELETE` on
  `audit_log` and `audit_subjects` and `INSERT, UPDATE` on
  `audit_anchor`. For example, with `rela_app` as the server role:

  ```sql
  REVOKE ALL ON audit_log, audit_subjects, audit_anchor FROM rela_app;
  GRANT SELECT, INSERT ON audit_log, audit_subjects TO rela_app;
  GRANT SELECT ON audit_anchor TO rela_app;
  ```

### Delivery

The record of an entity or relation write is appended inside the write's
own database transaction. The write and its record commit together. If
the append fails, the write fails and is rolled back, so a committed
write always has its record. This covers creates, updates, deletes
(soft and permanent), restores, purges, renames, conversions, moves and
copies, and the entities and relations that automations and cascades
write. A copy writes one `copy-state` record for the new entity; the
edges it copies have no records of their own.

The properties an on-create automation sets on the entity it was
triggered by are written after the create and have no record of their
own. The create's record shows the entity as the caller sent it.

On PostgreSQL every audited write takes the project's write lock, as
transactional writes already did, so writes to one project are
serialized.

Other records describe an event that is not a stored write, such as a
refused write (`denied-write`) or the use of elevated access
(`acl-bypass`). These go through a background writer that appends
queued records in batches. A failed batch is retried for about 20
seconds and then logged as `audit.write_failed` and dropped. The queue
holds at most 100,000 records; records that arrive when it is full are
dropped and counted. On shutdown the writer drains its queue for up to
ten seconds and logs how many records it could not write. A crash loses
the queued records of this second kind only.

### Moving JSONL files into the database

When a project moves from the filesystem build to SQLite or PostgreSQL,
append its old files to the database log:

```bash
rela audit import .rela/audit/
```

A directory is read in file name order, which is date order. Records
keep their original `time`, and their rows are appended at that time,
so retention counts from when the record was written. The rows get new
`seq` numbers. The import runs in one transaction: if a line fails to
decode, nothing is appended and you can run it again. It ends with an
`audit-import` record that names how many records it appended.

- Importing the same files twice appends them twice, so import each
  file once.
- An imported row is only as trustworthy as the file it came from. The
  JSONL files have no integrity protection, so anyone who could write
  to them could have changed a record before the import. The chain
  protects the rows from the import on.
- Records of `audit-prune` and `audit-import` operations are refused:
  only the database log itself may write those.

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
  "summary": "created",
  "source": "cli"
}
```

### Fields

| Field         | Meaning                                                             |
|---------------|---------------------------------------------------------------------|
| `time`        | UTC timestamp                                                        |
| `op`          | See the op list below |
| `subject`     | The thing acted on (see "Subject shape" below)                       |
| `before` / `after` | For rename and conversion — the identity diff                  |
| `principal.user` | The OS user (from `$USER`) that initiated the operation           |
| `principal.tool` | `cli`, `mcp`, `data-entry`, `scheduler`, `desktop`, or `realtime` |
| `principal.raw_user` | Optional. The pre-resolution identifier, when `principal_property` substituted a graph entity ID into `user` |
| `principal.org_id` / `principal.org_slug` | Optional. The tenant from a verified identity assertion. **Attribution only — see the warning below** |
| `principal.roles` | Optional. The `roles` claim from a verified identity assertion |
| `triggered_by` | Optional. Engine-initiated writes carry `automation:<name>`, `schedule:<task-name>`, `session:<channel>/<from>-><to>#<generation>/<index>`, or `cascade:delete-entity:<id>` |
| `summary`     | One-line human-readable summary; for updates names *which* properties changed but never their values (secret-leak defense) |
| `flow` / `run` / `step` | Optional. The flow, the run's id and the step, for a write a flow step made. See "`source` and flow steps" below |
| `by`          | Optional. The user who started or cancelled the run by hand, for a write a flow step made on that request |
| `source`      | The kind of actor that produced the record: `flow`, `automation`, `schedule`, `cascade`, `restore`, `renumber`, `rename`, `convert` or `channel`, else the principal's tool |

### Operations

| `op` | Meaning |
|------|---------|
| `create-entity`, `update-entity`, `delete-entity`, `rename-entity`, `convert-entity` | Entity writes, including type conversion |
| `restore-entity`, `purge-deleted-entity` | Undo of a web-app delete, and the final removal once the undo window has passed. See "Undoable deletes" below |
| `create-relation`, `update-relation`, `delete-relation` | Relation writes |
| `denied-write` | A write the ACL refused, or an upload the attachment policy refused. Subject names the would-be target; the summary carries the rule that fired |
| `acl-bypass`, `acl-bypass-read` | A write or read that skipped the ACL through an elevated script handle (`rela.bypass_acl`). The principal is the REAL triggering identity |
| `acl-query` | An effective-access query (`rela acl who-can`) — who asked, when, and about which entity. **The answer is deliberately not recorded**: it is a list of principals and their access routes, so logging it would duplicate the disclosure rather than record it |
| `purge-version` | An operator hard-delete of version history for compliance redaction. Never contains the purged content |
| `data-migration`, `data-gc` | Schema-driven data migration and garbage collection |
| `audit-prune`, `audit-import` | Maintenance of the database audit log: `rela audit prune` and `rela audit import`. The summary carries the counts and, for a prune, the last deleted `seq` and the cutoff |
| `audit-read` | A page of the global audit view (`GET /api/v1/_audit`) was served. The summary carries the cursor, the page size and the number of records served; the records themselves are not copied |
| `idp-roles-set` | The IdP webhook stored or cleared the asserted roles of one principal, for background jobs (see [ACL security](acl-security.md) "Stored asserted roles"), or the stored roles were erased with the person (delete, rename, `rela acl forget-idp-roles`); an erasure records `roles=0`. Subject is the bound user entity, absent when the subject resolved to none; the summary is `roles=<count>`. The IdP subject and the role names are not recorded |

### Conversion records

A conversion writes one `convert-entity` record per face. Its `before` and
`after` name the old and new id, type and face. Each relation the conversion
removes gets a `delete-relation` record, and each relation it creates gets a
`create-relation` record. Both carry `triggered_by: convert-entity:<old>-><new>`.
This includes relations the caller cannot see. On the filesystem and memory
stores, which cannot roll back, these records are still written when a later
step fails after the rename.

### `denied-write` records

When an ACL refuses a write (see [security](server-security.md)
"Access control"), the audit log records a `denied-write` row with
the would-be `subject` and a `summary` carrying the deny reason.
The record never produces side-effects on the store itself — the
deny short-circuits before any persistence.

Example:

```json
{
  "time": "2026-05-19T20:30:00Z",
  "op": "denied-write",
  "subject": {"kind": "entity", "type": "ticket"},
  "principal": {"user": "alice", "tool": "data-entry"},
  "summary": "denied: no role grants write on type 'ticket' (rule_kind=role-grant rule_id=-) attempted op=create"
}
```

The `summary` always names the **rule that fired** (rule_kind +
rule_id) and the attempted op so forensic queries can answer "what
did this user try to do that they weren't allowed to?".

For relation writes the `subject` carries `relation_type` instead of
`type` — same shape as the corresponding successful relation ops.

### Subject shape

- **Entity ops:** `{"kind":"entity", "type":..., "id":..., "face":...}`
- **Relation ops:** `{"kind":"relation", "relation_type":..., "from_id":..., "from_face":..., "to_id":...}`
- `face` and `from_face` are present only on a type with faces. A relation's
  far end has no face, because a relation leads to the entity, not to one of
  its faces.
- **Rename and conversion:** `subject` is absent from the JSON; `before` and
  `after` carry the old and new entity identities. A conversion preserves the
  entity's history lineage under its new id.

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
| `realtime`    | A session output: a graph write a channel session makes as the person whose message moved it |
| `flow`        | The flow engine, acting as `system:flow`. Only it writes flow runs |

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
[`docs/server-security.md`](server-security.md) for deployment guidance.

### Verified-assertion fields (`org_id`, `org_slug`, `roles`)

When rela verifies a signed identity assertion (the `--jwt-*` flags),
the record additionally carries the assertion's tenant and role claims:

```json
{
  "principal": {
    "user": "usr_abc123", "tool": "data-entry",
    "org_id": "org_acme", "org_slug": "acme", "roles": ["admin"]
  }
}
```

All three are omitted entirely for every other entry point, so records
from a CLI, MCP, or header-mode deployment are byte-identical to what
earlier versions wrote.

> **`org_id` in this log does NOT mean rela enforced tenant isolation.**
> Nothing in the ACL evaluates it — a principal in one org can reach
> every entity its roles grant, in every org. The field records which
> tenant a request came from, not what it was allowed to touch. See
> GUIDE-acl-security for the full rationale.

`roles` records the IdP's role names as asserted, before any policy
mapping — so an investigator can see what the token claimed, separately
from what `acl.yaml` chose to grant it.

Header values are trimmed, length-capped at 256 runes, and have
control characters replaced with a space — defense-in-depth against
header-injection corrupting the JSONL stream.

### `mcp` user is the host process owner

MCP records the OS user that launched `rela mcp ...`, not the LLM
agent making calls. MCP's wire protocol has no notion of "user", and
the operator who started the server is the right grain for forensics.

## `triggered_by` — engine-initiated writes

When a write is caused by a trigger or a scheduler task,
`triggered_by` distinguishes it from direct user actions:

- `automation:<name>` — an output of the `flows.yaml` trigger called
  `<name>` (see [Triggers](flows.md#triggers)). This covers scripts
  (`script:`) and the other outputs alike: a relation, a created entity,
  and the delete half of an `if_exists: replace`. Triggers were called
  automations before, and the label keeps that word, so queries on older
  records still work.
- `automation` — written by older releases for an automation declared
  without a `name:`. Every trigger has a name, so new records always
  carry `automation:<name>`.
- `schedule:<task-name>` — a scheduler-driven Lua task.
- `session:<channel>/<from>-><to>#<generation>/<index>` — output
  `<index>` (from 0) of a channel session's move from `<from>` to `<to>`,
  such as `session:DL-1/breach-response/assessed->notified#3/0`.
  `<channel>` is the channel key: the anchor, the channel type and, for a
  named channel, its name. The record's `principal.user` is the person
  whose message moved the session, with `principal.tool` `realtime` (see
  the realtime guide).
- `cascade:delete-entity:<id>` — a relation deleted as a side effect
  of `delete-entity` with `cascade=true`.
- `restore-entity:<id>` — a relation brought back by `restore-entity`.

**One label per record: the outermost cause wins.** A write can have
more than one true cause — a scheduled task whose write fires a
trigger, for instance — but `triggered_by` is a single string, not a
stack. The enclosing label is kept, so every row a scheduled task
produced (including the ones a trigger cascaded underneath it)
answers `triggered_by == "schedule:<task>"`. Querying "what did last
night's run do?" therefore returns the complete set.

## `source` and flow steps

Every record carries `source`, so a query can tell a person's edit from
an engine's write without parsing `triggered_by`. rela derives it in this
order:

1. `flow` when the write was made in a flow step.
2. The kind of `triggered_by`: the part before the first `:`. The
   mapping is closed: `automation` and `automation-job` give
   `automation`, `schedule` gives `schedule`, `cascade` gives `cascade`,
   `restore-entity` gives `restore`, `renumber` gives `renumber`,
   `rename` and `rename-entity` give `rename`, `convert-entity` gives
   `convert`, and the channel labels (`session`, `channel`, `reminder`,
   `timer`, `trigger`, `auto`) give `channel`.
3. Otherwise the principal's tool, such as `cli`, `mcp` or `data-entry`.

A write a flow step makes also carries `flow` (the flow's name in
flows.yaml), `run` (the run's id) and `step`. A trigger that the
step's write fires runs in the same step, so its records carry the
same three fields and `source: flow`. A background job the step queues
does not: the queue does not carry them. The principal stays the genuine
identity, `system:flow` for the engine's own writes. A write made on a
`rela flow start` or `rela flow cancel` also carries `by`, the user who
asked, because the principal of those writes is the flow's identity.

## Undoable deletes

A delete from the web app can be undone for a short while (see the data
entry guide). It is logged as an ordinary `delete-entity`, with
`delete-relation` rows for its relations, all with the summary
`deleted (undoable)`.

- An undo writes `restore-entity`, plus a `create-relation` row with the
  summary `restored` for each relation that came back.
- When the undo window passes, the server writes `purge-deleted-entity`. Its
  `principal.user` is the user who deleted the entity, and its
  `principal.tool` is `soft-delete-gc`. On backends with version history,
  the delete is recorded in the history at this point, not at the moment of
  the delete, so an undone delete leaves no trace there.

## Known gaps

### Crash window

On the database builds, see "Delivery" above: the window is the
background writer's queue. On the filesystem build, there's a window between a successful store write and the audit
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

### Partial cascade delete on a non-transactional backend

The filesystem backend's transaction tier is a write mutex with no
rollback, so a cascade delete that fails partway leaves the relation
files it had already removed off disk. Those removals are real and
they are audited: the store reports what it removed alongside the
error, and the write path records one `delete-relation` row per
relation, carrying the same `cascade:delete-entity:<id>` label a
successful cascade uses. A partial and a complete cascade are
therefore indistinguishable in the log except by how many rows they
produced.

No `delete-entity` row is written in that case — the entity survives a
failed cascade (the store aborts before touching it), and recording
its deletion would be the opposite error. Over-reporting is as broken
as under-reporting, and harder to notice.

The PostgreSQL and SQLite backends roll back, so a partial cascade
cannot persist there and the question does not arise.

### Retention

**Database builds.** `rela audit prune --keep-days N` deletes the rows
appended more than `N` days ago. The hash of the last deleted row
becomes the anchor, so the rows that remain still verify, and the prune
itself is recorded as an `audit-prune` row. Values below 1095 (three
years, the retention period for audit logs) are refused unless you pass
`--allow-short-retention`. Nothing prunes automatically; schedule the
command if you need it. Prune deletes by age only: before you run it,
check that no open incident or nonconformity, or one still in its
evaluation period, needs older rows, and raise `--keep-days` if one does.

```bash
rela audit prune --keep-days 1100
```

**Filesystem build.** `rela` rotates to a new daily file but **never deletes audit logs** —
there is no automatic pruning or expiry. Retaining the directory is an
operational responsibility of the deployment.

**Minimum retention.** Where a security-log retention requirement
applies, keep everything under `.rela/audit/` for at least that
window. Security logs need **≥ 12 months** (e.g. POLICY-017 §4 /
PROCEDURE-f4cu); audit logs need **three years**, longer while an open
incident or nonconformity needs them. The directory
is gitignored and per-machine, so back it up or ship it off-box if the
host is ephemeral. See the [security model](server-security.md#retention).

**Pruning, if any, must stay above the retention window.** Daily file
naming makes the granularity exact — delete only files older than your
window, never on a shorter interval:

```bash
# Delete audit logs older than three years (1095 days). Do NOT prune
# below your required retention window — a shorter -mtime would drop
# records you are required to keep.
find .rela/audit -name '*.jsonl' -mtime +1095 -delete
```

## Reading the log

### In the web app

On the database builds (PostgreSQL and SQLite), the data-entry app reads
the audit log in two places. The filesystem build keeps JSONL files, which
cannot be paged, so it offers neither.

- **The Changes tab** on an entity page lists the writes to that entity and
  its relations, newest first (`GET /api/v1/_changes/<type>/<id>`). It is
  gated like the entity's [version history](acl-security.md#version-history-read-gating-historyread):
  a live entity needs read access, a deleted one needs `history:read`. On a
  type with faces the list is per face. The list starts at the entity's
  latest creation, so a new entity that reuses a deleted entity's id does not
  show the old one's records. A record that names another entity the reader
  cannot read now (a relation's far end, a rename's other id) is left out.
  The record's summary is kept only when it names no entity, and names only
  the properties the reader sees on every row of the type. An update that
  changed only properties the reader does not see on every row is left out,
  so the list does not tell when a hidden value changed. Records of
  relations rela manages itself (types starting with `_`), changes to `_`
  properties, and the renumbering of siblings after a drag are left out too.
  Only the kind of `triggered_by` is shown, plus the automation or schedule
  name.

  The tab names relations, properties and entity types by their labels, and
  other entities by their title as the reader sees it (the response's
  `titles` map). A change made by a `system:` or `integration:` principal
  shows as "System", and a trigger shows only as "By an automation", "By a
  schedule" or "Automatically". The labels are applied in the browser: the
  response still carries the raw names, which are configuration, not data.
  The global audit view keeps the raw values.

  The tab follows `header_buttons.history` in `data-entry.yaml`: where that
  hides the History button, it hides the Changes tab as well. Use it to keep
  the list from customers, for example with
  `history: {permission: staff.manage}`. Like the button, this is a UX
  filter; the route keeps its own gate.
- **The global audit view** (`/audit`, `GET /api/v1/_audit`) pages through the
  whole log, newest first, including deleted entities and `denied-write`
  records. It needs the `audit:read` permission under an ACL policy; without
  a policy, and under `--read-only`, it is refused. Records are served whole
  and ungated, so treat `audit:read` like `history:read`: grant it only to
  audit and compliance roles. Every page served writes an `audit-read`
  record.

Both routes page with `?limit=` (1 to 200, default 50) and `?before=`, the
`next_before` value of the previous page.

### With SQL

On the database builds, query the tables directly. For example, every
record that names `TKT-001`:

```sql
SELECT l.seq, l.doc FROM audit_log l
JOIN audit_subjects s ON s.seq = l.seq
WHERE s.entity_id = 'TKT-001' ORDER BY l.seq;
```

`audit_subjects` also has `face` (empty when the record names every face)
and `starts`, which marks the record that created the id or renamed or
converted an entity into it.

### With jq

On the filesystem build, `jq` is the easiest tool for reading JSONL streams. Common queries:

```bash
# Who changed entity TKT-001 today?
jq 'select(.subject.id == "TKT-001")' .rela/audit/$(date -u +%Y-%m-%d).jsonl

# All scheduler-driven writes in the last week
cat .rela/audit/*.jsonl | jq 'select(.principal.tool == "scheduler")'

# All trigger cascades for entity TKT-007
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
- On the filesystem build, audit failures (disk full, permission
  errors, symlinked dir) never block a legitimate entity write. The backend logs
  `audit.write_failed` via slog, enters a 60-second cool-down, and
  retries on the next record.
- `.rela/audit/` should remain on the same filesystem as the project
  — moving it elsewhere isn't supported and the path isn't
  configurable.
