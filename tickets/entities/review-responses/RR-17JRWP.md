---
id: RR-17JRWP
type: review-response
title: AC1 'byte-identical to today' is untestable as written
finding: 'AC1 says behaviour with no comments: block is ''byte-identical to today — no routes served, no storage touched''. ''Byte-identical'' has a precedent in the repo (readonly_write_route_invariant_test.go asserts a byte-identical store), but here it is asserted about overall behaviour, which no single test can check. Restate as the two things actually verifiable: (a) GET/POST on the /_comments/ routes return 404 when the block is absent, and (b) no comment storage file/table is created. Both map onto existing probe patterns (router_walk_test.go).'
severity: nit
resolution: 'AC1 restated as the two verifiable facts: /_comments/ routes return 404 when the block is absent, and no comment storage file is created. Both map onto existing probe patterns (router_walk_test.go).'
status: addressed
---
