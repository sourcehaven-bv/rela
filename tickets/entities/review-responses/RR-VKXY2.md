---
id: RR-VKXY2
type: review-response
title: 'retries=2 in CI will hide flakes from items #1/#2'
finding: With the TOCTOU + SIGTERM-wait bugs unfixed, retries will mask them. Add a weekly scheduled job running with retries=0 to surface flake.
severity: nit
status: open
---
