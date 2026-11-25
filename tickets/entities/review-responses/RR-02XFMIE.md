---
id: RR-02XFMIE
type: review-response
title: Concurrent syncs could apply out of order
finding: syncQuickEntry read d.app and applied sync without one lock
severity: minor
resolution: sync takes a resolver that reads d.app under swapMu so syncs apply in order.
status: addressed
---
