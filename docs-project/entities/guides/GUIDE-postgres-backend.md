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
(`entities`, `relations`, `attachments`, `schema_version`, and the
`entity_versions` / `schema_versions` history tables — see
[Version history](#version-history-time-machine) — plus the `pg_trgm`
extension for substring/fuzzy search). Migrations are
embedded in the binary and applied idempotently on every start — they run
in a single transaction under an advisory lock, so concurrent starts are
safe — and upgrading is just deploying a newer binary and restarting.

The connecting role needs privileges to create the `pg_trgm` extension on
first run (typically a superuser, or have an administrator run
`CREATE EXTENSION pg_trgm;` once in the target database beforehand).

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

## Search

In the PostgreSQL build, search runs **in the database** (a `tsvector`
GIN index for ranked full-text plus `pg_trgm` for substring and fuzzy
matching) — there is no bleve index. Text search matches the same fields
as the default backend: entity ID, content, and string-valued properties.

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
- Live updates cover **entity** create/update/delete. Relation and attachment
  edits are reflected on the next page load rather than pushed live.

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
be able to read the `from` AND the `to` entity (a deleted relation uses the same
global `history:read`). In the web UI, a relation's history is owned by its
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

**Purge is one necessary step, not cryptographic erasure.** It removes rows from
the primary database; a value may still survive in your PITR/base backups and WAL
until their retention expires — accounting for the backup lifecycle is the
operator's separate responsibility.

## Other scope notes

- The desktop app (`rela-desktop`) is filesystem-only; there is no PostgreSQL
  desktop build.
- There is no automatic migration of an existing filesystem project into
  PostgreSQL; the database starts empty.
