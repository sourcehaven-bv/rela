---
id: RR-LQYCV2
type: review-response
title: Reader editorial-world test passes on an empty or broken page
finding: 'Only absence assertions: the test could not tell hidden drafts from a failed render or a fallback to the published world.'
severity: significant
resolution: Now asserts the empty state is shown after the list loads.
status: addressed
---

Only absence assertions: the test could not tell hidden drafts from a failed
render or a fallback to the published world.
