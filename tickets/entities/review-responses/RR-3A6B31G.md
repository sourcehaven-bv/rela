---
id: RR-3A6B31G
type: review-response
title: Grantee can extend own valid_until without requires_permission
finding: Relation updates fall back to update on the source type when no requires_permission is set.
severity: significant
resolution: 'Plan: a when: on a relation without requires_permission fails the policy load.'
status: addressed
---
