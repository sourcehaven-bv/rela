---
id: RR-ETY1PZZ
type: review-response
title: Lookup scans every stream under h.mu on each GET
finding: The plan iterated all conns under the hub lock for every live-path GET.
severity: significant
resolution: liveHub.byFP indexes conns by share key. register and unregister maintain it. A GET scans only its own principal's streams.
status: addressed
---
