---
id: RR-W3O7X2
type: review-response
title: Long label truncation hides the time and the +N
finding: 'RlBusyIndicator.vue:72 and :111-115 put the label, the duration and "+N" in one span with text-overflow: ellipsis. On a narrow card a long state label cuts off " · 20m +2", so the time in state (the stall signal) and the count of other busy channels disappear. The LongLabel story asserts that the overflow happens but does not check what is lost. Fix: render the label in its own span with min-width: 0 and ellipsis, and the duration and "+N" in a sibling span with flex: none. Extend the LongLabel story to assert that the duration stays visible.'
severity: minor
resolution: Split the text into a label span that truncates and a tail span (time and +N) that never shrinks; busyText became busyTail; the LongLabel story asserts the label overflows while the tail stays whole and inside the box.
status: addressed
---
