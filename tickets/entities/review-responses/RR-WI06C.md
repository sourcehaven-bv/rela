---
id: RR-WI06C
type: review-response
title: newPrefixTestApp and newManualPrefixedTestApp duplicate setup
finding: The two test fixture constructors differ only by the metamodel; both copies of FS/ctx/broker wiring are ~20 lines. Drifts over time.
severity: minor
reason: Real but cosmetic. The two helpers are stable and each is used by ~5 tests in the same file. A fluent builder per CLAUDE.md guidance is the right destination but is best done as a pass over the whole api_v1_test.go (and arguably the rest of the package) rather than carving out two helpers in this PR. Filing as a follow-up so the refactor can be rationalised across helpers consistently.
status: deferred
---
