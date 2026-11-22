---
id: RR-UX4P9T6
type: review-response
title: "The gutter landed on a hidden cell"
finding: "The compressed row put the right gutter on `:last-child`, which is a hidden secondary cell whenever the last column is not kept, so the last visible cell met the screen edge."
severity: significant
resolution: "The row marks the last kept column with `rl-table-row__cell--last-kept` and the gutter rule targets that class."
status: addressed
---
