---
id: TKT-87VSDE
type: ticket
title: 'Required file properties: decide when a required attachment is due, and enforce it'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

A `file` property declared `required: true` has no coherent meaning on the
create path today, and the create-time attachment work (TKT-7K3BJF) makes the
gap reachable rather than theoretical.

**This is latent, not a live bug.** No in-tree schema currently declares a
required `file` property, which is why TKT-7K3BJF was allowed to ship without
solving it.

## The problem

`required` is validated generically in `Metamodel.ValidateProperties`
(`internal/metamodel/validation.go:147`) — a property is present-and-non-empty,
or it is an error. There is no file-type special case.

But an attachment cannot exist at create time. The bytes can only be written
after the entity row exists (`AttachFile` returns `store.ErrNotFound` otherwise
— `fsstore/attachment.go:47`, `pgstore/attachment.go:43`,
`memstore/memstore.go:762`, pinned by `storetest/attachment.go:54`), and the
property is only stamped afterwards by `attachment.Service.WriteAttachment`
(`internal/attachment/attachment.go:200`).

So the create POST *necessarily* carries the property empty. `required: true`
therefore means the create either:

- hard-fails, making the property impossible to ever satisfy through the web
form; or
- passes with a soft warning (DEC-HWZHA), making `required` decorative on this
one property type.

Neither is a decision anyone made deliberately — it is a consequence of the
write ordering.

## The actual question

**When is a required attachment due?** The candidate answers are genuinely
different products, and this ticket should pick one before any code:

1. **Due at create.** Enforced client-side: the create form blocks Save until a
file is staged. Honest to the user, but the server still cannot enforce it, so
it is a UI convention an API/MCP client bypasses. Cheapest.
2. **Due eventually.** `required` on a file property means "this entity is
incomplete until a file lands" — a soft warning that persists on the entity and
surfaces in validation/analyze output until satisfied. Consistent with
DEC-HWZHA's existing "temporarily invalid state" stance, and enforceable
server-side because it is checked on read/analyze rather than on write.
3. **Refuse the declaration.** Reject `required: true` on a `file` property as a
schema **load error**, on the grounds that it cannot be honoured. Smallest
surface, and arguably the most honest — but it forecloses a real use case (an
evidence field that genuinely must be filled).

Option 2 looks closest to how rela already treats partially-valid entities, but
this needs a decision, not an assumption — likely a `decision` entity.

## Scope

- Pick and document the semantics (probably a linked `decision`).
- Implement whichever gate that implies. For (1) that is a client-side gate in
the staged `FileWidget` TKT-7K3BJF builds — that widget is the host site. For
(2) it is validation/analyze surfacing. For (3) it is a loader check plus a
clear error.
- Cover the API/MCP path too, or explicitly state that it is out of scope and
why (a UI-only gate is bypassable by design).

## Out of scope

- Create-time attachment UX generally — TKT-7K3BJF.
- Atomic create-with-attachments (multipart create), which would make option (1)
server-enforceable but is deferred there for independent reasons.

## Acceptance

- The chosen semantics are written down with their rationale, and the two
rejected options recorded so this is not re-litigated.
- A schema declaring `required: true` on a `file` property behaves per that
decision, with a test pinning it.
- The behaviour is the same whether the entity is created via the web form, the
API, or MCP — or the difference is deliberate and documented.

Blocked by / follows: TKT-7K3BJF. Parent: FEAT-870YCY.
