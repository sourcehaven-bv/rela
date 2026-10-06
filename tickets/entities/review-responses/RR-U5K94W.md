---
id: RR-U5K94W
type: review-response
title: The content-scope rule had two copies that disagreed on nil
finding: dataentry/edgeowner.go contentScoped reimplemented appbuild.metamodelScopes.IsContentScoped, against the RelationScopes godoc, and the two gave opposite verdicts for a nil metamodel.
severity: significant
resolution: Added metamodel.IsContentScoped as the single owner (package-level because Metamodel's API is capped; nil answers content-scoped, fail closed). appbuild.metamodelScopes and dataentry both delegate to it. The wiring-time vs live metamodel snapshot difference predates this change (worldreader is wired once) and is left to the Stage 1 resolver TKT-2528AB.
status: addressed
---
