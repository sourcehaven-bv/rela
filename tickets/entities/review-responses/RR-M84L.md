---
id: RR-M84L
type: review-response
title: ?include=relations bypasses the entire list filter
finding: 'resolveV1Includes (api_v1.go:1677) walks outgoing/incoming relations and calls getEntity on every neighbor with NO ACL check. After this PR, GET /api/v1/projects/PRJ-1?include=tickets returns every ticket of PRJ-1 in `included` even when the principal has zero `read: [ticket]` grants. The list endpoint is gated, the per-entity GET is gated, but ?include= is a third read path the plan defers to a ''future ticket'' while documenting it in the guide. Documentation is not a control. AC1 (`sees nothing of other types`) passes while the system still leaks one query parameter away. Fix in this PR: filter resolveV1Includes via req.Visible(ctx, target.Type, target.ID) before including. Add AC that pins ?include=* returns only entities visible to the principal.'
severity: critical
resolution: 'Incorporated into rescoped TKT-VQGN scope: resolveV1Includes filters each neighbour via req.Visible before including. Pinned by AC4 (include=* returns only visible entities).'
status: addressed
---
