---
id: RR-YP8UW
type: review-response
title: Long titles can blow up the selected chip — no max-width / ellipsis
finding: 'formatEntityLabel happily returns a 500-character string. The selected chip has no max-width / text-overflow: ellipsis. Not introduced by this change, but the new format makes overflow more likely (longer than just an id). Pre-existing condition; worth a follow-up.'
severity: minor
reason: Pre-existing condition (chip CSS already had no max-width / ellipsis before this change). The new format makes overflow more likely but does not introduce the bug. Out of scope for this enhancement; worth a follow-up CSS ticket.
status: deferred
---
