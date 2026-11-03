---
id: RR-0QCXLM
type: review-response
title: PromoteError duplicates realtime.DenialError
finding: DenialError already carries Status, Type, Title and Rule, and appbuild may import realtime, so the adapter can return *realtime.DenialError directly instead of adding a parallel PromoteError type to map.
severity: nit
resolution: The adapter returns *realtime.DenialError and PromoteError is dropped.
status: addressed
---
