---
id: RR-WOSDYI
type: review-response
title: 'analyze check: schema changes meaning under the gate'
finding: schema.NewStoreCounter now counts through the gated reader; so a hidden type reads as unused
severity: significant
resolution: 'Kept: a hidden type must not look used. Documented in docs/acl-security.md next to the other gated counts. Count errors already surface through StoreCounter.Err.'
status: addressed
---
