---
id: RR-X3O9RR
type: review-response
title: 'link_as semantics were inverted between server and form: incoming sections wrote a backwards edge and could not link to prefix-less ids'
finding: 'Found by manual verification against the demo project; no unit test covered it. The server defines link_as as the role of the NEW entity (sections.go:461-464: "new entity is the source (incoming to entry)" for FollowIncoming). DynamicForm read it as the role of the PEER ("for link_as=from we need peer --relation--> new_entity", :1494). These are opposites. Two concrete defects followed. (1) The payload prefill ran for BOTH directions, so an incoming section put the peer in `relations` — which the server writes as new --relation--> peer — AND then issued a reverse call, producing a duplicate backwards edge on any relation where both directions are legal. (2) The reverse call was addressed from the peer, so it needed the peer''s TYPE derived from its id prefix; the demo project''s category ids are `backend` and `devops` with no prefix, so getTypeFromId returned undefined and the link was refused. The user saw "TKT-007 was created, but linking it failed" for an edge the payload had in fact already written correctly — a false failure report on top of a real inversion. This was latent because the side panel emitted param names the form never read (AC8), so no caller exercised either direction.'
severity: critical
resolution: Adopted the server's definition (link_as = the new entity's role) and fixed the form. The prefill now runs only for link_as=to, where the payload's `relations` map genuinely expresses the edge. The reverse call for link_as=from is addressed from the NEW entity — createRelation(entity.type, entity.id, relation, peer) — which is both the correct direction and removes the peer-type lookup entirely, so a prefix-less peer id now links fine. Pinned by two new tests in DynamicForm.embedded.test.ts covering the previously untested from direction, both mutation-verified against the old code. Re-verified end to end in the browser.
status: addressed
---

## Finding

Found by **manual verification** against the demo project. No unit test covered
it, which is how the inversion survived planning, implementation and two review
rounds.

The server defines `linkAs` as the role of the **new** entity
(`sections.go:461-464`: "new entity is the source (incoming to entry)" for
`FollowIncoming`). `DynamicForm` read it as the role of the **peer** ("for
`link_as=from` we need `peer --relation--> new_entity`", `:1494`). Opposites.

Two defects followed:

1. **A duplicate backwards edge.** The payload prefill ran for *both*
directions, so an incoming section put the peer in `relations` — which the
server writes as `new --relation--> peer` — *and* then issued the reverse call.
On a relation where both directions are legal, that is two edges, one of them
wrong.
2. **Prefix-less peer ids could not be linked.** The reverse call was addressed
*from the peer*, so it needed the peer's type derived from its id prefix. The
demo project's category ids are `backend` and `devops`, with no prefix, so
`getTypeFromId` returned undefined and the link was refused.

The visible symptom was a **false failure**: "TKT-007 was created, but linking
it failed" for an edge the payload had already written correctly. My own error
surfacing (RR-8SP2UG) is what made it visible rather than silent.

Latent because the side panel emitted param names the form never read (AC8), so
until that was fixed no caller exercised either direction.

## Resolution

Adopted the server's definition and fixed the form:

- The prefill now runs **only** for `link_as=to`, where the payload's `relations`
map genuinely expresses the edge.
- The `link_as=from` call is addressed from the **new** entity:
`createRelation(entity.type, entity.id, relation, peer)`. That is the correct
direction *and* removes the peer-type lookup entirely — the new entity's type is
known outright — so a prefix-less peer id links fine.

Pinned by two new tests in `DynamicForm.embedded.test.ts` covering the
previously untested `from` direction, both mutation-verified against the old
code. Re-verified end to end in the browser.

**Process note:** this is the second finding in this ticket that only manual
verification caught, after the `pickerTypes` abort (RR-Q99JT6). Both live in the
same seam — the pre-link contract between a button and the form it opens — which
had no end-to-end coverage because its one caller was broken. The e2e test in
the test plan is what stops the next one.
