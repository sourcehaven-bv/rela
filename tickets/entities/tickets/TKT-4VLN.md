---
id: TKT-4VLN
type: ticket
title: Reject inverse-name collisions and shadowing in metamodel loader
kind: refactor
priority: high
effort: s
status: done
---

## Problem

`metamodel.RelationDef.Inverse` is a free-form string
(`internal/metamodel/types.go:232`) and the loader
(`internal/metamodel/loader.go`) does no cross-relation uniqueness validation.
Two failure modes are silently accepted today:

1. **Inverse-name collision** — two unrelated canonical relations
declare the same `inverse:` ID. E.g., `blocks` → `blockedBy` AND `prevents` →
`blockedBy`. A consumer that looks up a relation by inverse name has to pick
one; Go map iteration is randomized, so the choice is non-deterministic across
runs.

2. **Inverse shadowing a canonical name** — a relation declares
`inverse: X` where `X` is also the name of a separate canonical relation. A
consumer that resolves a body key by name has to choose precedence between
"canonical X" and "inverse-of-Y, which is X".

Today no code path in rela depends on inverse-name lookup, so neither mode
causes observable bugs. But TKT-GFQK builds the unified data-entry PATCH wire
format on top of inverse-name resolution. Without this validation, that work is
built on quicksand: a future metamodel author silently corrupts data.

## What we want

Reject both failure modes at metamodel load time with clear error messages, so
the constraint can be relied on by downstream code.

## Scope

In:

- `internal/metamodel/loader.go` (or a new `internal/metamodel/validation.go`):
load-time check that builds `inverseOwners := map[string]string{}` (`inverse.ID
→ owning canonical relation name`) in one pass over the relation set.
- On duplicate inverse ID: reject with `inverse_name_collision: relations %q and %q both declare inverse %q`.
- On inverse shadowing a canonical name (other than `symmetric: true`
self-inverse — see exception below): reject with `inverse_shadows_canonical:
relation %q declares inverse %q which is also the name of canonical relation
%q`.
- Exception: when `relDef.Symmetric == true` AND `relDef.Inverse.ID == relType`,
the relation is its own inverse — this is intentional and must load OK.
- Tests in `internal/metamodel/loader_test.go` covering each rejection
case and the symmetric exception.
- After the pass, attach `inverseOwners` to the loaded metamodel so
downstream consumers (TKT-GFQK and future) can do O(1) lookups without
re-scanning.

Out of scope:

- Any consumer of the new lookup. TKT-GFQK does that.
- Migration tooling for projects whose metamodel currently has a
collision. None of the in-tree metamodels (tickets/, docs-project/,
prototypes/data-entry/{,project/,catalog-metamodel.yaml}) currently trip the new
rules, verified by grep before opening this ticket.

## Acceptance criteria

| ID | Statement | Test |
|---|---|---|
| AC1 | Loading a metamodel where two relations declare the same `inverse:` ID fails with error code `inverse_name_collision` and names both relations + the duplicate inverse. | unit |
| AC2 | Loading a metamodel where a relation's `inverse:` ID matches a separate canonical relation's name fails with error code `inverse_shadows_canonical`. | unit |
| AC3 | Loading a metamodel where `symmetric: true` AND `inverse: <self>` is set on the same relation succeeds. | unit |
| AC4 | The loaded `Metamodel` exposes a way to look up the owning canonical relation type for any inverse name (e.g., `meta.InverseOwner(name string) (string, bool)`). | unit |
| AC5 | All in-tree metamodels (tickets, docs-project, prototypes/data-entry/*) continue to load after the change. | `just test` |
| AC6 | The existing metamodel test corpus continues to pass without modification — this is purely additive validation, not a change to existing accepted shapes. | `just test` |

## Risks

- A user's out-of-tree metamodel might trip the new validation
unintentionally. This is a desirable failure (they had a bug); the error message
names the offending relations so the fix is mechanical.
- If `loader.go` already performs some inverse validation (it doesn't,
verified by grep), we'd duplicate the check. Implementation must start by
re-confirming there's no overlap.

## Relation to other work

- **Unblocks**: TKT-GFQK — the unified data-entry PATCH wire format
resolves inverse names at runtime; without this validation, that resolver is
non-deterministic.
- **Independent of**: TKT-ZEKO4, TKT-6WLSW.

## Effort

s. A focused loader-validation pass + tests. Half a day.
