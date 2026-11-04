---
id: RR-IRWA2HI
type: review-response
title: Entry roll-ups outside a totals section go stale after a row save
finding: Only a section with totals reloads the view; a roll-up shown elsewhere on the page stays old until the next reload.
severity: minor
reason: The agreed design reloads only for sections with totals (RR-PIG9KY8). Widening it to any rolled-up entry property is a separate change.
status: deferred
---
