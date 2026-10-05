---
id: RR-L21II4
type: review-response
title: Converted empty face literals are missed
finding: GetEntityState(ctx, id, entity.Face("")) is a zero-face read the literal check did not match.
severity: minor
resolution: isEmptyFace unwraps a one-argument conversion. It found internal/docs/resolvers_graph.go, now on the allowlist. Variables and named constants are documented as not caught.
status: addressed
---
