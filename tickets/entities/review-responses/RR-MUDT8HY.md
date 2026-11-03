---
id: RR-MUDT8HY
type: review-response
title: '[security] Forged hints get past the 5 s resync limit via the subscriber change cap'
finding: '[security] Only gaps and resync hints are rate-limited. Many valid forged message hints with distinct keys push a subscriber past MaxSubscriberChanges and force a full round every tick. On postgres any database session can send them. Each replay is still an ACL-checked read, so the cost is load, not disclosure.'
severity: minor
reason: Needs a design choice (a received-key budget in the follower, or signed hints, or hint rows in a schema table). Tracked as backlog ticket TKT-PMNFOGP. docs/postgres-backend.md now states that any session on the database can send notifications that cause rechecks, and that this costs load, never data.
status: deferred
---
