---
id: RR-DKL5Y
type: review-response
title: Engine.Cache() is dead code (round 2)
finding: 'Engine.Cache() and Engine.LuaCache() return the same cache. Grepped: Engine.Cache() has zero callers. Added speculatively to preserve a name that had no existing caller.'
severity: significant
resolution: Deleted Engine.Cache(). Only Engine.LuaCache() remains, matching the ScriptExecutor interface method name.
status: addressed
---
