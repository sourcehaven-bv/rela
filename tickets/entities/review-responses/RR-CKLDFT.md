---
id: RR-CKLDFT
type: review-response
title: Commit message overstated the deleted test's coverage as lossless
finding: The commit message said the deleted e2e_test.go's "coverage is in the Playwright suite and internal/dataentry/document_test.go". Each of the three assertions is covered, but the SEAM — that a URL built in the Lua VM survives goldmark and reaches the rewriter in the right shape — is not asserted end to end by any single test. Recording it as a lossless move would mislead the next reader into believing the seam is tested.
severity: minor
resolution: Ticket now states this is a deliberate narrowing, names the uncovered seam, notes that the deleted test did not defend it either (it had not compiled for months and t.Skips without the prototype fixture), and points at the concrete way to close it (a rela.url.form_edit link in e2e/tests/fixtures.ts).
status: addressed
---
