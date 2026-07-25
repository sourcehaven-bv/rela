---
id: RR-KG2FCX
type: review-response
title: 'Null roles:/assignments: blocks made the migration a silent no-op'
finding: |-
    `key:` with nothing under it parses as a NULL SCALAR, not an empty mapping. Both ensureRole and ensureAssignment detected the wrong Kind, built a replacement mapping, and handed it to InsertMapKeyAfter — which no-ops when the key already exists (yaml_util.go:193-197). The freshly-built mapping was dropped.

    Two distinct failures, both reproduced before fixing:

    1. `roles:` null -> the assignment was written naming a role that was never defined. Detect (which only checked assignment-key existence) then returned FALSE, so the migration reported success and never retried. Policy.Validate does not reject a dangling role reference, so nothing downstream caught it. Operator sees success; tasks still read nothing.

    2. `assignments:` null -> the role was added but NO assignment was ever written, and Detect stayed TRUE forever. Every subsequent `rela migrate` re-detected, re-applied and re-wrote the file — a migration that never converges and never fixes anything.

    This is not an exotic input: it is what a commented-out block or half-written policy produces, and acl.Policy accepts it.
severity: critical
resolution: |-
    Root-caused to InsertMapKeyAfter being the wrong tool for "replace a non-mapping value". Added a shared migration.EnsureMapping(node, anchor, key) helper in yaml_util.go that returns the existing mapping, repairs a null/scalar value IN PLACE (preserving document position and comments), or appends a new one. Both call sites now use it, and the helper is available to every future migration in the package — the footgun was generic, not specific to this migration.

    Also added migration.SetMapNode for composite values (SetMapValue is scalar-only).

    Guarded by mutation testing: reverting EnsureMapping to the old semantics fails 6 subtests. Test fixtures gained `roles null`, `assignments null` and `both null` cases in both ProducesLoadablePolicy and ApplyIsIdempotent (idempotency being precisely the property that broke).
status: addressed
---
