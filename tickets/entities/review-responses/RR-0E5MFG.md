---
id: RR-0E5MFG
type: review-response
title: 'Put the due wake in a small type of its own, not in Local, and avoid reusing the key name from:'
finding: 'Local already holds the ephemeral expiry timer and 16 methods. The due wake has a single writer side (Service, systemWriter) and a single reader (the worker), which is the tombstoneConfirmer.wake shape (realtime_worker.go:74-146). Plan change: add a realtime.DueWake with Note, C and Take, injected into the service and the worker, and leave Local unchanged. Separately, from: on a timer means ''anchor'', while from: on a lifecycle op means ''source states''. Consider since: state|session to avoid two meanings for one key in one file.'
severity: nit
resolution: 'Plan updated: wake moved to its own realtime.DueWake (Note, C, Take) injected into service, system writer and worker; Local unchanged; anchor key renamed since: state|session and from refused.'
status: addressed
---
