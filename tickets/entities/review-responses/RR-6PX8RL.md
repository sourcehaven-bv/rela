---
id: RR-6PX8RL
type: review-response
title: lease_id is a bearer secret in URLs and plaintext storage
finding: 'Security design review: lease_id in a path segment reaches logs; FSKV writes 0644; renew/release by id alone lets a leaked id act.'
severity: significant
resolution: 'Plan: lease_id in the JSON body only, 128-bit crypto/rand, only sha256 stored, renew/release need id plus same identity, never in responses, frames, list_leases or logs (AC8).'
status: addressed
---
