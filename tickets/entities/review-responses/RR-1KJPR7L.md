---
id: RR-1KJPR7L
type: review-response
title: One damaged message fails the whole inbox
finding: Plan let a GetMessages error fail the inbox call; listings elsewhere skip damaged rows with DamageWarner. A torn segment would 500 the page and the badge for everyone waiting on that channel.
severity: significant
resolution: GetMessages skips damaged messages and reports them through DamageWarner; only I/O errors fail, and then only the inbox page (the badge never runs the message step). AC 2 extended.
status: addressed
---
