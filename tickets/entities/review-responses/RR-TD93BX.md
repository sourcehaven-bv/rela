---
id: RR-TD93BX
type: review-response
title: Gesture and template nits
finding: Preview reassigned on every move, text selection while dragging, arrows scroll the chart during a commit, second pointer replaces the gesture, Prettier reflows in the view test, unused async in a test helper.
severity: nit
resolution: All fixed except the repeated dragWindowStyle/handleValue calls in the template, which are cheap per row. The reflows were reverted.
status: addressed
---
