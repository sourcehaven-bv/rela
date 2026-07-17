---
id: RR-TZ2S3G
type: review-response
title: B1/B-checks must skip the '*' wildcard sentinel (not an entity type)
finding: 'B1 (grant references undeclared entity type) iterates create/update/delete/read lists, which legitimately contain ''*'' (the wildcard, handled by grantsVerb''s t=="*" branch). ''*'' is not an entity type — B1 must skip it or it false-positives on every wildcard role. The plan''s edge-cases note mentions this for B1; make it a hard requirement across ALL Tier-B checks that read grant lists (B1) and ensure the fakeMetamodelReader test includes a wildcard role asserting zero B1 findings. Also: affordance keys (fields/visible/options/relations maps) are keyed by entity type with NO wildcard — those keys CAN be checked against the schema directly.'
severity: minor
resolution: 'Plan now mandates B1 (and any verb-list check) skip the ''*'' sentinel; affordance map keys (no wildcard) checked directly. Mandatory test: wildcard role → zero B1 findings.'
status: addressed
---
