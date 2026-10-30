---
id: RR-RIA9NZ
type: review-response
title: Make selectsBusy independent of the round size
finding: collect calls selects for every subscription while it holds h.mu and c.mu (live.go:842-860). Step 4 loops over `f.moved[w.typ]` per busy subscription, up to MaxSubscriberChanges (1024) keys each. Build f.moved as `map[type]map[anchor]bool` once in addMoved. selectsBusy can then range over whichever of the wake anchors (at most 200) and the round's anchors for the type is smaller, and the lock hold time no longer scales with subscriptions × moved keys.
severity: nit
resolution: 'Adopted: addMoved builds map[type]map[anchor]bool once per round, and selectsBusy ranges over the smaller of the wake and the round anchors for the type.'
status: addressed
---
