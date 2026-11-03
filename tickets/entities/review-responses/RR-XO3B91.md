---
id: RR-XO3B91
type: review-response
title: Background and allow_acl_bypass automations escape the ceiling
finding: 'background: true actions re-stamp system:automation or run_as; allow_acl_bypass scripts bypass the gate.'
severity: significant
resolution: 'Documented in docs/webhooks.md and docs/acl-overview.md. The map does not list them: aclmap has no automation config, and the docs name the two escape routes.'
status: addressed
---
