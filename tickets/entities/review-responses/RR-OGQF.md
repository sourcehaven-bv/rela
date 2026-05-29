---
id: RR-OGQF
type: review-response
title: Empty-list YAML grant semantics ambiguous
finding: 'fields: {ticket: []}, fields: {ticket: null}, fields: {ticket:} parse differently in YAML and the plan didn''t say which the resolver distinguishes. One operator writes ''no grants for ticket fields,'' another writes ''empty allowlist,'' both deploy, one is silently wrong.'
severity: critical
resolution: 'Explicit decision: fields: key absent OR fields: {ticket: null} OR fields: {ticket:} → NOT opt-in (permissive). fields: {ticket: []} → opt-in with zero grants (closed-world deny-all). The PRESENCE of the per-type key signals intent. Matches write: [] meaning ''grants no writes.'' Added tests for all four YAML shapes.'
status: addressed
---
