---
id: RR-HMWCKQ
type: review-response
title: Task items starting with emphasis or a link break the highlight
finding: Skipping the checkbox by jumping to the first text node landed inside ** or [.
severity: significant
resolution: The checkbox is skipped in the source ([ ]/[x] plus blanks). Segment tests and the golden fixture cover it.
status: addressed
---
