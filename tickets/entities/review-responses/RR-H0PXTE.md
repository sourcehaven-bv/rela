---
id: RR-H0PXTE
type: review-response
title: DeleteEntity and RenameEntity read the zero coordinate, bypassing ACL on faced entities
finding: DeleteEntity and RenameEntity resolved the entity via GetEntity (zero coordinate) rather than the face they were authorizing. For a faced entity the read missed, took the not-found branch, and that branch skips the ACL check by design. A rename of a faced entity therefore succeeded under a deny-all ACL.
severity: critical
resolution: Added anyFaceOf (bounded EntityQuery{IDs, AllStates}) and routed both DeleteEntity and RenameEntity through it, so the row they authorize is the row they act on. Regression covered by facedwrite_test.go.
status: addressed
---

## Finding

`DeleteEntity` and `RenameEntity` resolved the target with `GetEntity`, which
addresses the zero coordinate. A faced entity has no row there, so the read
missed and both fell into the not-found branch — which deliberately skips the
ACL check, because "does not exist" must not be distinguishable from "not
permitted".

The consequence on rename is an authorization bypass, not just a wrong read: a
faced rename completed under `ReadOnlyACL`.

## Resolution

Added `anyFaceOf`, a bounded lookup (`store.EntityQuery{IDs: []string{base},
AllStates: true}` — not a full scan), and routed both operations through it so
the authorized row and the acted-on row are the same value.

Pinned by the regression tests in `internal/entitymanager/facedwrite_test.go`.
