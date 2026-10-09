---
id: RR-ZGUO49
type: review-response
title: Owner key unstable across person rename and lookup fallback
finding: principal.User becomes the person entity id after principal_property resolution and silently falls back to the raw value when the lookup fails, so piles could split across two owner keys or vanish on a person rename.
severity: significant
resolution: 'Revised after user input: the owner is principal.User so automations can push by person entity id ({{new.assignee}}). The rename hook rewrites owners when a person entity is renamed; deleting the person drops their piles. system:* and unknown owners are refused.'
status: addressed
---
