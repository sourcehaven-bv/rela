---
id: RR-7D0Q2T7
type: review-response
title: Prose editing in cards has no save path
finding: SectionEditForm disables the content channel, Milkdown has no conflict handling of its own, rows have no content _versions or mentions map.
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-5NFPUN0, which carries this constraint in its description. This ticket ships no prose editing in cards, so the missing save path cannot be reached in this change; it must be solved before TKT-5NFPUN0 ships.
status: deferred
---
