---
id: RR-JHQKY
type: review-response
title: Per-rule runtime construction continues after ctx cancellation
finding: 'CheckRules iterates all rules even after parent ctx cancellation. After ctx is cancelled, still calls lua.NewReader(...) for each remaining rule, then PCall fails fast. Construction itself isn''t free. With 100 rules + early cancellation, that''s 99 wasted runtime allocations. Location: internal/validation/validation.go:143-184.'
severity: nit
resolution: CheckRules now checks ctx.Err() at the top of each rule iteration and bails immediately if the parent ctx was cancelled, avoiding 99 wasted runtime allocations after early cancellation. Test TestLuaValidation_CheckRulesBailsOnCancellation runs 100 rules with a pre-cancelled ctx and asserts the call finishes under 100ms. Commit 7221fa2.
status: addressed
---
