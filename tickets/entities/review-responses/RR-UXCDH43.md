---
id: RR-UXCDH43
type: review-response
title: Seen set covers only the first inbox page
finding: diffInbox replaced the seen set with the ids of the 50-row page, so rows moving back onto the page notified late and rows leaving it were forgotten.
severity: significant
resolution: 'The seen set now keeps earlier ids (latest read first, capped at 500), and a row older than the previous read''s newest server time less 60 s does not notify. Both times are server times. Known limit, documented in inboxNotify.ts: with more than 50 waiting sessions a new one beyond the first page does not notify. Tests: row comes back, late row, waiting re-entry, cap.'
status: addressed
---
