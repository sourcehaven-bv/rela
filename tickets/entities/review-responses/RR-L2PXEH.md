---
id: RR-L2PXEH
type: review-response
title: 'PR-B minors: family-delete observer comment, shared invariant errors, migration comment accuracy'
finding: (7) One notifyDelete for an N-state family is correct-by-design but non-obvious beside N tombstones; (8) the type-mismatch/headless error strings were duplicated six times across three backends; (9) the migration comment credited the widened lower(id) index with rejecting cross-entity case collisions that the Go family probe actually rejects.
severity: minor
resolution: Comment added at the pg family-delete notify site (mirroring fsstore's); storeutil.HeadlessStateError + storeutil.StateTypeMismatchError are now the single constructors used by all six sites in three backends; the migration comment states the real division of labor (index rejects same-slot collisions, the family probe rejects cross-slot ones).
status: addressed
---
