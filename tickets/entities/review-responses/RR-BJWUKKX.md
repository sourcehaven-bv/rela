---
id: RR-BJWUKKX
type: review-response
title: 'Design: identity memo must key on the raw token and fall back to verification'
finding: Key the memo on the raw token string with an unexported ctx key, and verify normally when no memo matches. Match tenant-free routes on the cleaned path.
severity: minor
resolution: Memo keyed on the raw token with fallback to real verification; tenant-free paths judged on the cleaned path. Pinned by TestHost_TenantGateReusesVerification and the dot-segment cases of TestHost_FailsClosed.
status: addressed
---
