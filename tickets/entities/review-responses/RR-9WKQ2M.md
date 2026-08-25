---
id: RR-9WKQ2M
type: review-response
title: GatedReads godoc overclaimed field-level coverage
severity: significant
status: addressed
finding: >-
  The new GatedReads godoc said gating is "BOTH row-level and field-level"
  without qualification, but three of gatedGraphReader's six methods
  deliberately read raw. Relation meta is not redacted: GetRelation bypasses
  the redactor, and relation-level `visible:` grants (acl.RelationGrant.Visible,
  honored on the dataentry wire via affordances.RelationFieldVerdicts) are not
  consulted here. An operator who saw relation redaction work in the UI would
  reasonably assume MCP honored it — a fail-open shaped exactly like the one
  this ticket fixes, one layer down.
resolution: >-
  Narrowed the godoc to state that field redaction covers ENTITY PROPERTIES
  only, and named the relation carve-out explicitly with the reason. Wiring
  RelationFieldVerdicts is left to follow-up work rather than silently
  widening this ticket; the point was to stop the doc promising coverage the
  code does not provide.
---

Chose the narrower doc over implementing relation redaction because the two
have different risk profiles: an inaccurate guarantee misleads immediately and
silently, while a missing feature is visible the moment someone looks for it.
Fixing the claim is strictly urgent; adding the capability is not.
