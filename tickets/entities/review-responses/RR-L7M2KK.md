---
id: RR-L7M2KK
type: review-response
title: Abandoned-lease test depends on kvlease layout without build tag
finding: the test hard-codes the fs cache layout and would fail under postgres sqlite or memorybackend tags
severity: minor
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
