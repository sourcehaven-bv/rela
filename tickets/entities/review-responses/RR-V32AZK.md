---
id: RR-V32AZK
type: review-response
title: Plan claimed server-sent provenance for the relation triple; the page flow round-trips it through a user-editable URL
finding: The plan treated the server-computed AddInfo (relation, linkAs, peerId) as a trust anchor. In the page flow those values are serialized into router query params and return as link_relation/link_peer/link_as, so the server never sees its own AddInfo again — no signature, no nonce, no re-resolution against the offering section. A user may edit the triple to any relation and peer. The write path re-authorizes the operation (gateRead(peer) AND RelationOpCreate AND metamodel endpoint validity) but cannot re-validate provenance. This is compounded by validateRelationOp being default-permissive for undeclared relation types (affordances.go:608-611), so on a deployment with no relation verdicts, read access to an entity is sufficient to attach any metamodel-valid edge to it.
severity: significant
resolution: Dropped the provenance claim rather than implementing signing. Security Considerations now states the real boundary explicitly, notes the default-permissive verdict behaviour, records that this is pre-existing (every create form accepting link_* has it) and not widened by this ticket, and names server-side re-derivation via section identity as the deliberately-not-taken option.
status: addressed
---

## Finding

The plan asserted server-sent provenance as a security property. It is not one.

`resolveSectionButtonsWithTraverse` (`sections.go:436-495`) computes
`SectionAddInfo{Relation, LinkAs, PeerID: entry.ID}` server-side, and the plan
treated that as the trust anchor. But in the page flow those three values go
into `router.push('/form/:formId')` query params and come back as
`link_relation` / `link_peer` / `link_as`. **The server never sees the `AddInfo`
it produced again.**

A principal on `/entity/epic/EPIC-1` gets a button carrying
`link_relation=has-task&link_peer=EPIC-1&link_as=to` and may edit it to
`link_relation=owns&link_peer=EPIC-9&link_as=from`.

This matters because `validateRelationOp` is **default-permissive** when a
relation type has no verdict entry (`affordances.go:608-611`, `return nil`). On
a deployment declaring no relation verdicts — the common case — the forged
triple is accepted for any relation the metamodel permits between those endpoint
types, against any peer the principal can read.

## Resolution

Took the honest option rather than the expensive one: the claim is dropped and
the real boundary written down. The plan now records that `link_*` is fully
user-controlled in the page flow, that enforcement is `gateRead(peer) ∧
RelationOpCreate(relation, source) ∧ metamodel endpoint validity`, that
undeclared relation types are default-permissive so operators wanting
relation-level restriction must declare verdicts, and that server-side
re-derivation (carrying section identity instead of the raw triple) is the
option deliberately not taken.

Noted as pre-existing: every existing create form accepting `link_*` has the
same property, and this ticket does not widen it. What it would have done is
enshrine a guarantee that was never real, which the next reader would have built
on.
