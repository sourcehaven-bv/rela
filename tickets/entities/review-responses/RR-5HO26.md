---
id: RR-5HO26
type: review-response
title: Plan mislabels cmd.confirm origin as 'server config'; clarify it is project-config from data-entry.yaml
finding: 'cmd.confirm originates from internal/dataentryconfig/config.go (CommandConfig.Confirm), loaded from project data-entry.yaml — not runtime server output. The threat model is: an attacker who can edit data-entry.yaml can already run arbitrary Lua, so cmd.confirm trust is fine. Update the plan''s wording to say ''project config (data-entry.yaml)'' rather than ''server config'' to avoid confusion with the streaming command output (which is a different and untrusted pipeline).'
severity: significant
resolution: 'Plan wording corrected: cmd.confirm is project-config (data-entry.yaml), not runtime server output. Trust model is fine because anyone who can edit data-entry.yaml can also author the Lua scripts.'
status: addressed
---
