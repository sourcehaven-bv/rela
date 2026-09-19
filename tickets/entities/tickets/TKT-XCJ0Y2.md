---
id: TKT-XCJ0Y2
type: ticket
title: Replace the shape-hash migration chain with a committed applied-list; enable data-only migrations
kind: enhancement
priority: high
effort: l
status: review
---

## Problem

Migration files are **edges in a shape-hash graph**: `from:`/`to:` hashes plus
both schema projections embedded. A migration must therefore correspond to a
schema *shape change*.

A **data-only migration** — backfill a property, deduplicate, correct bad values
written by an old bug — has `from == to`, and is rejected at parse time:

```go
// internal/datamigration/file.go:75-77
if raw.From == raw.To {
    return nil, fmt.Errorf("datamigration: %s: `from` and `to` are the same shape hash", name)
}
```

There is no escape hatch. Worse, `loadDataMigrations` returns on the first parse
error rather than skipping the offending file, so dropping a data-only file into
`migrations/` breaks `rela migrate data` and `rela migrate status` for *every*
migration. `rela migrate gen` will not help either: `Generate` returns nil when
there are no shape deltas (`generate.go:34`).

Data-only migrations are routine in every comparable tool (Rails, Flyway,
Liquibase, Django, Alembic, goose) because they track applied migrations **by
name**, not by schema identity. rela structurally cannot express one.

## Root cause: the hash is documented as identity but is really absence-recovery

The marker already stores the **full projection**, not just the hash
(`marker.go:33-40`, whose own comment says "neither works from a hash alone"),
plus an `Applied []string` list. Everything load-bearing — `gen`, the gate's
tiering, `Resolve`'s free edges — diffs *projections*. The hash is only used for
three cheap equality checks: in-sync fast path, file integrity
(`file.go:89-93`), and chain position.

So rela is **snapshot-identified with a hash as a fast equality check** — the
same structure Prisma, EF Core and Django use, and the right one. But the
`from:`/`to:` file format advertises the hash as the identity, and that framing
is what forces the edge model onto every migration file.

Why the hash exists at all on fs: `.gitignore:20` excludes `.rela/`, so the
marker does **not** travel with the committed `entities/`, `relations/` and
`migrations/`. A fresh clone has no marker. Postgres and sqlite do not have this
problem — the marker lives in `state_kv` / `rela.db` alongside the data it
describes. On fs the data and its version pointer live in different trust
domains, so the hash exists to re-derive a baseline when the pointer is absent.

That recovery is optimistic and has its own hole: with no marker the gate adopts
the live shape and declares the store in sync (`gate.go:109-114`). Clone a repo
whose `schema.yaml` moved ahead of the committed entities — exactly when a
migration in `migrations/` still needs to run — and the pending migration
becomes unreachable, because the marker now claims the `to` hash. The hash
hashes `schema.yaml`, which is always current; it never inspects the data.

## Decision

Move to the common, simpler model. Agreed with Jeroen 2026-09-18.

1. **Commit the applied-list.** `migrations/applied.json` — a tracked,
non-hidden file (deliberately not a dotfile: it is a first-class reviewed
artifact like `schema.yaml`, and hiding it makes it easy to miss in review).
Holds the applied migration names **and** the current shape projection. The
version pointer then travels with the data through git on fs, matching what
postgres/sqlite get for free. Rails commits `schema.rb` for the same reason.
`.rela/` keeps only genuinely node-local state (the GC drift ledger).

2. **Timestamp-named files.** `20260918143022-backfill-owner.yaml`, replacing
`%04d` sequential (`generate.go:75`, `nextIndex` at `:381`). See "Numbering"
below.

3. **Drop `from:`/`to:` from migration files. The embedded projections STAY.**
REVISED 2026-09-19 after design review (RR-SJISFW): removing the projections
would reopen BUG-TMGWIN and break historical step validation, and is not
required for data-only migrations. The `from == to` refusal exists only because
the hashes exist, so removing the hashes alone unblocks the feature while
`validateDeltasResolved` and all 13 step `Validate(from, to)` impls keep their
inputs unchanged.

