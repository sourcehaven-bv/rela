---
id: RR-PL9YTZ9
type: review-response
title: Picker not reachable by Tab in decision mode
finding: With no option checked, RlSegmentedControl gives every option tabindex -1 (RlSegmentedControl.vue:79), so Tab skips the picker; the composer focus restore also looks for the checked option.
severity: significant
resolution: 'Plan 3a: with no checked option the first enabled option gets tabindex 0; story test tabs into an unchecked control (AC 7).'
status: addressed
---
