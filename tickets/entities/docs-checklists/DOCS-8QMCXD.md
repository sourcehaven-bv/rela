---
id: DOCS-8QMCXD
type: docs-checklist
title: 'Docs: webhook principal_type and acl map --expect (TKT-J3RDJ4)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (why the route clears the ctx Request, why principal_type is derived from the reserved name, why the map clamps everyone and asserted grants)
- [x] Function/type docs if public API (principal.Webhook, IsWebhookUser, Request.ClampedEveryoneGrants, ClampedAssertedGrants, CeilingBaseline, aclmap.MapWebhook, WebhookMapping, WebhookCheck, Extras, aclaudit.CheckWebhooks)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change; the feature lives in the webhook and ACL guides)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule; the ceiling rule in .claude/rules/acl-ceiling.md still holds)
- [x] Help text accurate (if CLI changes) (acl map --webhook and --expect flag help, the acl map command help and the ACLMapCmd doc)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; the upgrade impact of A15 is described in GUIDE-acl-security)
- [x] API docs updated (if applicable) (docs/webhooks.md ACL section with the baseline, what is and is not capped, the expect format and exit codes; GUIDE-acl-overview baseline selection; GUIDE-acl-security A15 and the A11 pair)
