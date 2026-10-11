---
id: RR-M50WTZ
type: review-response
title: Refresh token exposed to any granted script
finding: Callback design hands the refresh token to arbitrary script code.
severity: significant
resolution: 'Plan R1: refresh moved into Go via connections.yaml; Lua sees only access tokens.'
status: addressed
---
