---
id: TKT-0MMJ6D1
type: ticket
title: Give the entity write handlers one shared provision-and-read gate
kind: chore
priority: low
effort: s
status: ready
---

## Description

archplan v26.11.4 reports two `structure.repeated_orchestration` findings in
`internal/dataentry/write_handler.go`
(`structure.repeated_orchestration-545a7d43c1f7` and `-fa66fb12dc42`). Four
handlers start with the same steps, each written out:

- `handleV1CreateRelation`, `handleV1UpdateRelation`,
`handleV1DeleteRelation` and `handleV1CloneEntity` call `h.withProvision(r)` and
then `readAddressedOr404(w, r, h.visible, typeName, addr)`.
- `handleV1UpdateRelation` and `handleV1DeleteRelation` then call
`familyReadableOr404(w, r, h.visible, relationTargetID(targetID))`.

The order matters: the read gate must run before the body is parsed and before
any affordance check, or a 400 or 403 confirms that a hidden entity exists
(TKT-VQGN, BUG-BZQQDP). Today each handler has to repeat that order, and a new
handler can get it wrong.

Give the steps one home on `writeHandler`: a method that provisions, reads the
addressed entity and, when a peer id is given, gates the peer. It returns the
request, the entity and whether to continue. The four handlers call it first.
Check whether the other write handlers that call `withProvision` and then read
the addressed row fit the same method.

## Acceptance criteria

1. The four handlers call one shared method for provision, addressed read
and peer gate, and no longer call the three functions themselves.
2. Responses do not change: a hidden or absent entity or peer still gives
the same 404, before the body is parsed. The existing handler and ACL tests pass
unchanged.
3. A traced `archplan ci` no longer reports the two findings.
