---
id: RR-5TYGFO
type: review-response
title: Free-edge chain resolution needs projections migration files don't carry
finding: 'Chain resolution treats compatibility as a free edge: to bridge from the store''s current hash to a migration''s `from` hash, the resolver must run CompareShapes(current.projection, migration.from.projection). But migration files as specified carry only the from/to HASHES, not projections — and there is no reliable source for an arbitrary historical ShapeProjection (schema_versions is pg-only and holds RenderProjection, a different type; git archaeology is out). The same gap breaks the plan''s own ''step targets validated against the FROM projection at plan time'' requirement. The projection must travel with the migration.'
severity: significant
resolution: 'Amendment A2: migration files embed the FROM ShapeProjection JSON (from_projection field); the from hash is integrity-checked against the embedded projection at parse time. This supplies both free-edge CompareShapes input and plan-time step-target validation. Sidecar alternative rejected for self-containedness. AC5 updated.'
status: addressed
---
