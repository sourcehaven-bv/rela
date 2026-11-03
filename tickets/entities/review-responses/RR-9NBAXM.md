---
id: RR-9NBAXM
type: review-response
title: Setting the renderer after assembly races automation workers
finding: Automation jobs can run (durable pg queue) before dataentrywire sets the renderer; data race and a missing binding.
severity: significant
resolution: 'Plan: an atomic holder created at assembly and always registered on ScheduledLuaWriteDeps; without a renderer the binding returns nil and ''attach_render is not available in this runtime''. Documents which runtimes support it.'
status: addressed
---
