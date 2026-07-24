---
id: RR-FCWJ8Q
type: review-response
title: normalizeAssertedRoles merge was non-deterministic and aliased caller slices
finding: 'Two defects introduced BY the RR-IK355A fix commit (b1657597), both in normalizeAssertedRoles (policy.go). (1) Non-determinism: Go randomizes map iteration, so when several padded keys collapse to one trimmed key, which is stored first and which merges into it varies per run. Verified: 500 loads of {admin:[editor], '' admin '':[auditor], ''admin\t'':[reviewer]} produced 3 distinct orderings. Downstream reporting sorts (AssertedGrants, routesFromAttributions), so no golden artifact flakes today — but the same policy text yielded a different in-memory state on every load, and the resolver''s attribution append order varied. (2) Aliasing: the non-collision branch stored the caller''s RoleList header directly, then a later collision did append(existing, r) on it. With spare capacity that writes past len into the caller''s backing array. Verified with a constructed probe: a sibling slice''s SENTINEL element was overwritten. Unreachable via LoadPolicy (the YAML unmarshaler hands over fresh slices), but Validate''s godoc explicitly invites operators to call it on a hand-built policy, and this fix''s own rationale was that policies arrive by paths other than LoadPolicy — so it widened the contract and then wrote code safe only for the narrow one.'
severity: significant
resolution: 'Both fixed in normalizeAssertedRoles: slices.Clone on store so a caller-owned slice is never appended into, and slices.Sort on merged lists only (an unmerged list keeps the operator''s authored order, which is what they see in acl map output). The godoc now states both invariants and why each is load-bearing. Two regression tests added and fault-injected: TestAssertedRoles_MergeIsDeterministic (50 loads, fails with ''merge order varies between loads: run 0 = [editor auditor reviewer], run 3 = [reviewer editor auditor]'') and _MergeDoesNotAliasCallerSlices (fails with ''merge clobbered a caller-owned slice'').'
status: addressed
---

## Finding

Two defects introduced **by** the RR-IK355A fix commit, both in
`normalizeAssertedRoles` (`policy.go`).

**1. Non-deterministic merge.** Go randomizes map iteration, so when several
padded keys collapse to one trimmed key, which is stored first and which merges
into it varies per run. Verified — 500 loads of `{admin:[editor], " admin
":[auditor], "admin\t":[reviewer]}`:

```text
[editor auditor reviewer] -> 381
[auditor reviewer editor] ->  63
[reviewer editor auditor] ->  56
```

Downstream reporting sorts (`AssertedGrants`, `routesFromAttributions`), so no
golden artifact flakes today. But the same policy text yielded a different
in-memory state on every load, and the resolver's attribution append order
varied with it.

**2. Aliasing.** The non-collision branch stored the caller's `RoleList` header
directly; a later collision then did `append(existing, r)` on it. With spare
capacity that writes **past len into the caller's backing array**. Verified with
a constructed probe — a sibling slice's `SENTINEL` element was overwritten.

Unreachable via `LoadPolicy` (the YAML unmarshaler hands over fresh slices). But
`Validate`'s godoc explicitly invites operators to call it on a hand-built
policy, and this fix's own rationale for living in `Validate` was that policies
arrive by paths other than `LoadPolicy`. It widened the contract and then wrote
code safe only for the narrow one.

## Resolution

- `slices.Clone` on store, so a caller-owned slice is never appended into.
- `slices.Sort` on **merged lists only** — an unmerged list keeps the
operator's authored order, which is what they see in `acl map` output.
- The godoc now states both invariants and why each is load-bearing.

Two regression tests, both fault-injected:

- `TestAssertedRoles_MergeIsDeterministic` — 50 loads; fails with *"merge
order varies between loads: run 0 = [editor auditor reviewer], run 3 = [reviewer
editor auditor]"*.
- `TestAssertedRoles_MergeDoesNotAliasCallerSlices` — fails with *"merge
clobbered a caller-owned slice"*.
