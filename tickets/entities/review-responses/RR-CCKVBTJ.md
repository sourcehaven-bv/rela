---
id: RR-CCKVBTJ
type: review-response
title: One error per request when the session ends
finding: Every open request gets its own error; nothing offers one reload prompt.
severity: minor
resolution: 'Addressed after the rebase on BUG-9DR9867: an HTML answer now also starts the session-ended probe, which shows one session-ended screen when the status route is redirected.'
status: addressed
---
