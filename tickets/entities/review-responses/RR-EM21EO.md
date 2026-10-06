---
id: RR-EM21EO
type: review-response
title: One failing world fetch empties the relation picker
finding: The per-world list fetches ran in the same try as the ambient fetch; one rejection skipped the assignment and left candidates empty; which also starves buildOutgoingTypes (BUG-HOB9BR shape).
severity: significant
resolution: Widened fetches now run through Promise.allSettled; a rejected world is logged and left out; the ambient rows are still offered. Pinned by 'still offers the ambient rows when another world fails to load'.
status: addressed
---
