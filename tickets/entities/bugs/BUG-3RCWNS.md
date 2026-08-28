---
id: BUG-3RCWNS
type: bug
title: Case-variant entity IDs collide in fsstore but coexist in memstore/pgstore
description: Entity IDs differing only by case are two distinct entities in memstore/pgstore but collide into one file in fsstore on a case-insensitive filesystem (macOS/Windows) — the create-time conflict check is byte-exact so the write silently overwrites. Entities move between backends via migration and rela sync; the identity rule must therefore be shared and pinned by storetest conformance rather than patched per-backend.
priority: high
effort: m
why1: fsstore's create-time conflict check is a byte-exact Go map lookup while the filename it writes is case-folded by the host filesystem, so the index and the disk disagree about what already exists.
why2: The conflict check was written per-backend against each backend's own index, rather than against a shared definition of entity identity.
why3: There was no shared notion of ID identity at all — only a shared notion of ID validity (storeutil.ValidateID). Validity answers "is this ID legal", never "are these two IDs the same entity".
why4: The storetest conformance suite pins behaviour the backends must share, but had no case-variant case, so fsstore and memstore could diverge on identity while both stayed green.
why5: Identity was treated as a storage-layer implementation detail instead of part of the store contract, even though entities move between backends via migration and sync — which is exactly where a divergent identity rule causes silent data loss.
prevention: The identity rule now lives in storeutil.FoldID with the reasoning attached, and is enforced by storetest conformance (store-wide-case-insensitive-id-conflict-test) so any future backend inherits it and no backend can drift. The pgstore side is enforced by the database itself via a unique index on lower(id).
status: done
severity: medium
---

## Description

Entity IDs differing only by case (`abc` vs `ABC`) are treated as **two distinct
entities** by memstore and pgstore, but **collide into one file** in fsstore on
a case-insensitive filesystem (macOS, Windows). The create-time conflict check
is byte-exact in every backend, so fsstore silently overwrites instead of
returning `store.ErrConflict`.

Reported as an IB-review finding on PR #1272 by tschmits (CISO).

### Reproduction

Same two operations, three backends, two different outcomes:

| Backend | `CreateEntity("ABC")` after `abc` | `GetEntity("abc")` |
|---|---|---|
| fsstore (macOS, real FS) | `err=nil` — accepted | `"OVERWRITER UPPER"` — **data loss** |
| memstore | `err=nil` — accepted | `"LOWER"` — both coexist |
| pgstore | accepted (`id TEXT COLLATE "C"`, byte-exact) | both coexist |

fsstore verified against a real `OsFS` in a `t.TempDir()`, not `MemFS` — `MemFS`
is case-sensitive and does NOT reproduce it.

### Why this is store-wide, not an fsstore bug

Treating this as "fix fsstore's conflict check" is insufficient. The store
backends must agree on entity **identity**, because entities move between them:

- **Migration** (memstore/pgstore → fsstore): a project holding both `abc`
and `ABC` loses one entity on import, silently.
- **Sync** (`rela sync --remote`): the two sides disagree on whether they
hold one entity or two, so convergence is undefined.
- **`storetest` conformance** is the contract that keeps backends
substitutable (FEAT-CO4YP). An identity rule that holds in one backend and not
another is exactly the drift the harness exists to prevent.

So the invariant belongs in the shared contract, enforced by `storetest`, not
patched per-backend.

### Mechanism

`fsstore/entity.go:210` checks `s.entities[e.ID]` — a case-sensitive Go map
lookup — while `fsstore.go:372` writes `path.Join(entitiesKey, plural,
id+".md")`, where the filesystem does the case-folding. The in-memory index and
the filesystem disagree about what "already exists" means.

Both create and rename paths are affected, in every backend:
`fsstore/entity.go:211,380`, `memstore/memstore.go:334,445,611`,
`pgstore/entity.go:246,401`.

### Scope

IN scope:
- One identity rule for entity IDs, shared across backends.
- Both create and rename paths.
- `storetest` conformance coverage so every backend is pinned, plus a fuzz
target if the existing key-collision targets don't already cover it.
- Relations too, if the same divergence applies to the
`FROM--TYPE--TO` key.

NOT in scope:
- Changing the ID grammar. **Do not "reject uppercase"** — generated IDs are
uppercase by construction (`id.go:239` `strings.ToUpper`), so `TKT-001` and
`FEAT-CO4YP` would become invalid. Manual IDs are lowercase slugs by convention.
Both cases must remain legal; only two IDs differing *solely* by case are the
problem.

### Suggested direction (not prescriptive)

Make the conflict check case-insensitive at the store boundary — e.g. a
case-folded index alongside the exact one — so creating `ABC` when `abc` exists
returns `store.ErrConflict` in every backend. This fixes IDs already on disk
without invalidating a single existing entity.

Note pgstore's `id TEXT COLLATE "C"` makes its byte-exactness explicit and
deliberate, so changing pg semantics needs care: a migration touching the
primary key, and a decision about whether existing case-variant pairs (if any
exist in a deployment) are merged or rejected at upgrade.

### Back-compat

No collisions exist in this repo: all 2030 entities across `tickets/` and
`docs-project/` were scanned, zero pairs differ only by case. A downstream
project that *does* hold such a pair needs a documented resolution path before
the stricter check is enforced.

### Acceptance criteria

1. `CreateEntity("ABC")` when `abc` exists returns `store.ErrConflict` in
fsstore, memstore, and pgstore — asserted by `storetest` conformance, so any
future backend inherits it.
2. The same holds for the rename path.
3. No existing valid ID becomes invalid (`TKT-001`, `FEAT-CO4YP`,
`ai-integration` all still create fine).
4. A regression test reproduces the original data loss against a real
case-insensitive filesystem, or is explicitly skipped with a reason when the
test host's FS is case-sensitive.
5. The migration/sync implication is covered — a project with a case-variant
pair cannot silently lose an entity moving between backends.
