---
id: RR-O7UQ4B4
type: review-response
title: Closing the drawer does not restore focus
finding: InboxView sets answering = null on close; the plan says focus returns to the opening row or the next row if it left. When a refetch removed the row focus falls to body.
severity: minor
resolution: InboxView remembers the row order at open; on close focus goes to the row link, else the next row, else the last row, else the waiting heading. Four InboxView tests.
status: addressed
---
