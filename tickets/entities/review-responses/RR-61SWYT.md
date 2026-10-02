---
id: RR-61SWYT
type: review-response
title: World-scoped ListEntities can yield an entity twice
finding: The world keyset compared (p.id, p.face) outside the DISTINCT ON. A face written between two pages that becomes the new prime and sorts after the old face is yielded again.
severity: significant
resolution: The keyset is now id > after inside the DISTINCT ON candidates, removing whole families. Pinned by TestIteratorPaging_WorldPrimeChangesMidIteration (mutation-checked).
status: addressed
---
