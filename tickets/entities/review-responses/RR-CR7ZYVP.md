---
id: RR-CR7ZYVP
type: review-response
title: A stale load comes back as a plain refresh
finding: markStale does not record the read kind; kick always calls refreshView, so a stale background loadView loses commands, scope navigation and comments, and a stale first load loses the owner redirect and can leave the previous entity on screen.
severity: significant
resolution: 'markStale(full) remembers a full reload; kick honours it. Unit test: a load that runs out of retries ends in a full reload.'
status: addressed
---
