---
id: RR-R0VC2
type: review-response
title: Pre-existing TS DocumentConfig type drift
finding: 'DocumentConfig.command is marked required in TS but Go allows script: as an alternative. script is not declared in the TS interface at all. Pre-existing; flagged for a follow-up. Not introduced by this ticket.'
severity: nit
reason: Pre-existing TS type drift on DocumentConfig (command required, script absent). Not introduced by this ticket. Should be fixed in a focused follow-up that aligns the TS shape with the Go truth across all config types.
status: deferred
---
