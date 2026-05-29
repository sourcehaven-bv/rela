---
id: RR-TL9I
type: review-response
title: Relation integration test asserts wire shape but not the 403 it claims, uses wrong relation type (S5)
finding: TestPolicyResolver_RelationCreate_WireShape comment says 'and a 403 on relation create' but only does a GET inspecting _relations — no create PATCH issued. Policy grants on depends_on (underscore) while the real type is depends-on (hyphen); passes only because the resolver echoes whatever string the policy contains (see S2). Either drive the create PATCH and assert 403, or fix the comment, and use the real relation type.
severity: minor
resolution: Renamed test to TestPolicyResolver_RelationCreate_WireAndWrite; it now drives an actual POST .../relations/depends_on and asserts 403 with rule_id=relation-affordance:not-creatable:depends_on, in addition to the GET wire shape. depends_on is the correct relation type for dataentry's testMeta (ticket->ticket). The affordances-package testMeta gained real relation defs (implements/blocks/has-planning) so its relation tests reflect a valid metamodel.
status: addressed
---
