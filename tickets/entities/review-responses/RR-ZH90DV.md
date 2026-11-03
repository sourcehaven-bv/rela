---
id: RR-ZH90DV
type: review-response
title: Unwrapped comment line in liveHub.run godoc
finding: 'live.go:636-637: the added sentence was appended to an existing line and not rewrapped (''...(see live_busy.go). A reload of realtime.yaml (reconfigured) dirties'' is near the 120 limit while its neighbours wrap at about 76). Fix: rewrap the paragraph.'
severity: nit
resolution: 'Fixed: rewrapped the liveHub.run godoc paragraph.'
status: addressed
---
