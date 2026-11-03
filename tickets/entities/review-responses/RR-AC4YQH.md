---
id: RR-AC4YQH
type: review-response
title: BusyReader is justified, but it is a Service method by another name
finding: A separate exported type is fine given the plimsoll cap (internal/realtime/service.go:67), and dataentry's one-method consumer interface follows the call-site rule. But NewBusyReader(*Service) reaches into begin, anchors, acc and roleAdmits, so it is coupled to the whole Service. The plan should keep the per-call state in an unexported busyRun{s, c}, mirroring inboxRun (internal/realtime/inbox.go:456), so BusyReader stays a thin entry point. It should also not export BusyAnchor if dataentry can pass entity.Ref plus type through a smaller struct.
severity: nit
resolution: Per-call state moves to an unexported busyRun{s, c}, so BusyReader is a thin entry point. BusyAnchor stays exported because dataentry must pass each row's type, but it now carries an entity.Ref and a type only.
status: addressed
---
