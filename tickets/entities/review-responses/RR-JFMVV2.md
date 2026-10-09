---
id: RR-JFMVV2
type: review-response
title: Desktop keychain write order inverted
finding: Put writes state before refresh; a failed second write pairs a new access token with a revoked refresh token.
severity: minor
resolution: Refresh item written first, then state; comment explains why.
status: addressed
---
