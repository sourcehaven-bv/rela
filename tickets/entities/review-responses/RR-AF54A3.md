---
id: RR-AF54A3
type: review-response
title: newExportHandler panics on constructor error instead of returning it
finding: CLAUDE.md's constructors-reject-nil rule wants a returned error; the enclosing NewApp already returns (*App, error) and validates every other required field that way. The panic path is genuinely unreachable (collaborators non-nil by construction) but it's the one panic-on-construct in the package — thread the error instead.
severity: minor
resolution: newExportHandler now returns (*exportHandler, error); NewApp threads it (distinct variable to avoid govet shadow on the long-lived err); test builders fail loud. No panic-on-construct remains in the package.
status: addressed
---
