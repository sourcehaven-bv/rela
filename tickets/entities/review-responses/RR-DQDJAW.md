---
id: RR-DQDJAW
type: review-response
title: gateHop drops unknown read-query fields silently
finding: gateHop copied only Props and HasInbound; any other GraphQuery field would be dropped and the traversal would be wider than a plain read.
severity: minor
resolution: 'foldsIntoEndpoint refuses a read query using any field beyond EntityType, Props and HasInbound. Test: TestFoldsIntoEndpoint.'
status: addressed
---
