---
id: RR-6Y42VKH
type: review-response
title: hide_empty is not editable in the config editor
finding: configedit's allowlist has no dashboard.cards.[].hide_empty, so the in-app config editor cannot set it.
severity: nit
resolution: Classified hide_empty as locked in the configedit allowlist, beside width ("not in the Configure space yet"); configedit tests pass.
status: addressed
---
