---
id: RR-L1J8HF
type: review-response
title: Inbox computes entries it discards
finding: 'internal/realtime/inbox.go:860 calls sessionAffordances and drops entries. The cost is one Offers pass per transition per inbox row, with no I/O, so this is acceptable. A comment already explains it. Fix only if the inbox becomes hot: split out a types-only helper that shares the heldRoles lookup.'
severity: nit
reason: The reviewer rates the cost acceptable (no I/O, one Offers pass per transition) and the existing comment explains it; splitting a types-only helper is worth doing only if the inbox becomes hot.
status: wont-fix
---
