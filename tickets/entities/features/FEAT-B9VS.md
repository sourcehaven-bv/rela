---
id: FEAT-B9VS
type: feature
title: Consumer-side service narrowing at subsystem boundaries
summary: 'Subsystems (mcp, cli, dataentry, scheduler) take focused per-subsystem Deps structs of domain types instead of depending on the appbuild.Services aggregate. Enforced by arch-lint: Layer-3 subsystems may not import internal/appbuild.'
description: Each subsystem declares its own consumer-side Deps (named struct of domain types) in its own package; the entry-point wiring constructs it from the per-project services. appbuild.New stays as a construction helper because the construction order is load-bearing (search observer before store open; lua read deps before entity manager; validator and manager must share the same lua surface), but *appbuild.Services only escapes to Layer-4 wiring. arch-lint forbids internal/{mcp,cli,dataentry,scheduler} from importing internal/appbuild. Plan reviewed and approved via crit (.ignored/service-layering-plan.md).
status: in-progress
---
