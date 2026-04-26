---
id: RR-3U2JY
type: review-response
title: Loose timeout/cancellation assertions
finding: 'AC8 asserts elapsed > 2s (defeats the ''ScriptError appears within ~150ms'' intent). AC4 allows up to 14s wall clock. RunValidationString_HonorsTimeout asserts elapsed > 5s is false — passes at 4.9s. All three test the wrong invariant. Location: internal/validation/lua_timeout_test.go:96-99 and :53-57; internal/lua/runtime_test.go:2645-2647.'
severity: significant
resolution: 'Tightened three loose assertions: AC8 cancellation elapsed bound is now 500ms (was 2s), AC4 wall-clock is 2*validationTimeout + 500ms (was +4s), and RunValidationString_HonorsTimeout is 500ms (was 5s). Each bound now reflects the documented intent. Commit 7221fa2.'
status: addressed
---
