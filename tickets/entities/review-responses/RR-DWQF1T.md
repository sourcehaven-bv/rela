---
id: RR-DWQF1T
type: review-response
title: Channel read role can admit a reader who cannot read the anchor
finding: read roles from principal_type, permission, entity.prop or creator do not require anchor readability; trigger messages then reveal a hidden entity's existence and state; condition gates likewise
severity: significant
resolution: Read roles and conditional lifecycle gates now carry RequireAnchorRead; the read gate is attached AND source (578b40cf).
status: addressed
---
