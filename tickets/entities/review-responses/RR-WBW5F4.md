---
id: RR-WBW5F4
type: review-response
title: Unknown principal refusal disables piles on Windows desktop and plain servers
finding: SystemUser reads only $USER (unset on Windows) and rela-server without a principal header stamps unknown, so piles would 403 in common setups.
severity: significant
resolution: 'Plan: SystemUser falls back to %USERNAME%. A server-computed piles_available flag rides on the existing bootstrap payload so the SPA hides piles rather than discovering 403s. The e2e server sets RELA_DATAENTRY_USER.'
status: addressed
---
