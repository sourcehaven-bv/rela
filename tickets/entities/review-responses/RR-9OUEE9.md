---
id: RR-9OUEE9
type: review-response
title: Scripts writing action-reserved fields break
finding: Scripts writing fields read-only to users fail after upgrade.
severity: significant
resolution: Action scripts keep the default handle. Scheduled tasks are gated; acl-security.md has an upgrade note with the closed-list caveat.
status: addressed
---
