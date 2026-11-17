---
id: RR-HMLVLUD
type: review-response
title: 'Design: trigger firing order is not checked or audited'
finding: Triggers fire in document (map-key) order. CheckEdit and changedPaths ignore key order so a reorder passes and is not audited; moving a scripted trigger changes what reviewed Lua sees.
severity: significant
resolution: 'The new ordered map refuses moving a kept trigger (triggers reordered). A pin with order keeps the place of a script among its outputs. The audit lists the changed paths. Tests: TestCheckEdit_Flows.'
status: addressed
---