4. **Data-only migrations need no special flag.** A file with no shape edge is
just a file. It runs because its name is not in the applied-list.

5. **Conservative bootstrap.** When `migrations/` is non-empty and there is no
applied-list, refuse to silently adopt — that is precisely the case where "what
shape is this data" is genuinely unknown. Offer an explicit baseline command as
the auditable override (the Flyway `baselineOnMigrate` role).

Note this returns the system close to what RES-273DPJ originally recommended:
"Applied ledger in `internal/state.KV`: migration name + content hash +
applied-at + outcome." The shape-hash chain was introduced during
implementation, not by that research.

## Numbering

`nextIndex` picks one past the highest prefix, so two PRs branched from the same
point both generate `0007-`. Neither conflicts in git — different filenames, no
overlapping lines — so both merge clean with undefined relative order. This is a
known failure mode, and rela already has an instance of it in its own pgstore
ladder: BUG-TY2XQC ("Two pgstore migrations share version prefix 0003"). It is
also why OpenStack numbered sqlalchemy-migrate files 1, 50, 100, 150 to leave
insertion room, which pushed Alembic to abandon sequential ids entirely.

**Chosen: timestamps, plus the committed applied-list as backstop.**

Timestamps mean the common case never collides and ordering is stable without
coordination (Rails, EF Core). The residual weakness is that ordering is
*creation* order, not merge order: a migration authored Monday and merged Friday
sorts before one authored Wednesday and merged Thursday. For schema migrations
that is a real hazard; for data-only ones it is usually harmless. The committed
`applied.json` covers it — two PRs both appending to an ordered list conflict in
git, forcing a human to resolve ordering deliberately. That is Atlas's
`atlas.sum` argument: concurrent migration creation *should* produce a merge
conflict rather than a silent double-apply.

Rejected: declared dependencies / DAG (Django's model — most expressive, most
machinery; Django users still reach for `django-auto-rebase` because multiple
heads are annoying enough to automate away). Rejected: renumber-on-merge (Atlas
`migrate rebase`) — needs tooling to be tolerable.

## Consequences to accept

- **The applied-list becomes the only guard against double-apply.** Today a file
whose `to` shape is already reached is skipped even when `applied` is wrong
(`resolve.go`). That backstop disappears. Step idempotency is the remaining
guard — already a documented requirement, but it moves from second line of
defence to *the* line of defence. Say so in the docs.
- **The `from == to` hash integrity check goes** with the hashes. The
projections remain, so step-target validation and the BUG-TMGWIN face-adoption
guard are unaffected.
- **`applied.json` is a generated file in git**, so it conflicts on merge. This
is the intended coordination mechanism, not a defect, but it needs a documented
resolution recipe.

## Design review

Five findings, all resolved — see PLAN-2TN7LF. The material change is RR-SJISFW
above. Also settled: the server never persists migration state (CLI-only
writes), migration names are allowlist-validated through a `MigrationName` type,
and the legacy `state.KV` marker is dual-written for one release so a rollback
is safe.

## Out of scope

Two adjacent findings from the same review, worth separate tickets rather than
folding in here:

- The **drift tier auto-adopts with only a `slog.Warn`** and emits no audit
record (`gate.go` has no audit reference, while `data-migration`/`data-gc` do).
Every comparable middle tier routes to a human; Flyway/Liquibase make the
equivalent fatal with an auditable override.
- **`ShapeProjection.Hash()` has no golden-value pin and no format version.**
`TestShapeProjectionHash_Deterministic` only compares two hashes computed in the
same process, so it would pass through an encoding change that invalidates every
deployed marker at once.

## References

- `internal/datamigration/file.go:75-77` — the refusal (currently untested)
- `internal/datamigration/marker.go:33-40` — projection already stored
- `internal/datamigration/gate.go:109-114` — optimistic bootstrap
- `internal/datamigration/generate.go:75,381` — `%04d` naming
- `.gitignore:20` — `.rela/` excluded
- BUG-TY2XQC — the same numbering collision in pgstore's ladder
- RES-273DPJ — original research, recommended a name-keyed applied ledger
