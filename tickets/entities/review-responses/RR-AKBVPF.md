---
id: RR-AKBVPF
type: review-response
title: Relation gate re-read endpoint headers unparsed
finding: edgeEndpoints used key.From and key.To unparsed and read the headers a second time
severity: minor
resolution: Removed edgeEndpoints; the gate now runs on the families lookupFamily already loaded (which parses ID@face)
status: addressed
---
