---
id: BUG-CMG6Q7
type: bug
title: stateFamily returns relations in map order; a partial cascade is nondeterministic
description: >-
  FSStore.stateFamily sorts the entity family but returns `related` in whatever
  order ranging over the s.relations map produced. A cascade delete therefore
  visits relations in a random order, so which one a partially failed cascade
  removes before aborting -- and thus what DeleteResult.DeletedRelations
  reports -- varies per run. TestDeleteEntity_PartialCascade_ReportsWhatWasRemoved
  (TKT-A23L87) asserts the removal is SOL-1's and failed roughly half of 20
  runs under -shuffle=on; it passed on the branch's first CI run by luck.
priority: medium
status: backlog
why1: The partial-cascade test failed intermittently under -shuffle=on, on both the default and sqlite build tags.
why2: The cascade removes relation files in the order stateFamily returns them, and that order was random.
why3: stateFamily builds `related` by ranging over the s.relations map, which Go deliberately randomizes, and never sorts it.
why4: The sibling `family` slice IS sorted two lines above, so the function looked order-stable at a glance; only the second loop was missed.
why5: Nothing forced the question, because until a partial failure could be OBSERVED the order was unobservable -- every relation was deleted, so any order gave the same result. TKT-A23L87 made the intermediate state visible and turned a latent nondeterminism into a visible flake.
prevention: >-
  Where a map feeds a slice that anything downstream observes -- an order, a
  first element, a partial result -- sort it at the point of construction
  rather than relying on callers. The tell here was a function that sorted one
  of its two return values: an asymmetry like that is worth a second look,
  because it usually means the second slice's order was assumed rather than
  established.
---

## Symptom

`TestDeleteEntity_PartialCascade_ReportsWhatWasRemoved` fails intermittently:

```
--- FAIL: TestDeleteEntity_PartialCascade_ReportsWhatWasRemoved
    recovery_test.go:106:
        Error: "[]" should have 1 item(s), but has 0
        Messages: want only the relation actually removed, not all of them and not none
```

11 of 20 runs with `-count=20`, on both the default and `-tags sqlite` builds.

## Root cause

`stateFamily` (`internal/store/fsstore/entity.go`) sorts `family` but not
`related`:

```go
sort.Slice(family, func(i, j int) bool { return family[i].Face < family[j].Face })
for _, rm := range s.relations {   // map range: randomized
    if rm.From == id || rm.To == id {
        related = append(related, rm)
    }
}
```

The test injects a remove failure on SOL-2's relation file and expects SOL-1's
to have been removed first. When the map hands back SOL-2 first, the cascade
aborts immediately and removes nothing.

## Fix

Sort `related` by the same `(from, type, to, face)` identity the map key is
built from, so the cascade visits relations in a stable order and
`DeletedRelations` is reproducible.
