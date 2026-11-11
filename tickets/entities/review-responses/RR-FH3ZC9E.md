---
id: RR-FH3ZC9E
type: review-response
title: FlowControl sits in appbuild
finding: The authorization layer for starts and cancels lives in the wiring package rather than in its own package.
severity: significant
reason: 'It is the wiring-side access layer, like leaseGate: it needs the declarative ACL and the gated reader, which only appbuild assembles. A separate package would hold one type with one consumer and would need the same appbuild internals.'
status: wont-fix
---
