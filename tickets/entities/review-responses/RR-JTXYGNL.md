---
id: RR-JTXYGNL
type: review-response
title: New tab does not know peers' focus for up to 15 s
finding: Focus was learnt only from events and the 15 s heartbeat, so a new holder could notify while another tab had focus.
severity: nit
resolution: 'A starting tab posts with ask: true and every peer answers at once with its state. Test added.'
status: addressed
---
