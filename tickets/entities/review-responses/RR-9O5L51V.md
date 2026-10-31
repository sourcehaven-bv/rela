---
id: RR-9O5L51V
type: review-response
title: revertContent restores an old body while a rebase is pending
finding: While the body was held, lastSeenContent did not move; reverting after a kept-typing save restored the body from before that save and left contentRebase set.
severity: minor
resolution: revertContent adopts contentRebase.merged as the shown body and base and clears the rebase. Unit test 'reverts the body to the one its merged save wrote' fails without it.
status: addressed
---
