---
id: RR-P1QZK
type: review-response
title: Primary regression test reproduces a fast MATCH, not a hang
finding: 'The test uses ''a''.repeat(100_000) against `(a+)+$`. A string of all-`a`s MATCHES `(a+)+$` with no backtracking — confirmed 0.11ms. So with the fix reverted the test fails only on `.toBe(false)` (the guard flipping the result), NOT because anything was slow. The test comment (''reproduces the original critical scenario'', ''rejected on value length, not matched'') is misleading: it proves the guard fires, not that it prevents a hang. An honest catastrophic-backtracking test needs a non-matching suffix (''a''*n + ''!''), but that would hang WITHOUT the fix at n~40 regardless of the cap — which is exactly what exposes the critical finding.'
severity: significant
resolution: 'Confirmed independently: `(a+)+$` vs 100k all-`a`s returns true in 0.11ms (it MATCHES; no backtracking), so the test proved only that the guard fired. The misleading test is deleted along with the approach it defended. The rewritten suite is honest about what it pins: tests now assert the parse-time REJECTION of data-sourced patterns (`form.v =~ form.pat` / `entity.pat` / non-string literals), which is the actual control. Verified 3 of 4 new tests fail against the unmodified code. The dishonest ''reproduces a hang'' framing is gone — the module doc now states plainly that no cap could stop a catastrophic pattern (`(a+)+$` blows up at ~40 chars) and that what contains ReDoS is requiring a trusted literal.'
status: addressed
---
