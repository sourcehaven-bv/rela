---
id: RR-09AUCP
type: review-response
title: entityMutator's PatchEntity rationale is contradicted by the CalDAV writer three files over
finding: The claim is scoped to writeHandler but stated as a general fact about the data-entry write path
severity: significant
resolution: 'Scoped the claim: PatchEntity is absent FROM THIS HANDLER because a form save owns the whole record; and the comment now states that this is a fact about form saves rather than data-entry writes generally; naming the CalDAV writer as the patching caller.'
status: addressed
---

`entityMutator`'s doc says:

> Seven of the manager's nine write methods. PatchEntity is absent because a form
> save renders every field and therefore legitimately owns the whole record.

The reasoning is correct **for writeHandler**. But it is phrased as a general
fact about the data-entry write path, and `caldav_write.go:309,600,698` call
`entityManager.PatchEntity` on that same path — precisely because CalDAV does
*not* own the whole record.

CLAUDE.md's `duplication` rule warns about this shape: a fact asserted in one
comment that another site falsifies. Scope the claim to the handler.
