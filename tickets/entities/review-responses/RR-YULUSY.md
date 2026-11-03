---
id: RR-YULUSY
type: review-response
title: begin() denial for a caller without identity is logged as a store error on every list
finding: Service.begin returns ErrRealtimeOff, or a noPrincipal DenialError when the access layer finds no principal (internal/realtime/authz.go:70-85). Plan step 4 treats every error as a store error and logs a warning, so an anonymous or unidentified list request would log on every call. AC 3 should list "no principal" as ok=false, and TestBusy_NotServed should cover it.
severity: minor
resolution: AC 3 and step 2 treat ErrRealtimeOff and the no-principal denial from begin as ok=false with no log; TestBusy_NotServed gets a no-principal case.
status: addressed
---
