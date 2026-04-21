---
id: RR-O1UMW
type: review-response
title: E2E spec asserts on free-form error prose via regex
finding: forms-id-controls.spec.ts:133 uses expect(body).toMatch(/must start with.*MOD-/) and the duplicate-ID spec uses body.toContain('already exists'). Both couple the test to specific server message wording. Reworking the error text — e.g., the 'must start with' → 'must start with X and include a suffix' change introduced by RR-764AR — silently breaks tests that grep prose.
severity: significant
reason: 'Real concern but the structured-error shape needed to assert cleanly (e.g., a stable error type URL like https://rela.dev/errors/invalid-id-prefix plus an allowed_prefixes extension field) does not yet exist in the API — the current problem+json responses only carry {type: ''validation_failed'', title: <prose>, detail: <prose>}. Migrating tests to assert on type alone loses information; migrating to assert on a structured allowed_prefixes field would require API additions that are outside this ticket''s scope. Filing a follow-up to introduce typed problem+json error codes; once those land, both this spec and the frontend error-renderer can switch to structured assertions in lockstep.'
status: deferred
---
