---
id: RR-LSFGYJ
type: review-response
title: Sibling read outside Tx; UpdateRelation not nestable
finding: fs Tx is a non-reentrant mutex; reading outside races renumber.
severity: significant
resolution: 'Plan updated: Read siblings, densify/renumber, compute and write all inside UpdateRelation''s existing Tx; audit after commit.'
status: addressed
---
