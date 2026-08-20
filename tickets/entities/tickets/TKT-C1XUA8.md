---
id: TKT-C1XUA8
type: ticket
title: Copy kernel and declared copy definitions (Step 4)
kind: enhancement
priority: high
effort: l
status: backlog
---

Design doc §9. Kernel: write fields/body/edges/attachment-links into a target
face in one store `Tx`, audited (own `audit.Op*` constant recording definition
name + source + target + principal). Named, metamodel-declared copy definitions
invoked **by name only** (transforms-registry shape); mapped fields written,
unmapped untouched; per-relation-type `merge`|`replace`; same-type auto-map,
cross-type explicit map, load-validated; interpolation limited to the existing
`{{...}}` grammar — no expressions.

Security boundary, pinned with tests (§9.2): same-entity copies (promote/revise)
run elevated; cross-entity copies read through the caller's visibility gate
(visible fields/edges only). Identity-scoped/role-conferring relation types
excluded from definitions unless explicitly named; cross-entity edges authorized
per-edge as the acting principal. Guarded states (published) writable ONLY via
definitions naming them as target; `copy_from` is a load-validated constraint.

Guard machinery shared with the statemachine (`requires_permission`, `when:`,
enforcer, 403/422 sentinels), separate declaration. Per-entity checks via
`HoldsPermissionForEntity`. No Manager bypass (purge's shape explicitly
rejected).

**Owed from Step 1 (decided 2026-08-20, PR-B of TKT-DOFYR1):** per-state
content versioning. The Step-1 pg sweep deliberately captures DEFAULT faces
only (`WHERE pointer = ''`, entities AND relations scans) — capturing state
rows under the bare-id lineage would interleave faces in one history and
freeze per-state purge/stitch/addressing semantics before any consumer
exists. This ticket's copy kernel is that consumer: design per-state history
here (pointer column on entity_versions via forward-only migration, lineage
fencing per (id, pointer), the doc's copy-vs-sweep dedup check) or explicitly
re-defer with a reason. The deliberate-skip comments in sweep.go name this
ticket.
