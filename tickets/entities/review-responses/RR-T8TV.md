---
id: RR-T8TV
type: review-response
title: TS toggler regex doesn't match renderer's checkbox-set; 'lockstep' comment misleads
finding: 'Comment claims lockstep with renderer; but renderer accepts `* [ ]`, `+ [ ]`, `1. [ ]` (ordered lists), toggler regex `/^- \[[ xX]\] /` rejects all of them. Pre-existing bug shape from the Go version, preserved bug-for-bug. Post-fix, user-visible: click on a `* [ ]` now throws and toasts ''Failed to toggle checkbox''. Either widen the regex (3 LOC) or rewrite the comment to be honest about divergence.'
severity: significant
resolution: 'Widened the regex to `/^([-*+]|\d+\.) \[[ xX]\] /` so it matches the same bullet set marked v17''s task-list extension accepts: `-`, `*`, `+`, and ordered `N.`. Verified by running marked v17 against each shape. The bracket-position computation now uses the captured bullet length so indented ordered lists with multi-digit indices work too. Comment rewritten to be honest about the CONTRACT and pin the verified marked version. Added unit tests for star/plus/ordered/multi-digit/mixed-bullets cases (now 14 tests, up from 8).'
status: addressed
---
