---
id: RR-DWZKU
type: review-response
title: AC5/AC6 need concrete testable mechanisms
finding: AC6 says 'track via a counter in a test-local cache value' — not mechanizable without side channels. AC5 needs access to the stdout buffer that documentService.Render does not expose. Both ACs are not testable as stated.
severity: significant
resolution: AC5 now tests at runtime_test.go level (stdout buffer accessible). AC6 uses rela.write_file sentinel file (line count == 1 after two renders proves memoization worked).
status: addressed
---

From design-review on PLAN-78HJO.

For AC5: test at `internal/lua/runtime_test.go` level (direct runtime) rather
than through documentService — that's where the action-mode analog
TestLuaOutputActionMode already lives.

For AC6: have the script write to a file via `rela.write_file` whose line count
the Go test inspects. Or drive the runtime directly (not via documentService) in
a unit test that exposes the counter.

Rewrite AC5/AC6 test plans with the concrete mechanisms.
