---
id: RR-Y3J0L2D
type: review-response
title: Drawer lifecycle unspecified
finding: No behaviour for gone, changed, close after send, focus on close, and the mark-read side effect.
severity: minor
resolution: 'Specified: startSent closes, gone closes and refetches, changed refetches, focus returns to the row or the next row; mark-read behind the drawer documented. Tests in InboxView.test.ts.'
status: addressed
---
