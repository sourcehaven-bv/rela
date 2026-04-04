---
id: RR-LYWZ
type: review-response
title: Lua runtime created per-entity is inefficient
finding: 'At `/Users/jeroen/Work/sourcehaven/rela-3/internal/validation/lua.go:64`, a new Lua runtime is created for EVERY entity being validated. If validating 1000 entities with 5 Lua rules, that''s 5000 Lua VMs created and destroyed. The runtime initialization includes library loading and binding registration. CONSIDER: Reuse a single runtime per rule execution batch, just updating the `entity` global between validations. The runtime is already stateless (read-only workspace, discarded output). This would require careful stack management but could significantly improve performance for large repositories.'
severity: minor
reason: Creating a new Lua VM per entity ensures complete isolation and prevents state leakage between validations. The performance cost is acceptable for validation use cases (typically dozens to hundreds of entities, not thousands). If performance becomes an issue, runtime pooling can be added later as an optimization.
status: wont-fix
---
