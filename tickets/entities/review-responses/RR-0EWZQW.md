---
id: RR-0EWZQW
type: review-response
title: pgstore conformance suite skips silently when RELA_TEST_DATABASE_URL is unset
finding: 'internal/store/pgstore/testdb_test.go skips every conformance subtest when RELA_TEST_DATABASE_URL is unset, and a skip is indistinguishable from a pass in CI output. This is the mechanism that guarantees backend parity, so a dropped CI secret or a misconfigured job would silently disable the entire guarantee with nothing going red. This directly enabled the critical finding RR-RGAXHK: the list-drift bug passed every local check and was only caught by manually standing up a database. Suggested fix: in the CI postgres job assert the env var is set (fail if absent) rather than trusting it, so a missing secret fails loudly instead of quietly skipping.'
severity: minor
reason: 'Out of scope for TKT-F4TIS6, which changes store query semantics rather than CI configuration. The fix belongs in .github/workflows/ci.yml (the postgres job) and affects every pgstore test, not just this ticket''s. Filing separately keeps this ticket''s diff coherent. Mitigated for now: the parity-critical paths added here were verified manually against a live PostgreSQL instance before merge, and the new Props_value_shapes conformance case will catch regressions wherever the suite does run.'
status: deferred
---
