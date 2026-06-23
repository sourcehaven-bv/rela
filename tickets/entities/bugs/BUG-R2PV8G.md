---
id: BUG-R2PV8G
type: bug
title: Create silently becomes an overwrite-update on ID conflict
description: Manager.CreateEntity wrote through upsertEntity, which does CreateEntity then falls back to UpdateEntity on store.ErrConflict. So a create whose ID collided (a racing create slipping past the GetEntity pre-check, or a duplicate ID from a stale scan) silently overwrote the existing entity instead of failing. The explicit GetEntity 'already exists' pre-check was a TOCTOU that gave false atomicity.
priority: high
why1: createCore persisted via upsertEntity, whose create-then-update-on-conflict fallback turned a conflicting create into an update of the colliding entity.
why2: A single shared upsert helper was used for both the create path and the legitimate write-back-existing paths, so the create path inherited update-on-conflict semantics it must never have.
why3: The duplicate-ID guard was a check-then-act (GetEntity then write) rather than relying on the store's atomic create, leaving a TOCTOU window a concurrent create could exploit.
why4: '''A create must never become an update'' was an implicit invariant with no code or test enforcing it at the persistence step.'
why5: Convenience upsert helpers blur create vs update intent; nothing flagged that the create path was using an update-tolerant write.
prevention: createCore now writes with a direct Store.CreateEntity and maps store.ErrConflict to ErrEntityAlreadyExists — never falling through to update. The redundant TOCTOU GetEntity pre-check is removed; the store's atomic create is the uniqueness guard. upsertEntity stays for the three write-back-existing callers (post-automation rewrite, UpdateEntity, cascade WriteEntity). A regression test forces ErrConflict on write and asserts ErrEntityAlreadyExists with zero UpdateEntity calls; it fails without the fix.
status: done
---

## Bug

Found in the 2026-06-09 backend review (write-path theme B4).

`Manager.CreateEntity` persisted through `upsertEntity` (`core.go:259`), which
does `CreateEntity` and, on `store.ErrConflict`, **falls back to
`UpdateEntity`**. The create path therefore had update-on-conflict semantics it
must never have: a create whose ID collided — a concurrent create landing
between the `GetEntity` pre-check (`manager.go:257-264`) and the write, or a
duplicate ID from a truncated scan — silently **overwrote** the existing entity
instead of failing with "already exists".

The explicit `GetEntity` "already exists" pre-check was a check-then-act TOCTOU
that gave a false sense of atomicity; under a race it passed, and the upsert
fallback then clobbered.

## Fix (PR pending)

The principle: **a create must never fall through to an update.**

- `createCore` writes with a direct `Store.CreateEntity` and maps `store.ErrConflict` → `ErrEntityAlreadyExists`. No fallback.
- The redundant `GetEntity` pre-check in `CreateEntity` is removed — the store's atomic create is now the uniqueness guard, closing the TOCTOU. (The `IsManualID` check stays; it's a separate validation.)
- `upsertEntity` is untouched and still serves its three legitimate write-back-existing callers: the post-automation rewrite (`manager.go:293`), `UpdateEntity` (`:425`), and `cascadeHost.WriteEntity` (`:76`).

Regression test `TestCreate_ConflictDoesNotOverwrite` forces `store.ErrConflict`
on write and asserts `ErrEntityAlreadyExists` with **zero** `UpdateEntity`
calls. Verified it fails without the fix (the old upsert fell through to
UpdateEntity).
