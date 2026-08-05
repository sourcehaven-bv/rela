---
id: TKT-IZGF7T
type: ticket
title: Unify the two entity-ID validators into one enforced rule
kind: refactor
priority: high
effort: m
status: ready
---

## Description

Entity ID validation is implemented twice, with materially different rules, and
the **looser** implementation is the one actually enforced at the storage
boundary.

- `entity.ValidateID` (`internal/entity/id.go:134`) — strict:
`^[A-Za-z0-9_-]+$`, plus rejects `..`, `/`, `\`, and `--`.
- `storeutil.ValidateID` (`internal/store/storeutil/storeutil.go:29`) —
loose: rejects only empty, `--`, `/`, `\`, and ASCII control chars.

`entity.ValidateID` only runs on the manual-ID path (`entitymanager/core.go:50`)
and on rename (`entitymanager/rename.go:47`). Generated IDs skip it. The
importer (`internal/importer/importer.go:312`) and the fsstore write paths reach
only the loose `storeutil` check.

Verified end-to-end against memstore: entities with IDs `a;b`, `a$(id)`, `a b`,
`a*b`, `аdmin` (Cyrillic homoglyph) and `-rf` are all created and retrieved
successfully.

`FEAT-CO4YP` already declares ID validation to be a backend-independent
invariant living in `storeutil` — so `storeutil` is the intended home, and the
divergence is drift from that design, not an intentional split.

### Why it matters

A hostile ID reaching the store becomes the input to
`internal/dataentry/document.go:renderCommand`, which splices `{id}` into a
shell string run via `sh -c`. Today the only thing standing between a stored ID
and that shell is `isSafePathSegment` — a third, separate validator. A leading
`-` passes all of these (see the related CLI-safety ticket), producing argument
injection into the operator's render command.

### Scope

IN scope:
- One ID validator, enforced at the store boundary so no write path can
bypass it.
- Reject leading `-` (argument-injection safety).
- Decide and implement the handling for IDs already on disk that the
unified rule would reject.

NOT in scope:
- Removing `{id}` from the `sh -c` string (separate ticket — the
security-by-design fix).
- Relaxing to non-ASCII IDs. Staying ASCII-only keeps Unicode confusables
and normalization out of the threat model entirely.

### Back-compat constraint

There is **no content-migration machinery** — `internal/migration` is
schema-only (`yaml.Node` transforms over project config files). So a
strict-everywhere rule could make an existing project fail to load.

"Lenient on read, strict on write" is NOT acceptable: a read-modify-write of an
entity with a legacy ID would then fail on save, which is a worse failure than
failing on load.

The viable option is strict everywhere plus a repair path built on the existing
`Manager.RenameEntity` primitive (which already re-keys incident relations
atomically) and the existing `rela rename` CLI surface.

### Acceptance criteria

1. Exactly one `ValidateID` implementation remains; the other is removed
(not merely delegated to) and no caller can reach a laxer check.
2. Every write path — manual ID, generated ID, rename, importer, direct
store use — is gated by that one rule.
3. Leading `-` is rejected.
4. Storetest conformance (`storetest/fuzz.go` uses ValidateID as its
validity oracle) passes for every backend.
5. A project containing a now-invalid ID has a documented, tested repair
path; loading such a project does not silently corrupt data.
