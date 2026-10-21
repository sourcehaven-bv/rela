---
id: RR-NONZO5
type: review-response
title: Busy state leaks onto another task item
finding: setTaskItemBusy clears by text lookup, which fails after link or remove; the mapped position lands on the next item, whose button stays aria-disabled.
severity: significant
resolution: 'Busy marks are now tokens: markTaskItemBusy returns a token, clearTaskItemBusy clears by token, and a mark whose item is deleted is dropped rather than mapped onto the next item. Pinned by two editor tests (remove, then link and type).'
status: addressed
---
