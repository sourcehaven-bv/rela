---
id: RR-4J6HA
type: review-response
title: CreateOptions.Prefix comment reads awkwardly across layers
finding: 'internal/entitymanager/entitymanager.go: comment says ''Ignored when ID is set or when the entity type uses manual IDs'' — the second clause is the opposite of what validateCreateIDOpts now does (it rejects, not ignores). The comment is accurate for the workspace layer it lives in but inconsistent with API-layer behavior above.'
severity: nit
reason: 'The comment describes the workspace-layer contract honestly: at that layer, prefix IS ignored for manual IDs (the workspace doesn''t run validateCreateIDOpts). Adding a cross-reference to API-layer rejection would couple the two layers in documentation. The reader expected to know about the validator is the API handler author, not consumers of CreateOptions — they get the rejection at the HTTP boundary.'
status: wont-fix
---
