---
id: RR-5B94OAL
type: review-response
title: entity_ids parsing touches every action body
finding: Parsing entity_ids for every action would change request-scoped bodies of script actions.
severity: significant
resolution: 'Plan: parse entity_ids only for llm actions with a typed decode.'
status: addressed
---
