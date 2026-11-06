---
id: RR-XW9UMI4
type: review-response
title: User guide is wrong on read-only joins and restarts
finding: The SPA joins only when the principal may update the body, so the guide's read-only join statement is false. The restart statement is false as well (see the restart finding).
severity: minor
resolution: 'GUIDE-data-entry corrected: joining needs update permission, leaving saves for every editor, saving only while connected, 30 s recheck and 2 min lifetime, restart behaviour, attribution, latest-save resend. docs regenerated.'
status: addressed
---
