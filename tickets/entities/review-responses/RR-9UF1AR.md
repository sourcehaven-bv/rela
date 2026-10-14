---
id: RR-9UF1AR
type: review-response
title: Docs lack ACL grants for the subscription principal
finding: In a project with acl.yaml, system:subscription:<name> has no grants, so find sees nothing and creates are forbidden.
severity: minor
resolution: Added an Access control section to docs/feed-subscriptions.md.
status: addressed
---
