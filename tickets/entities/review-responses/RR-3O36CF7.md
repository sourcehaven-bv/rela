---
id: RR-3O36CF7
type: review-response
title: 'Security: a new trigger can fire an unchanged trigger that writes ACL names'
finding: Cascades chain; a new trigger can set a value that fires an existing trigger B, skipped because it is unchanged, which writes an ACL name.
severity: significant
resolution: 'appbuild.CheckTriggerACL judges each added or changed trigger against the parsed acl.yaml (checked against the draft schema) and the editor''s request; configedit calls it through the TriggerChecker interface before any early return. A record a judged trigger creates fires the created triggers of its type; each unchanged trigger reached that way is held to the script, template, replace and write rules, editor included, and to the reader rule against the edited trigger''s subjects, to a fixed point. set and relation events fire no trigger, so creation is the only route. Tests: "a created record fires an unchanged scripted trigger", "a second hop creates a record read more widely".'
reason: Needs a cascade-closure analysis over the parsed acl.Policy, which configedit cannot import. It also depends on a project already having a trigger that writes ACL names. Tracked in TKT-CB1YV7D; docs state config:edit is administrator access.
status: addressed
---
