---
id: RR-2D1T4F
type: review-response
title: 'lineDiff: unbounded O(n·m) LCS table (UI-DoS on large content) + propertyDiff key-order false diffs'
finding: 'cranky-code-reviewer (frontend) #3+#4: (a) lineDiff.ts:23-30 materializes a full (n+1)×(m+1) LCS table synchronously on the main thread; a ~5000-line body vs 5000-line body ≈ 25M cells → hundreds of MB + multi-second freeze/OOM. Entity markdown is user-authored + unbounded — a UI-DoS. Fix: cap n·m (or line count) and fall back to a ''too large to diff'' notice; strip common prefix/suffix first. (b) propertyDiff (lineDiff.ts:88) compares via JSON.stringify which is key-order-sensitive — object-valued properties that differ only in key order report a phantom ''change''. Go''s encoding/json sorts map keys but the live entity path may not, so base-vs-current object comparisons can diverge. Fix: stable-key serialize or recursive deep-equal; tests only cover arrays (false confidence).'
severity: significant
resolution: 'Fixed both: (a) lineDiff now trims the common prefix/suffix before the LCS core and caps n·m at 2M cells, degrading to a coarse all-del+all-add block diff above that — no unbounded table / main-thread freeze. (b) propertyDiff uses stableStringify (recursive key-sorted) for equality, so object-valued properties differing only in key order no longer report a phantom change. Tests added: prefix/suffix trim, large-input fast fallback, key-order equality, nested-change detection.'
status: addressed
---
