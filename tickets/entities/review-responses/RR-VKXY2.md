---
id: RR-VKXY2
type: review-response
title: 'retries=2 in CI will hide flakes from items #1/#2'
finding: With the TOCTOU + SIGTERM-wait bugs unfixed, retries will mask them. Add a weekly scheduled job running with retries=0 to surface flake.
severity: nit
reason: 'Nit. retries=2 in CI masks bugs from item #1/#2 which are now fixed. A weekly flake-hunt job is valuable but out of scope for this ticket; can be added if flakes reappear.'
status: deferred
---
