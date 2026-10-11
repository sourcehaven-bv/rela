---
id: RR-99FC0Z
type: review-response
title: 'Design: bypass and namespace gate unspecified'
finding: Unclear whether allow_acl_bypass skips the tag:<ns> permission; denial audit shape missing.
severity: significant
resolution: One authorizeTag helper; bypass skips both; namespace denials audited as denied-write (R8).
status: addressed
---
