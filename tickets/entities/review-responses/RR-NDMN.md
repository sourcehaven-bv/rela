---
id: RR-NDMN
type: review-response
title: _position with hidden entities — neighbor IDs reveal gap existence
finding: 'Plan claims _position inherits the gate ''for free'' via scopedSortedEntities. The FILTER is inherited correctly. The position SAFETY is not: when the principal sees TKT-001 and TKT-003 with TKT-002 hidden, `next` from TKT-001 returns TKT-003. The gap is informational — anyone with knowledge of ID ordering (which is monotonic for ksuid prefixes) infers that something exists between 001 and 003. There is no clean fix beyond accepting it (the alternative — fake dense numbering — is worse). Plan should document this in GUIDE-acl-security alongside the SSE deferral and AC7 should add: ''neighbor IDs may exhibit gaps that reveal hidden cardinality; ordering is best-effort, only cardinality and content are hidden.'' Pin a test: with 3 entities where the middle is hidden, _position for the third returns prev=first (not second).'
severity: significant
reason: Carried over to the future '_position filter + per-id gate' ticket (referenced as a follow-up in both TKT-VQGN and TKT-VMD8). Per user direction, _position is deferred until after PR 2 lands.
status: deferred
---
