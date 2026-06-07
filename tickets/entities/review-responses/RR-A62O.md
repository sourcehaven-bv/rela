---
id: RR-A62O
type: review-response
title: Missing nil-rejection test for new graphQueryer constructor arg
finding: 'NewDeclarative now rejects three nil args; declarative_test.go only covers two (policy, graph). CLAUDE.md rule is explicit (''Constructors reject nil required fields''). A regression that drops the nil check goes uncaught. Fix: one table-driven test with three rows, each setting one arg to nil, asserting ErrorContains(err, ''must be non-nil'') + Nil(d). The existing TestNewDeclarative_RejectsNil in resolver_test.go already covers all three (was updated in this PR) — but the test only checks that an error is returned, not the message. Tighten with ErrorContains.'
severity: significant
resolution: TestNewDeclarative_RejectsNil rewritten as table-driven with ErrorContains checks for 'policy must be non-nil', 'graph must be non-nil', 'graphQueryer must be non-nil'.
status: addressed
---
