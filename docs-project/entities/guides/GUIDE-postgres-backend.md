---
audience: intermediate
id: GUIDE-postgres-backend
order: 13
status: published
summary: Run rela-server and the CLI against PostgreSQL instead of markdown files
title: PostgreSQL Backend
type: guide
---

By default rela stores entities and relations as markdown files and
indexes search with an in-process bleve index. A separate **PostgreSQL
build** stores the same data — entities, relations, attachments, and the
search index — in a PostgreSQL database instead. It is selected at compile
time with the `postgres` Go build tag and shipped as separate binaries:
`rela-postgres` and `rela-server-postgres`.

Use it when you want a real database behind rela (durable, concurrent,
SQL-queryable) rather than a directory of markdown files — including
deployments with **multiple server processes** sharing one database (see
[Multiple writers](#multiple-writers)).

## What still lives on disk

PostgreSQL backs **data only**. The project's schema and configuration
are still read from the filesystem, exactly as in the default build:

- `schema.yaml` — the entity/relation schema.
- `templates/` — optional entity/relation templates.
- `.rela/` — the per-machine cache and audit log.

So a PostgreSQL deployment still points at a project directory (via
`--project` or the working directory); the database URL is **additional**
configuration, not a replacement for the project directory.

## Configuring the connection

The connection string is read from the `RELA_DATABASE_URL` environment
variable. It is **env-only** — there is deliberately no `--database-url`
flag, because a credential-bearing connection string passed on the command
line is visible in `ps` output, shell history, and process listings.

```bash
export RELA_DATABASE_URL='postgres://user:password@db.internal:5432/rela?sslmode=require'
rela-server-postgres --project /srv/rela/project --bind 0.0.0.0 --port 8080
```

On a connection or parse failure, rela surfaces a sanitized error — the
password is **not** echoed.

For production, set `sslmode=require` (or stricter — `verify-full` with a
CA) so the connection is never silently unencrypted.

## Schema and migrations

On first start the PostgreSQL build creates its schema automatically
(`entities`, `relations`, `attachments`, `comments`, `schema_version`, and
the `entity_versions` / `schema_versions` history tables — see
[Version history](#version-history-time-machine) — plus the `pg_trgm`
extension for substring/fuzzy search). Migrations are
embedded in the binary and applied idempotently on every start — they run
in a single transaction under an advisory lock, so concurrent starts are
safe — and upgrading is just deploying a newer binary and restarting.

The connecting role needs privileges to create the `pg_trgm` extension on
first run (typically a superuser, or have an administrator run
`CREATE EXTENSION pg_trgm;` once in the target database beforehand).

Migration 14 rewrites the `search_text` column of every entity once, inside
the migration transaction, so the first start after upgrading pauses for a
time proportional to the entity table — seconds at twenty thousand rows,
longer on a large one. Run `rela db migrate` as a deploy step if that pause
must not land on the first web request.

Migration 18 rebuilds the entity read index so it serves every face. The
build holds a lock that blocks writes to the entity table, but not reads,
until the migration commits: seconds at twenty thousand rows, longer on a
large table. Plan a maintenance window, or run `rela db migrate` as a deploy
step, when the table is large.

Migration 20 adds the `piles` and `pile_items` tables. A pile is one user's
named stack of entity references. Like comments, piles live in the tenant's
schema, so every rela-server process on that schema sees the same piles and
another tenant sees none of them. They are not graph content: they are not
audited, versioned or searched.

Migration 35 adds read markers for channels. It reads every stored
channel message once and builds an index on that table, while it holds a
lock that blocks reads and writes of channel messages until the migration
commits. The pause grows with the number of channel messages. Plan a
maintenance window, or run `rela db migrate` as a deploy step, when that
table is large.

### Applying migrations explicitly

If you would rather apply the schema as a separate, controlled step
(for example with a more-privileged role at deploy time, or to gate a
release in CI) the `rela db` command group does this without starting a
server:

```bash
rela db status    # report current vs expected schema version; non-zero exit if behind
rela db migrate   # apply pending migrations; a no-op when already current
```

`rela db status` is read-only and makes no changes — handy as a CI gate.
Both commands read the connection string from `RELA_DATABASE_URL` and
exist only in the PostgreSQL build. Auto-migrate
on startup remains the default, so no explicit step is required for the
common single-server case.

rela's tables are created in the connection's default schema (typically
`public`). Point rela at a database it owns; if you share a schema with
another application, rela's tables sit alongside it.

## Derived schema (constraints and query indexes)

Some things you declare in the metamodel are enforced not only in the
application but by the **database itself** on the PostgreSQL backend. Today
that means `unique: true` properties: for each one, rela maintains a partial
unique index (named `rela_derived_uniq__…`) so a duplicate value is rejected
atomically at write time, even across two server processes writing at once.
On the filesystem backend the same rule is enforced by an application-level
check, which is race-free enough for a single process; the database index is
what makes it safe for a multi-writer PostgreSQL deployment.

These indexes are **derived from the metamodel, not from a migration**. rela
reconciles them at every start: it compares what `schema.yaml` declares
against the indexes actually present and creates the missing ones, dropping
any it previously created for a rule you have since removed. This is
idempotent and self-correcting — a hand-dropped index is recreated on the
next start — and in the steady state (nothing changed) it does no work.

PostgreSQL also derives ordinary B-tree indexes from eligible static queries in
`data-entry.yaml`. Dashboard cards, global next-action queries, and `pick_one`
option queries qualify when they target exactly one entity type and all pushed
filters are non-empty equality checks on declared, scalar string-shaped
properties (`string`, `enum`, `date`, `datetime`, or a custom type). rela
creates one composite `rela_derived_query__…` index for the complete set of
properties in each query shape. Equivalent queries share an index even when
their literal values or filter order differ. (A list's *sort* keys are the one
exception: an enum's declared value order is part of the index definition, not
a literal — see below.)

Lists get the same treatment. A list with a `sort:` whose keys (and whose
static `=` filters, if any) are string-shaped properties gets one
`rela_derived_list__…` index over its type, its filters, its sort keys and the
id — the exact scan a list page performs when the server pushes paging into the
database.

A sort key on an **enum** property (or any property whose type declares
`values:`) is indexed by its declared position rather than its text, so a
workflow enum pages in workflow order. The declared values are part of the
index definition, which has one consequence for operators: editing a `values:`
list in `schema.yaml` changes the index's name, so the next reconcile builds
the new index and drops the old one. With it, page 40 of an 11,000-row list is an index range scan; without
it the database sorts the whole type for every page. A list sorted on an
integer, a list-valued property, or with a `!=` or range filter gets no index,
because such a page is not pushed down (see "Observing query cost").

Runtime and URL queries are not observed. Free text, relations, sorting, glob,
not-equal, empty-value, list-valued, and non-string filters do not produce an
index. Query indexes are recalculated only at startup or by `rela db reconcile`,
so restart or reconcile after changing static query configuration. A missing
`data-entry.yaml` is valid; an invalid or unreadable file aborts reconciliation
without dropping existing rela-owned query indexes.

### When a constraint can't be enforced

Adding `unique: true` to a property whose existing rows already contain
duplicate values cannot create the index — PostgreSQL rejects it. rela treats
this as a **warning, not a fatal error**: the server still starts, the
property is simply left unenforced at the database level (the application
check still runs), and a message tells you which property and how many
duplicate value groups are blocking it. This is deliberate: a configuration
edit must never turn existing data into a startup outage. Clean up the
duplicates and the constraint is enforced on the next reconcile.

### Inspecting and reconciling explicitly

Like migrations, reconciliation runs automatically at startup, but you can
also drive it as an explicit step:

```bash
rela db status              # also reports any derived-schema drift (always exit 0 for drift)
rela db reconcile           # create/drop derived objects to match the metamodel
rela db reconcile --dry-run # show what WOULD change; non-zero exit if anything would — a CI/pre-deploy gate
```

`rela db reconcile --dry-run` is the pre-flight: run it against your live
database before shipping a configuration change to see exactly which constraints
would be created, dropped, or left unenforced (and, with `--show-values`, a
sample of the blocking values — this prints entity data, so it is opt-in and
operator-only). Because the dry-run and the real startup reconcile share one
planner, what the dry-run predicts is what startup does.

## Search

In the PostgreSQL build, search runs **in the database** — there is no bleve
index. Matching is a case-insensitive substring match over a maintained
`search_text` column (the id, then string properties, then the body),
accelerated by a `pg_trgm` GIN index. Results are ranked by trigram similarity
between the query and each entity's **title**: its type's `display_property`,
or a required `title`, `name` or `label`. An entity *about* the term therefore
outranks one that merely mentions it, and entities whose title does not
resemble the query follow in id order. A type with no such property, or with a
templated `display_property`, ranks by its id. Ranking reads only the title,
so it stays cheap when the term appears in most entities; what such a search
still pays for is the match itself, which has to check every candidate row. Text
search matches the same fields as the default backend: entity ID, content, and
string-valued properties.

## Observing query cost

A page in the web app is one HTTP request that may turn into many SQL
statements. To see how many, and how long the database spent on them, run the
server with Debug logging:

```bash
rela-server-postgres --project . -verbose
```

With `-verbose` every API response carries a `Server-Timing` header — shown
under *Timing* in the browser's network inspector — and the server log gets one
`request` line per request:

```text
Server-Timing: db;dur=12.4;desc="7 queries"
level=DEBUG msg=request method=GET path=/api/v1/tickets status=200 wall_ms=18.2 queries=7 db_ms=12.4
```

`queries` is the number of statements the request issued (a transaction's
`BEGIN`/`COMMIT` count too); `db_ms` is their summed database time, which is
less than wall time because Go work between statements is excluded. A count
that grows with the number of rows on the page is the signature of a per-row
lookup and the first thing to fix; `db_ms` that stays high with a small count
points at a statement missing an index. Each individual statement is also
logged at Debug (`msg="pgstore: query"`) with its SQL, arguments and duration.

What a well-behaved page costs today: a list page is one scoped count and one
bounded read for the rows, one query for every edge touching the page, one
content-free read of the neighbours those edges name, and the membership walk
for the principal — a handful of statements whatever the page size. Collection
rows are read as headers (no markdown body) throughout; see the API reference
for `include_content`. The list page is pushed into the database — paging,
ordering and equality filters in SQL — when the request carries no free-text
search, no relation filter, and only `=`/`!=` filters and sort keys on
string-shaped properties; anything else takes the whole-type path in Go, which
still reads headers only.

Ordering, on either path, compares property values byte-wise as text and puts
a row without the property after every row that has one when ascending and
before them when descending (SQL's default null placement). That asymmetry is
what lets one index serve both directions.

Below Debug nothing is emitted and no header is set. That is deliberate: a
per-response query count can vary with rows the principal is not allowed to
see, so it is an operator diagnostic, not part of the API — see
`docs/acl-security.md`.

For the database's own view, turn on PostgreSQL's slow-statement log for the
rela database (no restart needed; new sessions pick it up):

```sql
ALTER DATABASE rela SET log_min_duration_statement = '20ms';
ALTER DATABASE rela SET session_preload_libraries = 'auto_explain';
ALTER DATABASE rela SET auto_explain.log_min_duration = '20ms';
```

## Multiple writers

You can run **multiple processes against one database** — for example several
`rela-server` instances behind a load balancer, or a `rela-server` plus an
occasional `rela` CLI write. Each process sees the others' changes, so the UI
stays live across servers: an entity created, updated, or deleted on one server
appears in browsers connected to another, without a manual refresh.

What you need to know to run it:

- All processes that should share live updates must point at the **same database
  and schema** — which they do when they serve the same project.
- Each process opens **one extra connection** to receive change notifications.
  If it can't, the process still works normally; only the live cross-server
  updates are unavailable (a warning is logged).
- That is **one connection per process, not per schema**. All processes LISTEN
  on a single shared channel (`rela_changed`) and the writing schema travels in
  the notification payload, so a process serving several schemas on one database
  still needs only the one connection. Notifications from a schema a store does
  not read are discarded on arrival.
- Live updates cover **entity** create/update/delete. Relation and attachment
  edits are reflected on the next page load rather than pushed live.
- **Channel changes travel on a second channel**, `rela_channels`, over the
  same connection. Each notification names only the channels that changed
  (entity ids and channel types) and, for a mention or a read marker, the
  principal keys involved; never message text. See "Changes from other
  processes" in the realtime guide. Like `rela_changed`, every session that
  LISTENs on the database receives these notifications, from every schema.
  If another tenant on the same database must not learn entity ids or
  principal keys, give it its own database. TKT-147C19D tracks keeping
  principal keys out of these notifications. The other direction holds too:
  any session on the database can send a notification that names a schema,
  and its processes then recheck the channels it names. Every recheck is a
  normal read as each subscriber, so this costs load, never data.
- **Durable UI state is shared too.** The document render cache, user settings,
  the operator logo/theme and the CalDAV alias table live in the database on
  this build, not in each node's `.rela/` directory. Upload a logo on one node
  and every node serves it; a document rendered on one node is not re-rendered
  on the next. (On the filesystem build this state stays under `.rela/`, which
  is correct for a single-process deployment.)
- **The scheduler may run on several nodes.** Last-run times, the retry ladder
  and every run's status live in the `scheduler_tasks`, `scheduler_runs` and
  `scheduler_run_children` tables. A unique index allows one active run per
  task, so two nodes ticking at the same moment cannot both queue it. Any
  node's worker may execute a run, and records the outcome in the same tables.
  A run whose node dies is marked `abandoned` when its lease expires and then
  retried. See [scheduled-tasks.md](scheduled-tasks.md#run-state).

- **Shared body editing needs sticky routing.** A shared editing session
  lives in the memory of one server process. Two people whose editors reach
  different processes are in different sessions: each session saves, and the
  second save merges with the first like any concurrent edit. To keep people
  in one session, route every request for an entity to the same process, for
  example by hashing the path on the load balancer, or route by user and
  accept the merge. The session's address is
  `/api/v1/_collab/{type}/{id}`, a WebSocket.

### Write transactions

The store exposes a write-transaction contract (`store.Store.Tx`,
DEC-8UIL0). On PostgreSQL a `Tx` callback runs inside one real database
transaction, serialized **across the processes sharing a schema** by a
transaction-scoped advisory lock (its own key, distinct from the
migration and version-sweep locks): every process serving that schema
executes write transactions one at a time. An error rolls the whole
callback back — no rows, no cross-process NOTIFYs, and no in-process
events are delivered (event fan-out is buffered until commit). On the
filesystem backend the same contract degrades to a process-local write
mutex with no rollback — acceptable for its single-user deployments.

Because the advisory lock covers the whole schema, a slow transaction
stalls every writer against it. Keep `Tx` callbacks short and never
perform external I/O inside one.

All three of rela's advisory locks — migration, version sweep, and
write — are keyed by the schema they act on. PostgreSQL advisory locks
are otherwise database-global, so without this two rela schemas sharing
one database would migrate, sweep, and write behind each other's locks
despite being entirely independent deployments.

## Version history (time machine)

The PostgreSQL build automatically keeps a **content version history** of every
entity — the analogue of the git history a filesystem project gets for free.
You can list an entity's past versions, view any one of them (rendered against
the schema it was written under, not today's), diff two, and restore an entity
to an earlier state — each version carrying who made the change and when.

How capture works:

- **Edits are debounced.** A background reconciliation sweep snapshots entities
  that have been *settled* (un-edited) for a few minutes, so a burst of edits
  collapses to one version rather than one-per-keystroke. An entity under
  continuous editing is still snapshotted at least once per staleness ceiling
  (default ~1 hour), so a long editing session is never entirely unrecorded.
- **Deletes and renames are captured immediately** at write time (a delete
  records the final pre-delete state so a deleted entity's history — and the
  ability to restore it — survives; a rename records the old→new id so history
  stays continuous across the rename).
- Unchanged re-saves are de-duplicated (no version row when content is identical
  to the latest).

Attribution: the version's principal (user + tool) and any `triggered_by`
(automation / schedule / cascade) are recorded. Every create/update write stamps
the editing principal onto the live row (`last_edited_by_user` /
`last_edited_by_tool`), and the sweep copies it onto the debounced snapshot — so
a swept version names the real editor, not a system account. Rows with no
recorded editor (pre-existing rows from before the columns existed, or writes
that carried no principal) fall back to the `version-sweep` system principal
rather than guessing; they self-heal on their next attributed edit. A rename
only re-keys incident relations, so those keep their last *content* editor. One
limit today: if two different people edit the same entity within one debounce
window, the single collapsed version is attributed to the last of them. This all
inherits the same trust model as the [audit log](audit-log.md) — attribution is
only as strong as your deployment's identity front door, so use a verifying
(JWT) principal source where version attribution matters for accountability.

Storage: two tables — `entity_versions` (one full snapshot per version, keyed by
a versioning-internal record id so history survives id rename/reuse) and
`schema_versions` (the content-addressed render-schema each snapshot was taken
under, de-duplicated so an unchanged schema across thousands of writes stores
one row). Both are separate from the hot `entities` table; the sweep filter adds
an index on `entities(updated_at)`. History is retained indefinitely by default;
apply your own retention (by age or count) if storage growth matters — but note
audit/compliance deployments typically keep ≥ 12 months.

From the CLI (PostgreSQL build):

```bash
rela history TKT-42                 # the version timeline with attribution
rela history TKT-42 --version 3     # print version 3's snapshot (JSON) …
rela history TKT-42 --version 3 | diff - <(rela history TKT-42 --version 5)
                                   # … pipe two snapshots to any diff tool
rela restore TKT-42 3              # restore the entity to version 3
```

The data-entry web UI shows the same timeline, an in-page diff, and a restore
button (gated by your write permission) on each entity's detail page.

**Sharing a diff.** The two compared versions live in the URL as `?base=` and
`?target=`, so a specific diff can be linked, bookmarked, or reopened after a
reload:

```text
/history/feature/FEAT-42?base=3&target=7          # v3 → v7
/history/feature/FEAT-42?base=3&target=current    # what changed since v3
```

Either side takes a version ordinal or `current`. Note that `current` is
**live-relative**: a link with `target=current` shows the diff against the
entity as it stands when the recipient opens it, not as it stood when the link
was made. Link two ordinals for a diff that is frozen. Omit the params for the
default view, and a value that names no existing version (a stale link, a typo)
falls back to that default rather than erroring — the address bar is rewritten
to the pair actually being shown, so a corrected link is what you copy. A
shared link is not a capability — the recipient still needs their own read
permission on the entity, and sees the same 404 they would without the link.

Access control: reading the history of a **live** entity requires the same read
permission as reading the entity itself. Reading the history of a **deleted**
entity requires the global `history:read` permission — see
[ACL security](acl-security.md). Restore is a normal write: it is authorized,
validated, and audited like any edit, and produces a new version.

Faces: on a type that declares `faces:`, each face has its own history. The
CLI, the API and purge all take an `ID@face` address, and a bare id of a faced
entity is refused on the command line. A purge reaches one face only. See
[Content states](content-states.md).

Scope: an entity version captures that entity's content and properties. Its
*relation set* as-of the version is not part of the entity snapshot, so an entity
restore recovers content and properties — relations are versioned separately (see
below).

### Relation history

Relations carry their own rich content (a property set plus a markdown body), and
the PostgreSQL build versions them too, with the same time-machine model. Capture
is the same hybrid: relation create/update are debounced by the sweep;
**relation delete is captured immediately** — including when a relation is
destroyed by an entity **cascade delete** (deleting a hub entity records a final
version for every edge it removes, so an edge's history never just vanishes).

An endpoint **rename** stitches, rather than forks: when an entity is renamed, its
incident relations are recorded as a continuous timeline across the new endpoint
(the version carries the pre-rename endpoints), so a rename reads as a rename —
not as a mass delete-and-recreate.

Each relation has a stable internal record id, so a delete-then-recreate of the
same `from--type--to` starts a **fresh** history (the two lifetimes are not
merged).

From the CLI (PostgreSQL build):

```bash
rela relation-history TKT-42 blocks TKT-99            # the relation's timeline
rela relation-history TKT-42 blocks TKT-99 --version 2  # print version 2 (JSON)
rela relation-restore TKT-42 blocks TKT-99 2          # restore to version 2
```

Access control: relation history is read-gated on **both** endpoints — you must
be able to read the `from` AND the `to` entity. When part of the relation is
gone, every endpoint that still exists keeps its check and the global
`history:read` is required too (see the ACL security guide). In the web UI, a relation's history is owned by its
**source** (`from`) entity: each outgoing relation on an entity's detail page has
a History affordance. Restore goes through the normal write path; re-creating a
relation whose endpoint entity no longer exists is refused (409).

Relation diffs are shareable the same way, with `?base=`/`?target=` on the
relation-history URL — though here `current` resolves to the newest captured
version (labelled *latest*), since a relation has no separate live-read
endpoint:

```text
/relation-history/ticket/TKT-42/blocks/TKT-99?base=1&target=3
```

### Version tags

A version tag is a name that points at one version of an entity. A sync
connector tags the version it last pushed (`sync/jira`), so the next run can
diff against it. A reviewer tags the state they approved (`reviewed`). Tags
live beside the history, not in the entity, so setting one is not an edit and
creates no version.

Tag names are lowercase. A name may have one namespace segment, `ns/name`.
Each face of an entity has its own tags, and one name points at one version
at a time: tagging again moves the tag.

```bash
rela history-tag TKT-42 reviewed                  # tag the current state
rela history-tag TKT-42 reviewed --version 3      # tag version 3
rela history-tag TKT-42 sync/jira --expect <tok>  # tag only if unchanged since <tok>
rela history-tag TKT-42 reviewed --delete         # delete the tag
rela history TKT-42                               # the timeline lists each version's tags
```

Tagging the current state records it as a version at once, so the tag never
waits for the sweep. With `--expect`, the tag is written only while the entity
still matches the token a script read earlier (`rela.version_token` in Lua).
If someone edited it in between, nothing is tagged.

Access control: a tag needs `update` on the entity. A tag in a namespace
`ns/...` also needs the permission `tag:ns`, so a user who can edit an entity
cannot move a connector's sync base. See
[ACL security](acl-security.md#version-tag-permissions-tagns). Every tag write
is audited (`version-tag`, `version-untag`). Lua scripts tag with
`rela.tag_version` (see the [Lua scripting guide](lua-scripting.md)).

A tag write is refused inside a store transaction (`ErrTagInTx`), because it
writes history on its own connection. Tag from a background action or a
scheduled script, not from a synchronous automation.

### Purging history for compliance

History is append-only by design. When you must actually **remove** content from
history — a leaked secret, PII, a GDPR erasure request — the `history-purge` /
`relation-history-purge` commands hard-delete version snapshot rows. This is the
deliberate, audited, **irreversible** exception to the append-only model, and it
is **operator-only**: the trust boundary is shell access plus the database
credential (`RELA_DATABASE_URL`), the same as `rela db migrate` — the CLI applies
no ACL check.

```bash
# Dry-run (the DEFAULT): shows exactly what would be purged, deletes nothing.
rela history-purge TKT-42 --content-hash <h> --reason "erase SSN per DPO-42"

# Commit the deletion (irreversible). Confirmation asks you to type the id.
rela history-purge TKT-42 --content-hash <h> --reason "erase SSN per DPO-42" --commit
```

Target a specific row by `--vseq` (from `rela history`), every row holding a
value by `--content-hash` (the "erase this value everywhere it was captured"
operation — verifiable, since afterward no row carries that hash), or the whole
lineage with `--all`. `--reason` is required and recorded in the audit trail;
**do not put the secret itself in `--reason`** (it is logged in cleartext). The
purge itself is audited (who, what, count, when) — that record is the surviving
compliance trail and never contains the purged content.

Two guardrails you will hit:

- **Purge refuses while the live entity/relation still holds the content.** The
  version sweep reconciles history *toward* the live row, so purging history
  without first redacting the live value would just re-capture it within a few
  minutes. Redact (or delete) the live value first; or pass `--force-live`, which
  writes a tombstone that stops the sweep from re-capturing.
- **Purge refuses a `rename` row** (purging one would sever unrelated history).
  Select non-rename rows by `--vseq`/`--content-hash`.
- **Purge refuses a row a version tag points at.** The dry run lists those
  tags. Move or delete the tag first, or pass `--force-tags`, which deletes the
  tags with the rows.

**Purge is one necessary step, not cryptographic erasure.** It removes rows from
the primary database; a value may still survive in your PITR/base backups and WAL
until their retention expires — accounting for the backup lifecycle is the
operator's separate responsibility.

## Data migration (schema shape changes)

Each schema (tenant) tracks the shape of the metamodel its DATA conforms to
in its own `migration_state` row, so tenants at different points migrate
independently: `rela migrate data` resolves each store's own chain (see the
[data-migration guide](data-migration.md)). Postgres-specific behavior:

- Migrated content lands in the version history: create/update rewrites are
  captured by the version sweep (attributed to the operator with the
  `data-migration` tool via `store.WithAttribution`), and destructive steps
  (`drop_entities`, `drop_relations`, `rename_relation_type`, GC deletions)
  capture pre-delete snapshots **synchronously** — the sweep cannot
  reconstruct a row that is already gone.
- `rename_relation_type` recreates relations under the new type, so relation
  history starts a fresh lifetime (`rel_record_id`); the old lifetime is
  closed with a delete capture.
- The drift GC sweep runs in each process that assembles services, one
  goroutine per store; runs are serialized against writers through the
  store's transaction contract. `RELA_DATA_GC=off` disables it (e.g. on
  read-mostly replicas where one designated process should own GC).
- Migration and GC applies (and gate marker writes) are mutually excluded
  across processes by a schema-scoped **advisory migration lock** — a
  session lock held on one pool connection for the duration of the run, so
  two `migrate data --apply` invocations against the same schema cannot
  interleave, while different schemas stay independent.

## Undoable deletes

A web-app delete can be undone for a short while (see the data entry guide).
While it waits, the deleted rows sit in two side tables, `marked_entities` and
`marked_relations` (migration 0018), so no query on `entities` or `relations`
can see them. The delete itself writes the usual tombstones and change
notifications, and an undo gives the rows a fresh sequence number, so other
processes see a delete followed by a create.

The soft-delete GC runs through the durable job queue with one fixed
idempotency key, so several servers against one schema run one purge at a
time. `RELA_SOFT_DELETE_DELAY` and `RELA_SOFT_DELETE_GC_INTERVAL` set the
window and the check interval.

## Job queue tables

The durable job queue uses [River](https://riverqueue.com). Its tables
(`river_job`, `river_queue`, `river_leader` and a few more) live in the
tenant's schema. Every rela process brings them up to date when it starts.
River deletes finished, cancelled and discarded jobs after 24 hours.
Waiting jobs, including failed ones that will be retried, stay. The queue
uses its own pool of up to six database connections.

### Upgrading from a neoq build

Earlier builds used neoq, with tables named `neoq_*`. The first start of a
River build moves the jobs that were still waiting into River and drops the
neoq tables. Stop every older rela process first. While an older process
is running a job, or is adding one, the move waits for the next start. Once
the tables are gone, an older process that is still running can no longer
add jobs. The waiting jobs run once the move succeeds. The step is one way;
an older build cannot read River's tables.

## Tenant mode

One `rela-server-postgres` process can serve several organizations, each in
its own PostgreSQL schema. Turn it on with the `-tenant-mode` flag or with
`RELA_TENANT_MODE=1`, and give the database in `$RELA_DATABASE_URL`:

```bash
export RELA_DATABASE_URL='postgres://rela@db.internal/rela?sslmode=verify-full'
export RELA_TENANT_MODE=1
rela-server-postgres -project /srv/rela -jwt-issuer ... -jwt-audience ... -jwt-jwks-url ...
```

There is no tenant list in the project directory. Where a tenant lives is
derived from its verified `org_id` claim:

- The schema is `org_` followed by the first 16 hexadecimal characters of
  the SHA-256 hash of the org id, for example `org_822b33ad87c148a0`. The
  name never depends on what the org id contains, so an org id cannot
  inject SQL or reach another schema.
- The connection is `$RELA_DATABASE_URL` with only `search_path` changed to
  that schema. Every other setting, including TLS, is kept.

A table records which tenants exist: `rela_control.tenants`. It has one row
per tenant with the schema name, the org id and a status. The server only
reads it. Only the `rela-server tenant` commands write it.

The table lives in a schema of its own, `rela_control`, for these reasons:

- It cannot live in a tenant's schema. It is how a tenant is found, and
  erasing that tenant would drop it.
- It must not live in `public`. Every tenant connection has `public` on its
  `search_path`, so a table there is reachable from every tenant. Erasure
  also refuses to run while `public` holds a table.
- It lives in the same database as the tenants, so the server needs no
  second credential to find them.

A tenant is served only when its row says `active` and its schema exists.
Every other case is refused and opens no store:

| Case | Response |
| --- | --- |
| No row for the org | 403 `unknown_tenant` |
| Status `provisioning`, `suspended` or `erasing` | 403 `unknown_tenant` |
| The org's schema is recorded for another org | 503 |
| Status `active`, but the schema does not exist | 503 |
| The control table cannot be read | 503 |

Two org ids that derive the same schema name are refused, never shared. The
control table stores the org id of each schema, so a second org that
derives a taken name is detected. With 64 bits of hash this is very
unlikely; if it happens, the second org is refused until an operator
resolves it.

Tenant mode sizes are flags, with environment variables as defaults:

- `-tenant-max-resident` (`RELA_TENANT_MAX_RESIDENT`, default 4) bounds the
  idle tenants kept open. Each open tenant holds its own connection pool. A
  tenant that a live update stream still uses stays open past that bound
  until the stream ends.
- `-tenant-max-open` (`RELA_TENANT_MAX_OPEN`, default twice the resident
  bound) is the hard ceiling. Above it a new tenant gets 503 with
  `Retry-After` until a slot frees. Size it against the cluster's
  `max_connections`.

The server also holds a small pool of at most four connections to read the
control table. It reads one row per request, by primary key, and keeps no
cache, so a status change applies to the next request.

Every tenant shares the project configuration (`schema.yaml`,
`data-entry.yaml`, `acl.yaml`, scripts). Each tenant has its own store,
search, live updates, co-editing rooms, Lua cache and audit log
(`.rela/audit/tenants/<schema>/`).

How a request is routed:

- The server verifies the identity assertion and reads its `org_id`.
- A request without an assertion, or with one that does not verify, gets
  401.
- A verified assertion without `org_id`, or with an org that is not an
  active tenant, gets 403 `unknown_tenant`.
- If the tenant cannot be found or opened, or the server is at the hard
  ceiling, the request gets 503 with `Retry-After`.
- The SPA shell and its static assets load without a tenant, so the browser
  can sign in.

No refusal falls back to a default store. Tenant mode needs JWT identity
(`-jwt-issuer`, `-jwt-audience`, `-jwt-jwks-url`). It refuses
`-principal-header`, `$RELA_DATAENTRY_USER` and `--config-editing`, and it
refuses to start on a build other than postgres. It also refuses to start
when the control table does not exist yet: create a tenant first.

A server, CLI command or `rela-docs` run without tenant mode refuses a
database that serves tenants: one with `rela_control.tenants`, reached on
`public` or on a tenant's schema. So forgetting the switch fails at start
instead of serving every org from `public`.

Anonymous requests reach no tenant. This includes requests that Pratique
forwards on `upstream.anonymous_paths` with `X-Pratique-Anonymous: true`,
because they carry no `org_id`. This is deliberate: every network deployment
runs behind Pratique with a verified `org_id`, and public pages belong in
Pratique or a separate publishing service, not in rela.

The IdP webhook (`POST /webhooks/idp`) is routed by the `org_id` in its
signed body. An event for an org this server does not host, or for one
being erased, is acknowledged and ignored. An event for a suspended or
provisioning tenant gets 503 with `Retry-After`, so the IdP delivers it
again: a role removal sent during a suspension still applies after the
resume. Declarative webhooks (`/hooks/<id>`) carry no assertion, so the
server cannot tell their tenant and refuses them.

Each open tenant holds one extra connection, which holds the tenant's
schema lock while the tenant is open (see "Erasing a tenant").

Every tenant uses the same database role, the one in `$RELA_DATABASE_URL`.
The `rela-server tenant` commands connect as that role too, so the tables a
create makes belong to the role the server uses.

A different database for one org is not supported yet. When it is added,
it will be a resolver that wraps the derived one for the orgs it names,
with each such DSN kept in the secret store, not in a project file.

### Managing tenants

The `rela-server tenant` commands create and change tenants. Run them with
the postgres build on a host that has the project directory, with
`$RELA_DATABASE_URL` set. They read the DSN from the environment only,
never from their arguments, and never print it.

```bash
rela-server-postgres tenant create  -project /srv/rela org_7f3a
rela-server-postgres tenant show    -project /srv/rela org_7f3a
rela-server-postgres tenant suspend -project /srv/rela org_7f3a
rela-server-postgres tenant resume  -project /srv/rela org_7f3a
rela-server-postgres tenant erase   -project /srv/rela org_7f3a
```

- `create` records the org as `provisioning`, creates its schema, checks
  that the tenant connection lands in it, runs the store migrations and
  sets the status to `active`. The first create also creates the control
  table. A create that fails part way leaves the status at `provisioning`,
  which is not served; run it again to finish. It refuses an org that
  already has a row, and a schema that already exists without a row,
  because that schema may hold other data. Use `-adopt` to take over such
  a schema on purpose.
- `show` prints the org's derived schema and its status, or "no tenant".
- `suspend` stops serving the org and keeps its data. Requests get 403 at
  once. Each server closes the tenant within 30 seconds, and ends its live
  update streams. IdP webhooks for the org are refused with 503 so that the
  IdP retries them. If the suspension outlasts the IdP's retries, resync
  the org's roles from the IdP after the resume.
- `resume` serves a suspended org again. It cannot resume an org that is
  being erased.
- `erase` is described below.

Every status change is checked against the current status in the same
statement, so two operators cannot undo each other's change.

Each `create`, `suspend` and `resume` attempt, whether it fails or
succeeds, is appended to `.rela/audit/tenant-lifecycle.jsonl` on the host
that ran it: the time, the operator (`$USER`), the command, the org id,
the schema when known and the result. Erasures have their own record,
described below. Neither record holds tenant data.

A `rela-postgres --tenant <org_id>` process, such as the scheduler or stdio
MCP, checks the same status every 30 seconds. When the tenant is no longer
served, it prints a message and exits.

Automatic creation when the IdP reports a new organization is not built
yet (TKT-TNPRV8).

**Upgrading from `tenants.yaml`.** Earlier versions listed tenants in
`tenants.yaml` with schema names you chose. That file is no longer read.
While the project holds it, `rela-server`, `rela-docs` and every `rela`
command that opens data refuse to start. The `rela-server tenant` commands
still run, so you can do the upgrade. For each tenant in the old file:

1. Work out the derived schema name. `tenant show` cannot tell you yet:
   it reads the control table, which the first `tenant create` makes. In
   `psql`, run
   `SELECT 'org_' || left(encode(sha256(convert_to('ORG_ID', 'UTF8')), 'hex'), 16);`
   or in a shell, run `printf '%s' 'ORG_ID' | sha256sum | cut -c1-16` and
   put `org_` in front (on macOS, use `shasum -a 256`). For org id `acme`,
   both give `org_822b33ad87c148a0`. In SQL, write a single quote in the org
   id twice.
2. Stop the servers. Rename the schema:
   `ALTER SCHEMA tenant_acme RENAME TO org_822b33ad87c148a0;`
3. Rename its audit directory the same way:
   `.rela/audit/tenants/tenant_acme` to `.rela/audit/tenants/org_822b33ad87c148a0`.
4. Run `rela-server-postgres tenant create -adopt ORG_ID`. Then
   `rela-server-postgres tenant show ORG_ID` prints the schema, so you can
   check it against step 1.

Then delete `tenants.yaml` and start the servers with tenant mode on.

### Operator commands in tenant mode

The CLI, the scheduler and stdio MCP have no identity assertion, so you
name the tenant yourself. Set `RELA_TENANT_MODE=1` and use
`--tenant <org_id>` or `$RELA_TENANT`, with the postgres build
(`rela-postgres`):

```bash
export RELA_TENANT_MODE=1
rela-postgres --tenant org_7f3a list ticket
RELA_TENANT=org_7f3a rela-postgres mcp
```

The rules:

- In tenant mode, every command that reads or writes data needs a tenant.
  Without `--tenant`, it fails.
- An org that is not an active tenant is an error. No command falls back to
  a default store.
- Commands that touch no data run without a tenant: `version`, `init`,
  `completion`, `apps`, `migrate` (config files only) and `validate`.
  Without a tenant, `validate` skips the scheduled mail recipient check,
  and `validate --check` fails, because both read data.
- Without tenant mode nothing changes, and `--tenant` is refused, so you
  cannot act on the wrong store by mistake.
- A build other than postgres refuses tenant mode.
- The tenant opens as the server opens it: its schema is checked and
  locked, and writes go to its own audit log.
- `rela-postgres --tenant <org_id> db migrate` (and `db status` and `db
  reconcile`) checks that the connection lands in the tenant's schema
  first, so a missing schema is refused rather than migrated in `public`.
  Without tenant mode, a `db` command refuses a database that serves
  tenants.
- `rela-docs` refuses tenant mode and a database that serves tenants.

Run one scheduler process per tenant:
`rela-postgres --tenant <org_id> scheduler`. Each process runs that
tenant's schedules against its own store, job queue, run state and audit
log. One process that loops over every tenant is not available yet: it
needs tenants that stay open with their queue workers, which the tenant
host does not yet provide.

The command's principal is bound as before and resolved in the tenant's
own graph. It carries no `org_id` claim: the tenant is fixed by the store
the command opens.

### Erasing a tenant

`rela-server tenant erase` removes one org's schema and this host's audit
log for it:

1. Check that no open or recently closed incident, NCR or dispute refers to
   the tenant while its evaluation period is still running
   (PROCEDURE-u020). Erasure deletes the tenant's audit log at once, ahead
   of the normal audit retention period, because it is that customer's
   data; such a case needs it kept first.
2. Run the erasure. It shows the database it will act on and asks you to
   type the org id; `-force` skips the question.

   ```bash
   export RELA_DATABASE_URL='postgres://...'
   rela-server-postgres tenant erase -project /srv/rela org_acme
   ```

3. On each other server host, remove that host's audit directory,
   `.rela/audit/tenants/<schema>/`, by hand. The command printed the schema
   name, and `tenant show` prints it too.

The command first sets the status to `erasing`. From that moment the org
is not served, and the status cannot be changed back: neither `resume` nor
`suspend` accepts an erasing tenant. Servers close the tenant, and
`rela-postgres --tenant` processes exit, within 30 seconds. The command then drops the schema under the tenant's lock,
removes the audit directory and deletes the control row.

If a server or command still has the tenant open, the drop is refused with
"open in a running server or being erased". The status stays `erasing`.
Wait 30 seconds, or stop the process that holds it, and run the command
again. Running it again after any partial failure is safe.

What it removes:

- The tenant's schema, with `DROP SCHEMA ... CASCADE`. The schema holds
  everything the store keeps for the tenant: entities, relations,
  attachment bytes, search, version history, comments, jobs, leases, piles
  and runtime state.
- The tenant's audit log on this host, `.rela/audit/tenants/<schema>/`.
  Each tenant has its own audit directory so that it can be deleted with
  the tenant. If that directory is a symlink, the command refuses, so that
  it never reports records removed that are still on disk.
- The tenant's row in `rela_control.tenants`.

What it does not remove:

- Audit directories on other server hosts (step 3), and copies of any
  audit log.
- Server and access logs, which name the org id and the paths requested.
- Backups (see below).

Each erasure attempt that passes the confirmation, whether it is refused
or completes, is appended to `.rela/audit/tenant-erasures.jsonl`: the
time, the operator (`$USER`), the org id, the schema and the result. It
holds no tenant data and is not removed with the tenant.

The command refuses:

- an org that is not a tenant;
- an org whose derived schema is recorded for another org;
- a mismatched confirmation;
- a tenant whose lock is held (see below);
- a database whose `public` schema holds any table (see below).

**The schema lock.** Every server holds a lock on each tenant it has open,
on a connection of its own, and the erasure takes that lock exclusively.
While a server has the tenant open, the erasure is refused. While the
erasure runs, no server can open the tenant. After it, the org has no
control row, so no server opens it again.

The lock only lasts as long as the connection that holds it. A database
restart, `pg_terminate_backend` or an `idle_session_timeout` ends it while
the server keeps the tenant open. That is why the `erasing` status comes
first: it stops the servers serving the org whether or not they still hold
the lock. If an open tenant's queries run after the drop, they fail: every
tenant connection has `public` on its `search_path`, and the erasure
refuses while `public` holds any table, so there is no other data to fall
back to. The lock is a PostgreSQL session lock, so it needs a direct
connection or a pooler in session mode, not PgBouncer in transaction mode.
A process that reaches the schema without tenant mode, such as `psql`,
holds no lock.

**What erasure guarantees.** When the command returns, the tenant's schema
and control row are gone from the live database, and its audit directory
from this host. PITR backups (for example pgBackRest) still hold the data
until they expire, so erasure is complete when the backup retention period
has passed. State that period in days to your customers, as the time
within which their data is erased. Restoring a backup taken before the
erasure restores the tenant, so erase it again after such a restore.

**When the IdP deletes an org.** A verified `organization.deleted` webhook
does not erase anything. The server logs a warning that names the org and
says that erasure is needed, and acknowledges the event. Erasure stays an
operator action at the host: it cannot be undone, and a signed event is a
weaker authorization than an operator who can reach the database.

## Other scope notes

- The desktop app (`rela-desktop`) is filesystem-only; there is no PostgreSQL
  desktop build.
- There is no automatic migration of an existing filesystem project into
  PostgreSQL; the database starts empty.
