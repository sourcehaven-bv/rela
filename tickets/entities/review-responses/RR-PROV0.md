---
id: RR-PROV0
type: review-response
title: Test fixtures could use fluent builders per CLAUDE.md guidance
finding: Both newPrefixTestApp and newManualPrefixedTestApp are prime candidates for the fluent-builder pattern (testApp().withType(manualType('tag').withPrefix('TAG-')).build()).
severity: nit
reason: Suggestion-level. RR-WI06C tracks the related concern about the two helpers duplicating wiring; a fluent builder is one possible answer to that. Holding both as one consolidated follow-up so the refactor can be designed without constraints from this PR's diff boundary.
status: deferred
---
