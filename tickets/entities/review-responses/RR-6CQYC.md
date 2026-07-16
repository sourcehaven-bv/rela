---
id: RR-6CQYC
type: review-response
title: TransitionVerdicts has no production callers yet (dormant); not disclosed
finding: 'TransitionVerdicts is called only from tests; no dataentry handler emits it on the wire. The query is plumbed but dormant. Defensible for an incremental ticket, but the commit/docs read as if it ships; findings #1/#2/#4 are only latent because nothing calls it.'
severity: significant
resolution: 'Disclosed explicitly: this ticket delivers the resolver method + accessor (the DATA), scoped OUT the wire surface and SPA control per the ticket''s Scope section. Documented in the implementation checklist and the follow-up note. The wire/_transitions surface + SPA control is the linked consumer ticket. Not a code defect - a scoping disclosure the checklist now makes explicit.'
status: addressed
---

## Finding

`TransitionVerdicts` is called only from tests; no handler emits it on the wire.
The read query is dormant. That's fine for an incremental ticket, but the
framing should say so — the guard/drift questions are only *latent* until it's
wired.

## Resolution

Not a code defect — a scoping disclosure. The ticket's Scope section already
places the wire surface + SPA control out of scope (separate consumer ticket);
the implementation checklist now states the query ships dormant. The criticals
(#1/#2) were fixed regardless, so wiring it later is safe.
