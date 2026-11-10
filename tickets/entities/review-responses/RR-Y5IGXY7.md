---
id: RR-Y5IGXY7
type: review-response
title: Archplan baseline overwritten from a stale local run
finding: The committed .archplan/baseline.json replaced develop's metrics (types 4008->3955, concepts 7082->7053, propagation 8.9->9.5) and dropped two log entries; lua.Runtime gained a 25th field (types +47).
severity: significant
resolution: Restored develop's baseline, grouped lua.Runtime aiProvider/httpClient into an egress struct (types back to 4008), and recorded only the measured concepts +1 with a reason.
status: addressed
---
