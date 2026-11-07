---
id: RR-36EG7RU
type: review-response
title: 'Security: range read is a raw ungated capability'
finding: A hand-built allow-all scope bypasses both gates.
severity: minor
resolution: 'Plan changed: an archguard test pins callers of RangeFingerprint and ListRange outside internal/store to an allowlist (empty in this step).'
status: addressed
---
