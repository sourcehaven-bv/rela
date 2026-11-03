---
id: RR-UN9B0H7
type: review-response
title: No-silent-loss criterion accepts stale errors
finding: After a property or body conflict, a later save of another field applies the server value over the user's value while fieldErrors/contentError stays set (useAutoSave mergeServerResponse). Criterion 3 passes on a stale error. Require the error to postdate the last edit and the form to still show the value.
severity: critical
resolution: Criterion 3 now requires the tab to still show its value with an error. It found the case in BUG-SJURXLT (unsaved value replaced on screen), now fixed.
status: addressed
---
