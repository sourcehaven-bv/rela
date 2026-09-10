---
id: BUG-22XSH3
type: bug
title: A face created while its entity is being deleted survives with its graph already swept
description: DeleteEntity sweeps the whole family plus every relation referencing the id and all attachments. A CreateEntity of a not-yet-existing face on that id, running concurrently, now serializes behind the delete (the family advisory lock added in BUG-HC6I2T) but still succeeds -- producing a row that looks intact while the inbound relations and attachments that belonged to the entity are gone. Separately, because the version lineage fence is built only from `rename` rows, the reused id inherits the deleted entity's version history.
priority: low
effort: m
status: backlog
---

## Description

`DeleteEntity(POL-1)` removes every face of the family, every relation
referencing the id (`WHERE from_id = $1 OR to_id = $1`, so INBOUND edges too),
and every attachment (`internal/store/pgstore/entity.go:529-534`). A concurrent
`CreateEntity(POL-1@concept)` for a face that does not exist yet now waits on
the family advisory lock, then finds no family and creates the row anyway.

The result is an entity that looks intact and is not: the risk that pointed at
the policy, the control implementing it and the norm it satisfied were all
swept, and nothing on the surviving row records that. Deleting it later does not
restore them.

## Why it is unreachable in practice

Both writes must overlap within the same milliseconds, on the same entity, and
the create must target a face that does not exist yet. In an ISMS that means one
person deleting a policy at the instant another adopts a concept of it. Filed
for completeness, not because it is expected.

## How BUG-HC6I2T changed it

It was unreachable before, by accident rather than by design. The headless-state
rule refused any non-default row whose zero-coordinate row was absent, so a
create landing after a delete was rejected. BUG-HC6I2T removed that rule (no
face is privileged by storage), which removed the refusal.

The advisory lock added there (`lockFamily`, `internal/store/pgstore/entity.go`)
closes the *interleaving* half: no row can land between the delete's family scan
and its sweep. What it does not decide is what the create should do once it wins
the lock and finds the family gone.

## The sharper half: version lineage

`lineageCTE` (`internal/store/pgstore/version.go:200`) fences a lineage using
**rename** rows only:

```sql
COALESCE((SELECT max(vseq) FROM entity_versions
          WHERE prev_id = $1 AND op = 'rename'
            AND face = CAST($2 AS text) COLLATE "C"), 0)
```

A delete leaves no rename row, so `lo` stays 0 and the walk is unbounded below.
An id reused after a delete therefore inherits the deleted entity's version
history. `grep` confirms nothing in the walk consults a delete row.

This is the part worth fixing regardless of the race: it applies to ANY id reuse
after a delete, not just the concurrent case. A policy register that deletes
POL-7 and later mints POL-7 again shows the old policy's history under the new
one.

## Two decisions to make

**What a create means when the family is absent.** Either it is a new entity
reusing a freed id (current behaviour, and defensible), or it is refused.
Refusing needs care: "create a face on an entity that does not exist" is also
what the first-ever create of a faced entity looks like, so a naive refusal
breaks the normal path.

**Whether a delete bounds a lineage.** Adding `op = 'delete'` to the fence's
`max(vseq)` term would separate the histories. Check it against the note in
CLAUDE.md that the `[lo,hi)` fencing exists precisely so a rename or id-reuse
cannot merge two entities' histories — a delete is the third case that comment's
reasoning covers but the code does not.

## Test

`TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace`
(`internal/store/pgstore/deleterace_test.go`) currently asserts that NO row of
the id survives. It was written to pin the headless invariant — its name says so
— and that invariant is gone. It is skipped with a pointer to this ticket rather
than rewritten, because what it should assert depends on the first decision
above.

Requires a real database: `RELA_TEST_DATABASE_URL=... just test-postgres`.
