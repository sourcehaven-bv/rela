---
id: TKT-7K3BJF
type: ticket
title: 'Attachments on entity create: stage files in the create form, upload after the entity exists'
kind: enhancement
priority: medium
effort: m
status: in-progress
---

## Description

Let users attach files **while creating an entity** in the data-entry web app,
instead of the current save → reopen in edit mode → upload dance.

Today the create form renders a `file` property as a dead field.
`FileWidget.vue` gates all mutation on `canEdit = mode === 'edit' && !disabled
&& !!entityType && !!entityId`, and in create mode `entityId` is `undefined`
(`DynamicForm.vue:1742` binds `props.entityId`, which the create route does not
set). The widget therefore shows "Attachment editing unavailable."

(Note: `stagedEntity.ts` exports a `++new++` STAGED_ID sentinel, but it has no
production consumer — `entityId` really is `undefined` here, not the sentinel.
See RR-QTCKCW.)

### Why the obvious approaches don't work

Two hard constraints, both verified in the code, force the design:

1. **No attachment can be written before the entity row exists.** `AttachFile`
returns `store.ErrNotFound` unless the entity is present — enforced
independently in all three backends (`fsstore/attachment.go:47`,
`pgstore/attachment.go:43` as an in-transaction `SELECT` — there is no FK,
`memstore/memstore.go:762`) and pinned by the cross-backend conformance suite at
`storetest/attachment.go:54`.
2. **There is no ID reservation primitive.** `resolveCandidateID`
(`entitymanager/core.go:43`) has exactly three branches: client-supplied
(rejected unless the type `IsManualID()`, `manager.go:494`), a `…DRYRUN`
placeholder that is never persisted, and server-generated. Sequential IDs
(`entity/id.go:236`) are predictable, but guessing buys nothing — the attach
fails the existence check anyway. Short-ID types are crypto-random.

So the write order can only be: create entity → attach.

### Approach (Option A — client-orchestrated two-phase)

Chosen over the alternatives because it adds **no new HTTP ingress, no new blob
namespace, and no new ACL subject** — it reuses the already security-reviewed
upload path (MIME allowlist, sandboxed scan/transform, size caps,
entity-inherited ACL) verbatim.

- **Frontend only.** `FileWidget` gains a *staged* mode: when `entityId` is
absent/staged it holds the selected `File` objects in component state and
renders them (name, size, image preview, remove) without uploading.
- `DynamicForm` collects staged files per property; on successful create it
calls the existing `uploadAttachment(type, createdId, property, file)` per
staged file against the **returned** id, then refreshes the entity so the
stamped property value and `_attachments` are correct.
- Respect the property's `max`: stage at most `max` files, matching edit-mode
capacity semantics.

### Partial-failure handling (the known cost of Option A)

Create can succeed and a subsequent upload fail (413 / scan rejection /
network). This must be **surfaced, never silent**: on any upload failure,
navigate to the created entity in edit mode with the file field in error state
and the server's problem+json detail shown inline. The entity the user asked for
exists; only the file needs re-picking.

Note this window is *not novel* — `attachment.Service.WriteAttachment` already
writes bytes (step 5) before stamping the property via `UpdateEntity` (step 8),
and its orphan branch (`internal/attachment/attachment.go:203-208`) points at
`rela gc --temp-files`. Option A reaches the same window one step earlier.

### Out of scope

- **`required: true` file properties → TKT-87VSDE.** Option A cannot satisfy one
server-side: the create POST necessarily carries the property empty, so
validation either fails the create or (per DEC-HWZHA) passes with a warning, and
the file only lands on the follow-up request. That is a validation-semantics
question about when a required file is *due*, not a create-form question, and it
is strictly additive to this work — the staged widget built here is the host
site for whatever gate that ticket lands. Deliberately split so the common
(non-required) case ships.
- **Atomic create-with-attachments.** Extending `handleV1CreateEntity`
(`write_handler.go:122`, already holds `writeMu` for the whole create) with a
multipart body variant would close the window, but costs a body-shape branch,
multipart size accounting, and a compensating delete on scan rejection.
Deferred; revisit if the partial-failure UX proves insufficient.
- **A staging/unowned-blob namespace.** Would let scan/reject run before the
entity exists, but needs its own ACL subject, GC for abandoned uploads, and a
quota — a real new attack surface for a UX convenience.
- **API/MCP create-time attach.** This ticket is the SPA form only.
- **Rename leaves file property values stale.** `RenameEntity` moves attachment
*bytes* atomically (`pgstore/entity.go:487`, `fsstore/attachment.go:239`) but
nothing re-stamps the property strings, which embed the literal old id.
Pre-existing and orthogonal — worth its own ticket, not folded in here.

### Acceptance

- Creating an entity with a `file` property lets the user pick/drag a file in
the create form; after save the entity exists with the file attached and the
property stamped, in one user gesture.
- A staged file can be removed before save and is then not uploaded.
- At `max > 1`, up to `max` files stage and all upload; the add control
disappears at capacity (parity with edit mode).
- An upload that fails after create (oversize / rejected MIME) leaves the
created entity reachable, lands the user on it, and shows the server's error
inline — no silent loss, no orphaned bytes beyond the pre-existing window.
- A user without `update` on the created entity gets the same 403 the edit path
gives; the staged UI does not pretend the upload succeeded.
- Attachment uploads use only the server-returned `entity.id` — never a
client-side placeholder. (Pinned by asserting the upload call's arguments.)
- A `required` file property is **not** made worse: creating such an entity
behaves exactly as it does today (TKT-87VSDE owns improving it).

Parent: FEAT-870YCY. Sibling of TKT-RXFD5B (web write path), TKT-WLLRO7 (`max`).
Follow-up: TKT-87VSDE (required file properties).
