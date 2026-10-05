---
id: RR-7QG8G5
type: review-response
title: Scope in shape reports a false change on upgrade
finding: Recorded projections from releases before Scope joined the shape decode content-scoped relations as identity-scoped, so the first boot after upgrade reported a false identity to content drift on every boot.
severity: significant
resolution: ShapeProjection records RelationScopes; CompareShapes and the rename/reverse Validate checks compare scopes only when both sides record them. An old record adopts silently. Pinned by TestCompareShapes_OldRecordReportsNoScopeChange; the guide documents it.
status: addressed
---
