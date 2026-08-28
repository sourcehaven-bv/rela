---
id: TKT-WAA092
type: ticket
title: 'CalDAV: alias service — own injected CalDAV↔rela identity service (rename-safe), not an observer bolt-on'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Persist the mapping between a CalDAV resource and a rela entity, as its **own
service** with its own package — not bolted into an existing subsystem. Design
in **RES-1Y2EB5**.

### Why it is required, not optional

Verified against Apple Reminders (2026-08-09): a **client-created todo uses a
bare UUID** as both its `UID` and its resource filename —
`D8AAE77A-89CB-46D2-BDA4-F319D2014D6B`, not domain-qualified. rela entity IDs
must start with a letter or digit (see `entity.ValidateID`), so a raw UUID can
**never** be a rela entity ID. There is no derivation; the link must be stored.

The rela→CalDAV direction stays derivable via the existing `feedUID` /
`splitFeedUID` (`<type>--<id>@rela`, double-hyphen separator so hyphenated types
split unambiguously). Only the inbound direction needs the table.

## Shape: an injected service, following `store.VersionService`

**Do not implement this as a `store.EntityObserver` registered on the store, and
do not add fields to an existing service.** Versioning is the precedent and the
codebase has already made this move once: `appbuild.go:934-936` notes the
recorder "takes the service rather than type-asserting the store — versioning is
a separate injected concern, not a store capability." Follow that trajectory
rather than the older observer/type-assertion shape.

Concretely:

1. **A new leaf package** (e.g. `internal/caldavalias`) owning the alias domain
type and its persistence. Needs its own `.go-arch-lint.yml` component entry plus
`mayDependOn` grants from its consumers.
2. **An umbrella service interface** used only as the nil-able wiring vehicle
threaded through `appbuild` — mirroring `store.VersionService`'s doc, which is
explicit that the umbrella "is a WIRING vehicle only … never a parameter to a
handler or command. It groups one cohesive concern, not a cross-subsystem
service locator."
3. **Narrow consumer-side interfaces at each call site**, per CLAUDE.md. The
CalDAV backend binds a lookup/record interface; the rename hook binds a
one-method rewrite interface. Nobody takes the umbrella.
4. **Wired in `appbuild.assemble`** (build-agnostic — `buildStateKV` is already
called there), nil where unconfigured, with consumers nil-checking.

### Alias record

`(collection, href, uid) → entity_id`, plus the last-served ETag for `If-Match`
conflict detection.

Note the entity TYPE is not stored: a collection declares exactly one
`entity_type` (TKT-UGYSC8), so an inbound request already knows the type from
the collection before it consults the alias. The alias answers only *which
entity*.

## Rename is the central risk

The one place old→new is knowable is the synchronous choke-point. The
entitymanager says so (manager.go:816-818): *"Only the choke-point knows
old→new; a later sweep sees the renamed entity as an ordinary update and cannot
reconstruct this link."* **A missed rename orphans the alias, and the client
sees a delete + a create — a duplicated task.**

Two routes, decide in planning:

- **`store.EntityObserver.EntityRenamed(oldID, renamed)`** (store.go:777-804) is
purpose-built — its doc names "ID-keyed observers … can rewrite those references
in one step" and guarantees rename emits **exactly this one callback**, not
delete+put. But **every firing site discards the error** (`_ =
o.EntityRenamed(...)`, fsstore.go:361, memstore.go:144), so a failed rewrite is
silent.
- **An entitymanager hook**, mirroring `VersionRecorder` (version_hook.go:25) —
a consumer-side one-method interface on `Deps`, nil-disabled, called
synchronously at rename. This is the injected shape and it can at least log.

**Note the divergence from versioning:** version capture is explicitly
best-effort ("a recorder error must never fail the underlying write"). Alias
integrity is *not* obviously best-effort — a lost alias silently duplicates a
user's task on their phone. Decide deliberately whether a failed alias write
should fail the rename, and write the decision down either way.

### Other residual risks

- **`store.Event` has no rename op** — a rename by *another process* is
indistinguishable on the event feed. fsstore must tolerate orphans; postgres
could use an FK with `ON UPDATE CASCADE`.
- **Entity IDs are case-insensitive** since migration 0007 — the alias key must
fold case identically or it fragments on macOS/Windows fsstore.

## Persistence and corruption policy

Back it with `state.KV` (`internal/state`) using **per-key** entries, not one
whole-file blob. Writes are crash-safe (`SafeFS` does temp→fsync→rename) but
there is **no file locking and no cross-process coordination** — every existing
consumer compensates with an in-process mutex plus a whole-file cache, which is
exactly what a read-modify-write alias table cannot rely on. Per-key entries
limit the clobber window.

Owning its own package means the service can encapsulate that concurrency
discipline once, rather than each consumer re-deriving it.

**Corruption must be a hard error**, following
`internal/cli/sync/state.go:53-58` — *not* the scheduler's silent-empty. A
silently emptied alias table re-creates every task as a duplicate, the same
failure `sync` guards against.

## Acceptance criteria

1. The alias service lives in its own package with its own arch-lint component;
`just arch-lint` passes.
2. Consumers bind narrow interfaces; the umbrella type appears only in wiring,
never in a handler or command signature.
3. An inbound client-created todo (bare UUID) maps to a newly created entity and
the alias survives a process restart.
4. Renaming an entity rewrites the alias; the CalDAV client continues to see the
**same** resource (no delete+create, no duplicate).
5. Deleting an entity removes its alias.
6. Case-differing entity IDs resolve to one alias entry.
7. A corrupt alias store fails loudly at startup rather than silently emptying.
8. Concurrent writes to distinct keys do not clobber each other.
9. Works on fsstore and memstore; postgres behaviour documented (FK cascade or
orphan-tolerance).
10. The rename-failure policy (best-effort vs. fail-the-write) is documented at
the interface.

## Out of scope

- The CalDAV protocol surface.
- Cross-process rename reconciliation beyond documenting the gap.
