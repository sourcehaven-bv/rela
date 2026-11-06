---
id: RR-GXUNEH9
type: review-response
title: Concrete Definition instead of a one-implementer interface
finding: FlowState(name) would build a []flow.Timer on every call next to State(name). A concrete flow.Definition compiled once at load is simpler and is the shape later run definitions compile to.
severity: minor
resolution: 'Addressed in plan: flow.Definition is a struct built once by the realtimecfg loader; Session.Flow returns it and Pausing copies it.'
status: addressed
---
