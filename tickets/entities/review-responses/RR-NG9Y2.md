---
id: RR-NG9Y2
type: review-response
title: 'SearchView regression: cannot add multiple filters on same property'
finding: lockedFilterProperties adds every active filter property to the locked set. Once a user adds status:open, the menu hides status, so they can not add status:done to OR-combine. Pre-extraction code allowed multiples (each chip got property-Date.now() so collision was timestamp-based, not property-based).
severity: critical
resolution: 'Changed SearchView.lockedFilterProperties to lock only the synthetic ''type'' option (genuinely single-valued) and let arbitrary properties be added multiple times again — restoring pre-extraction OR-on-same-property behavior. Each property chip then becomes an additional prop: clause in fullSearchQuery.'
status: addressed
---
