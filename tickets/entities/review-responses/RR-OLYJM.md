---
id: RR-OLYJM
type: review-response
title: WithRouteCatalog wiring duplicated across cmd/*
finding: 'cmd/rela-server/main.go and cmd/rela-desktop/main.go both hardcode dataentry.WithRouteCatalog(lua.RouteCatalogFunc(frontendroutes.Has)). Two call sites to keep in sync. Fix: expose dataentry.DefaultFrontendRouteCatalog() helper that returns the wired option (or bakes the wiring in so NewApp defaults to it when no option given). Cleaner callers.'
severity: nit
reason: DefaultFrontendRouteCatalog helper is a small ergonomic win but two call sites for the same wiring is tolerable. Revisit if a third binary shows up (e.g. a CLI mode that renders documents).
status: deferred
---
