---
id: RR-FA1D
type: review-response
title: debounceMs precedence -- covered, but test the negative
finding: |
  Precedence is well-specified: fieldDebounceMs ?? debounceMs ?? 800. The risk is a test that accidentally passes whether the precedence is right or wrong (e.g., both set to the same value). The plan's test in row 3 already uses different values (debounceMs: 300, contentDebounceMs: 100) -- keep that. Add one more: fieldDebounceMs: 100, debounceMs: 300 and assert field fires at ~100ms, not 300ms.
severity: minor
status: addressed
resolution: |
  AC test plan amended: add explicit "per-channel-wins" test with fieldDebounceMs: 100, debounceMs: 300, assert field fires at ~100ms (not 300). Also add the inverse: contentDebounceMs override case. Uses fake timers to assert wall-clock-independent timing.
---
