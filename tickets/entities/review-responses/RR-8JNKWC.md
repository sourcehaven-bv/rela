---
id: RR-8JNKWC
type: review-response
title: dataentry/appbuild diverge on nil-redactor handling while claiming to match
finding: 'The new scriptReader godoc claims it matches appbuild''s unattended-path wiring, but appbuild.go:257-259/289-291 substitutes visibility.NopRedactor{} for a nil redactor while dataentry returns DenyReader. Two seams documented as identical behave oppositely on the same input. Load-bearing for the tests: a future ''alignment'' refactor copying appbuild''s guard into dataentry would delete the only fault path failclosed_test.go exercises, and the tests would fail with no indication why.'
severity: critical
resolution: 'Resolved deliberately rather than by alignment. dataentry keeps the strict behavior and the godoc now documents the divergence as intentional with its rationale: appbuild''s callers (scheduler, cascades) legitimately have no affordance resolver, so NopRedactor is the best available there (RR-7408F5); data-entry always has one, so a nil redactor is a wiring bug, not a capability gap. Silently swapping in NopRedactor there would drop field-level redaction on a path required to enforce it. The comment explicitly warns against copying appbuild''s guard.'
status: addressed
---
