---
id: RR-5JE9KM
type: review-response
title: Focus lost after a keyboard commit
finding: The commit cleared every verdict, unmounting the focused slider.
severity: significant
resolution: Verdicts are kept (set from the commit's fresh read; false on 403). The keyed handle survives the reload; view test asserts focus stays on the move slider.
status: addressed
---
