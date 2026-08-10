---
id: RR-0QWLRC
type: review-response
title: PatchEntity omits the IsLocked (git-crypt) guard that ApplyEntity has
finding: 'The plan lists IsLocked as an out-of-scope ''noted gap'', but PatchEntity is precisely the shape the guard exists for. ApplyEntity guards it at apply.go:85-91; dataentry guards it at write_handler.go:335. PatchEntity does a raw read-modify-write: it reads the stored entity, clones it, merges, and writes it back. If the entity is locked (git-crypt encrypted, entity.Inaccessible non-empty, entity.IsLocked() at entity.go:111), the read yields a shell whose real property values are unavailable — and writing that shell back persists the cleartext shell OVER the encrypted content. That is the same data-destruction class the whole ticket exists to prevent, just via encryption rather than redaction. UpdateEntity not having the guard is not a licence to omit it: UpdateEntity receives a caller-constructed entity, whereas PatchEntity owns the read, so it is the correct place to enforce it. Add the guard and pin it with a test.'
severity: significant
resolution: |-
    ACCEPTED — moved from out-of-scope to IN scope. PatchEntity adds an IsLocked() guard immediately after the raw GetEntity and before authorize, reusing ApplyEntity's error (apply.go:85-91) for consistency.

    The reviewer's argument is decisive: a locked (git-crypt) entity reads as a shell whose real property values are unavailable, so a read-modify-write persists that cleartext shell OVER the encrypted content. That is the same erasure class the ticket exists to prevent — via encryption rather than redaction. Because PatchEntity owns the read internally (the whole point of the design), consumers can no longer perform this check themselves, making PatchEntity the only correct place for it.

    Test: patching a locked entity returns the lock error and leaves the stored bytes byte-identical.
status: addressed
---

## Why this matters more than the plan assumed

The ticket's core thesis is *"forgetting a property yields a no-op, not an
erasure."* A locked entity is the encrypted analogue of a redacted one: the read
returns something that is **not** the full stored state. Writing it back is
exactly the read-modify-write erasure the primitive is supposed to make
impossible.

Since `PatchEntity` owns the read internally (that is the whole design), it is
the natural and only correct place for the guard — consumers can no longer
perform the check themselves because they no longer hold the entity.

## Evidence

- `internal/entitymanager/apply.go:85-91` — `ApplyEntity`'s guard.
- `internal/dataentry/write_handler.go:335` — dataentry's guard.
- `internal/entity/entity.go:111` — `IsLocked()`; `Inaccessible` field at
`entity.go:57`.

## Resolution

Add the `IsLocked()` check immediately after the raw `GetEntity`, before
authorize. Reuse `ApplyEntity`'s error for consistency. Test: patching a locked
entity returns the lock error and leaves the stored bytes untouched.
