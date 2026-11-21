---
id: RR-6Y42VKH
type: review-response
title: hide_empty is not editable in the config editor
finding: configedit's allowlist has no dashboard.cards.[].hide_empty, so the in-app config editor cannot set it.
severity: nit
reason: The allowlist is default-locked and also leaves out width and permission on cards; adding one display key there is a separate decision for the config editor.
status: wont-fix
---
