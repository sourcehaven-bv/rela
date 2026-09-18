---
id: TKT-FTOENU
type: ticket
title: Adopt stranded bare rows into a face on a type that already has faces
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`rela analyze` reports stranded rows as `bare-row-on-faced-type` (BUG-UA3BK3,
#1625) and tells the operator to "adopt the rows into a face with a
`migrate_face` migration step for that type". That advice cannot be followed.

`migrate_face` only parses across the `faces_introduced` boundary. Its
`Validate` requires the faces to be absent from the from-shape and present in
the to-shape, and the keying property to exist in the from-shape
(`internal/datamigration/steps.go:380-434`). A type that **already** declares
faces has no shape change at all: no delta, no `from`/`to` hash pair, and so no
migration file the step could live in. The remedy text names a step that will
refuse the job.

`Run` is already direction-agnostic — it moves any zero-coordinate row to the
face its keying value maps to (`steps.go:449-523`). Only the parse-time gate and
the absence of a surface stand in the way.

## How rows get stranded

The write path refuses a bare create on a faced type (`ErrFaceRequired`,
`internal/entitymanager/errors.go:99`), so these rows are never produced by
normal use. They come from history: data written before the type declared
`faces:`, a hand-edited file, an import, or a seed. `rela dev seed` produces 35
of them in `prototypes/perf/project`, which is how BUG-UA3BK3 was found.

The rows are unreachable — no world's chain names the bare id — but they are not
corrupt. The content is intact and sitting at a coordinate nothing addresses.

## Proposed solution

A standalone CLI command that adopts stranded bare rows into faces on a type
that already declares them, with no shape change involved:

```
rela migrate adopt-face --entity article --property status \
    --map draft=draft,active=published,withdrawn=published
```

Dry-run by default, `--apply` to execute, mirroring `rela migrate data` and
`rela migrate gc`.

It is a fifth raw-store exception under the established terms: operator-shell
trust boundary, no ACL, `store.WithAttribution` naming the tool, an explicit
audit record, and synchronous pre-delete version capture on the database
backends. It takes the migration lock, so it cannot interleave with a migration
apply or a GC sweep.

## Why a command rather than a migration file

A migration file is an edge between two shape hashes. This repair has no edge:
the schema is already correct and unchanged, and the data is behind it. Writing
a file whose `from` and `to` are the same hash would be stating a schema change
that did not happen, and the gate would have nothing to adopt on success.

The alternative considered was relaxing `ParseFile` to accept a same-hash file
so the existing step could be reused. Rejected: it makes "this file spans no
schema change" a legal migration, which weakens the invariant that a file is an
edge, and it gives `migrate_face` two validation modes keyed on whether the
hashes match.

## Open questions for planning

- Should the exhaustive-mapping rule carry over? Here it is weaker than in
`migrate_face`: the keying property is not being dropped afterwards, so an
unmapped value leaves the rows where they are, still visible to `analyze`,
rather than becoming unrecoverable. Reporting them may be the better answer.
- What happens when the destination coordinate is occupied? `rename_face` and
`migrate_face` both treat an identical-content destination as "already moved"
and refuse a genuine collision (`steps.go:300-307`, `504-510`). The same rule
should apply.
- Does the remedy text in `internal/analysis/states.go:300-302` get fixed in
this ticket or separately? It is wrong today regardless of whether this command
ships.
