---
id: RR-NUBHXP
type: review-response
title: 'Design: default identity silently drops jobs'
finding: system:automation has no grants under an ACL.
severity: significant
resolution: Drop log names the principal with 'or this identity cannot read it'; docs show assigning a role. TestBackgroundAction_IdentityNeedsGrants.
status: addressed
---
