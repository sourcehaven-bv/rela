---
id: DOCS-CAS34X
type: docs-checklist
title: 'Docs: Optimistic concurrency at the store: expected-version on entity.Patch / UpdateEntity'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`store.UpdateEntityIf`, `store.UpdateCondition`, `store.EntityVersion`,
`store.VersionOf` and `store.VersionConflictError` each carry godoc stating the
contract rather than restating the signature. `VersionOf` documents what the
token is derived from, which is the part a backend author can get wrong.

`entity.Patch.ExpectedVersion` and `Manager.PatchEntity` document when a caller
actually needs CAS: a patch already preserves properties it does not name, so
an unconditional patch can only lose an update when two writers name the SAME
property — notably a `Content` replacement computed from a base read. Saying so
stops CAS being cargo-culted onto every patch.

The non-obvious decisions are recorded where they would otherwise be undone:

- **fsstore compares against the entity loaded from DISK**, not the in-memory
index. A hand edit or a `git pull` changes the file with no store write; a
token derived from the bytes catches that, a per-write counter would not.
- **Each backend's unconditional update routes through the conditional core**
with a zero `UpdateCondition`, so there is one update path per backend rather
than two that could drift.
- **`writePatchError` matches by `errors.As`, never on the message**, with a
note naming the failure this avoids: a broken error chain silently makes a
client's retry loop unreachable and turns every loser into a 500.
- **The PATCH handler passes the fused state ref, not the bare id** (RR-1GM1NB).
The comment explains why: `PatchEntity` resolves the face from the row it
loads, so a bare id lands on the default face — invisible at the type level,
since the parameter is a plain string.
- **`storetest.RunCASTests` is in `RunAll`, not opt-in**, and the godoc says
why: a backend that accepts a stale version does not degrade gracefully, it
silently reports success for a write that lost a race.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new architectural rule.
An optional capability added to `store.Store` and exercised by a conformance
suite is the established pattern here.)
- [x] docs/ updated for changed behaviour
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency
direction or wiring change; `arch-lint` clean.)

`docs/data-entry/api-reference.md` (ETag / If-Match) now records that the
precondition is re-checked by the store atomically with the write. The
user-visible status is unchanged — 412 either way — but an operator running
several `rela-server` processes against one database previously had a real lost
update here, and that is worth stating because the remedy (re-read, re-apply,
retry) is the same and their existing clients already implement it.

`docs/webhooks.md` is deliberately NOT changed. It names TKT-34XS2R as the
planned fix for cross-process `append_section` loss, and that limitation still
stands: the webhook pipeline still reads-compares-writes under `writeMu` and
does not pass `ExpectedVersion`. Verified by inspection of
`webhook_routes.go` and by `TestWebhookConflict_CrossProcessAppendsCanBeLost`,
which still documents the loss. This ticket delivers the store-level primitive
that fix needs; converting the webhook path to use it is separate work, and
marking the doc fixed now would be false.

## External Documentation

- [x] ~~README updated~~ (N/A: no new user-visible feature. The HTTP contract
is unchanged.)
- [x] ~~CLI reference updated~~ (N/A: no new or changed command or flag.)
- [x] API docs updated (the ETag / If-Match section, above)

## Rationale for N/A

The observable HTTP surface does not change: `If-Match` still yields 412 on
mismatch, and no request that succeeded before now fails. What changes is that
the guarantee is real under concurrency instead of depending on a process-local
mutex. That is why the doc edit is one paragraph in the existing section rather
than a new page — there is no new thing to learn, only a previously
unreliable promise that now holds.
