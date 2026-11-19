---
id: RR-4AVJG15
type: review-response
title: Atomic box for two-phase wiring
finding: createStartEngineBox plus atomic.Pointer exists only for the late engine.
severity: nit
reason: The hook is called concurrently once set, and the engine is built after the manager that needs the hook; the atomic box is the simplest race-free form.
status: wont-fix
---
