---
id: RR-6Y42VKH
type: review-response
title: hide_empty is not editable in the config editor
finding: configedit's allowlist has no dashboard.cards.[].hide_empty, so the in-app config editor cannot set it.
severity: nit
reason: CI showed configedit requires every key to be classified. hide_empty is now locked beside width, as "not in the Configure space yet".
status: addressed
---
