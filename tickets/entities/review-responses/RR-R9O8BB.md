---
id: RR-R9O8BB
type: review-response
title: hidesNavEntry re-read the schema mid-request, violating capture-state-once
finding: 'hidesNavEntry called h.schema() per navigation entry while handleV1Sidebar had already captured s := h.schema() at the top. The root CLAUDE.md rule is explicit that multiple loads against the underlying atomic.Pointer can observe different snapshots. Concretely: a config reload landing mid-loop means the navigation list comes from the old snapshot and the documents map from the new one; a renamed document then misses the lookup, hits the !ok fail-open branch, and a gated entry renders.'
severity: significant
resolution: Changed hidesNavEntry to take the caller's *Schema as a parameter (and made it a plain function rather than a method, since it no longer needs the handler). Both call sites now pass the already-captured snapshot. The godoc states why, so the parameter is not later 'simplified' back into an h.schema() call.
status: addressed
---
